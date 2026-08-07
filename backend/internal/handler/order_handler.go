package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"my-digital-store/backend/internal/repository"
	"my-digital-store/backend/internal/service"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// OrderHandler handles order-related HTTP endpoints.
type OrderHandler struct {
	db          *pgxpool.Pool
	orderRepo   *repository.OrderRepo
	stockRepo   *repository.StockRepo
	productRepo *repository.ProductRepo
	duitku      *service.DuitkuService
	crypto      *service.CryptoService
	telegram    *service.TelegramService
	resend      *service.ResendService
	redisClient *redis.Client
}

// NewOrderHandler creates a new OrderHandler.
func NewOrderHandler(
	db *pgxpool.Pool,
	orderRepo *repository.OrderRepo,
	stockRepo *repository.StockRepo,
	productRepo *repository.ProductRepo,
	duitku *service.DuitkuService,
	crypto *service.CryptoService,
	telegram *service.TelegramService,
	resend *service.ResendService,
	redisClient *redis.Client,
) *OrderHandler {
	return &OrderHandler{
		db:          db,
		orderRepo:   orderRepo,
		stockRepo:   stockRepo,
		productRepo: productRepo,
		duitku:      duitku,
		crypto:      crypto,
		telegram:    telegram,
		resend:      resend,
		redisClient: redisClient,
	}
}

// ─── Checkout ────────────────────────────────────────────────────

// CheckoutRequest is the JSON body for POST /api/v1/checkout.
type CheckoutRequest struct {
	ProductSlug  string `json:"product_slug"`
	Pin          string `json:"pin"`
	CustomerName string `json:"customer_name"`
	Quantity     int    `json:"quantity"`
}

// Checkout handles POST /api/v1/checkout
// Creates an order, reserves stock, and initiates a Duitku payment.
func (h *OrderHandler) Checkout(w http.ResponseWriter, r *http.Request) {
	var req CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate input
	if req.ProductSlug == "" || req.Pin == "" {
		writeError(w, http.StatusBadRequest, "product_slug and pin are required")
		return
	}
	if strings.TrimSpace(req.CustomerName) == "" {
		writeError(w, http.StatusBadRequest, "Nama pembeli wajib diisi")
		return
	}
	req.CustomerName = strings.TrimSpace(req.CustomerName)
	if len(req.Pin) < 4 || len(req.Pin) > 10 {
		writeError(w, http.StatusBadRequest, "PIN must be between 4 and 10 digits")
		return
	}
	if req.Quantity <= 0 {
		req.Quantity = 1
	}

	ctx := r.Context()

	// 1. Fetch product
	product, err := h.productRepo.GetBySlug(ctx, req.ProductSlug)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "Product not found")
			return
		}
		log.Printf("[CHECKOUT] GetBySlug error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to fetch product")
		return
	}

	// 2. Check available stock
	availableCount, err := h.stockRepo.CountAvailable(ctx, product.ID)
	if err != nil {
		log.Printf("[CHECKOUT] CountAvailable error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to check stock")
		return
	}
	if availableCount < req.Quantity {
		writeError(w, http.StatusConflict, fmt.Sprintf("Insufficient stock: %d available, %d requested", availableCount, req.Quantity))
		return
	}

	// 3. Time zone setup (WIB UTC+7) for daily reset logic
	loc := time.FixedZone("WIB", 7*3600)
	nowWIB := time.Now().In(loc)
	dateStr := nowWIB.Format("20060102")

	// Calculate total with sequential unique code (starts at 100, resets at 200 & resets daily at 00:00 WIB)
	baseAmount := product.PriceIDR * float64(req.Quantity)
	var startCode int64 = 100

	if h.redisClient != nil {
		redisKey := fmt.Sprintf("order:unique_code_seq:%s", dateStr)
		val, err := h.redisClient.Incr(ctx, redisKey).Result()
		if err == nil {
			if val == 1 {
				_ = h.redisClient.Expire(ctx, redisKey, 48*time.Hour).Err()
			}
			startCode = 100 + ((val - 1) % 101)
		}
	} else {
		startCode = 100
	}

	// Prevent collision with any existing PENDING order with the exact same total amount
	var uniqueCode int64 = startCode
	var totalAmount float64
	for attempt := 0; attempt < 101; attempt++ {
		candidateTotal := baseAmount + float64(uniqueCode)
		var pendingCount int
		err := h.db.QueryRow(ctx, `SELECT COUNT(*) FROM orders WHERE total_amount = $1 AND status = 'PENDING'`, candidateTotal).Scan(&pendingCount)
		if err == nil && pendingCount == 0 {
			totalAmount = candidateTotal
			break
		}
		uniqueCode++
		if uniqueCode > 200 {
			uniqueCode = 100
		}
	}
	if totalAmount == 0 {
		totalAmount = baseAmount + float64(uniqueCode)
	}

	// 4. Generate unique order number with ID- prefix and 001-100 daily reset logic (WIB UTC+7)
	datePrefix := fmt.Sprintf("ID-%s-", dateStr)

	var seqNum int
	if h.redisClient != nil {
		redisKey := fmt.Sprintf("order:daily_seq:%s", dateStr)
		val, err := h.redisClient.Incr(ctx, redisKey).Result()
		if err == nil {
			if val == 1 {
				_ = h.redisClient.Expire(ctx, redisKey, 48*time.Hour).Err()
			}
			if val > 100 {
				seqNum = int(((val - 1) % 100) + 1)
			} else {
				seqNum = int(val)
			}
		}
	}

	if seqNum <= 0 {
		var err error
		seqNum, err = h.orderRepo.GetNextDailySeq(ctx, datePrefix)
		if err != nil {
			log.Printf("[CHECKOUT] GetNextDailySeq error: %v", err)
			seqNum = 1
		}
	}

	orderNumber := fmt.Sprintf("%s%03d", datePrefix, seqNum)

	// 5. Create order in DB
	order := &repository.Order{
		OrderNumber:   orderNumber,
		CustomerEmail: req.CustomerName,
		Pin:           req.Pin,
		ProductID:     product.ID,
		Quantity:      req.Quantity,
		TotalAmount:   totalAmount,
		Status:        repository.OrderStatusPending,
	}

	if err := h.orderRepo.Create(ctx, order); err != nil {
		log.Printf("[CHECKOUT] Create order error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create order")
		return
	}

	// 6. Reserve stock within a transaction
	tx, err := h.db.Begin(ctx)
	if err != nil {
		log.Printf("[CHECKOUT] Begin tx error: %v", err)
		writeError(w, http.StatusInternalServerError, "Transaction error")
		return
	}
	defer tx.Rollback(ctx)

	for i := 0; i < req.Quantity; i++ {
		_, err := h.stockRepo.ReserveOne(ctx, tx, product.ID, order.ID)
		if err != nil {
			if err == pgx.ErrNoRows {
				writeError(w, http.StatusConflict, "Stock sold out during reservation")
				return
			}
			log.Printf("[CHECKOUT] ReserveOne error: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to reserve stock")
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Printf("[CHECKOUT] Commit error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to commit reservation")
		return
	}

	// 7. Direct Payment URL to frontend checkout page
	paymentURL := fmt.Sprintf("/checkout?order_id=%s&pin=%s", orderNumber, req.Pin)
	reference := fmt.Sprintf("NOTIF-%s", orderNumber)

	_ = h.orderRepo.SetPaymentInfo(ctx, order.ID, paymentURL, reference)

	// 8. Return response with unique code details
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"order_id":     order.ID,
		"order_number": orderNumber,
		"payment_url":  paymentURL,
		"reference":    reference,
		"base_amount":  baseAmount,
		"unique_code":  uniqueCode,
		"amount":       totalAmount,
		"status":       repository.OrderStatusPending,
	})
}

// SimulatePay handles POST /api/v1/orders/simulate-pay
// Mengubah status pesanan PENDING menjadi PAID dan mengubah stok RESERVED menjadi SOLD untuk keperluan testing.
func (h *OrderHandler) SimulatePay(w http.ResponseWriter, r *http.Request) {
	var req struct {
		OrderNumber string `json:"order_number"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.OrderNumber == "" {
		writeError(w, http.StatusBadRequest, "order_number is required")
		return
	}

	ctx := r.Context()
	order, err := h.orderRepo.GetByOrderNumber(ctx, req.OrderNumber)
	if err != nil {
		writeError(w, http.StatusNotFound, "Order not found")
		return
	}

	if order.Status == repository.OrderStatusPaid {
		writeJSON(w, http.StatusOK, map[string]string{"message": "Order is already paid", "status": "PAID"})
		return
	}

	// Update status order ke PAID
	if err := h.orderRepo.UpdateStatus(ctx, order.ID, repository.OrderStatusPaid); err != nil {
		log.Printf("[SIMULATE-PAY] UpdateStatus error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to update order status")
		return
	}

	// Mark stok RESERVED -> SOLD
	soldCount, err := h.stockRepo.MarkSoldByOrderID(ctx, order.ID)
	if err != nil {
		log.Printf("[SIMULATE-PAY] MarkSoldByOrderID error: %v", err)
	}
	log.Printf("[SIMULATE-PAY] Order %s marked as PAID, %d stocks sold", order.OrderNumber, soldCount)

	// Send Telegram alert
	product, _ := h.productRepo.GetByID(ctx, order.ProductID)
	productTitle := "Digital Account"
	if product != nil {
		productTitle = product.Title
	}
	h.telegram.AlertNewOrder(order.OrderNumber, order.CustomerEmail, productTitle, order.Quantity, order.TotalAmount, order.Pin)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "Payment simulated successfully!",
		"order_number": order.OrderNumber,
		"status":       "PAID",
	})
}

// ─── Guest Order Lookup ──────────────────────────────────────────

// OrderLookup handles GET /api/v1/orders/lookup?order_id=...&pin=...
func (h *OrderHandler) OrderLookup(w http.ResponseWriter, r *http.Request) {
	orderID := r.URL.Query().Get("order_id")
	pin := r.URL.Query().Get("pin")

	if orderID == "" {
		writeError(w, http.StatusBadRequest, "order_id is required")
		return
	}

	ctx := r.Context()

	// Try lookup by order_number
	order, err := h.orderRepo.GetByOrderNumber(ctx, orderID)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Try by UUID
			order, err = h.orderRepo.GetByID(ctx, orderID)
			if err != nil {
				writeError(w, http.StatusNotFound, "Pesanan tidak ditemukan")
				return
			}
		} else {
			log.Printf("[LOOKUP] Error: %v", err)
			writeError(w, http.StatusInternalServerError, "Failed to look up order")
			return
		}
	}

	// If PIN is provided, verify PIN if wrong
	if pin != "" && order.Pin != pin {
		writeError(w, http.StatusUnauthorized, "PIN yang kamu masukkan salah")
		return
	}

	response := map[string]interface{}{
		"order": order,
	}

	// If order is PAID and PIN matches, include decrypted credentials
	if order.Status == repository.OrderStatusPaid && (pin == "" || order.Pin == pin) {
		stocks, err := h.stockRepo.GetByOrderID(ctx, order.ID)
		if err != nil {
			log.Printf("[LOOKUP] GetByOrderID error: %v", err)
		} else {
			var credentials []map[string]string
			for _, s := range stocks {
				password, err := h.crypto.Decrypt(s.PasswordEncrypted)
				if err != nil {
					log.Printf("[LOOKUP] Decrypt error for stock %s: %v", s.ID, err)
					password = "[decryption error]"
				}
				credentials = append(credentials, map[string]string{
					"email":           s.Email,
					"password":        password,
					"additional_info": s.AdditionalInfo,
				})
			}
			response["credentials"] = credentials
		}
	}

	writeJSON(w, http.StatusOK, response)
}

// ─── Credentials Download (.txt) ─────────────────────────────────

// DownloadCredentials handles GET /api/v1/orders/download?order_id=...&pin=...
// Generates a .txt file in-memory and streams it directly to the response.
func (h *OrderHandler) DownloadCredentials(w http.ResponseWriter, r *http.Request) {
	orderID := r.URL.Query().Get("order_id")
	pin := r.URL.Query().Get("pin")

	if orderID == "" || pin == "" {
		writeError(w, http.StatusBadRequest, "order_id and pin are required")
		return
	}

	ctx := r.Context()

	// Verify order ownership via PIN
	order, err := h.orderRepo.GetByOrderNumberAndPin(ctx, orderID, pin)
	if err != nil {
		if err == pgx.ErrNoRows {
			order, err = h.orderRepo.GetByID(ctx, orderID)
			if err != nil || order.Pin != pin {
				writeError(w, http.StatusNotFound, "Order not found")
				return
			}
		} else {
			writeError(w, http.StatusInternalServerError, "Failed to look up order")
			return
		}
	}

	if order.Status != repository.OrderStatusPaid {
		writeError(w, http.StatusForbidden, "Credentials available only for paid orders")
		return
	}

	// Fetch stocks
	stocks, err := h.stockRepo.GetByOrderID(ctx, order.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to fetch credentials")
		return
	}

	// Format timestamp PostgreSQL ke WIB rapi (04 Aug 2026 17:38 WIB)
	formattedDate := order.CreatedAt
	if t, err := time.Parse(time.RFC3339, order.CreatedAt); err == nil {
		formattedDate = t.Format("02 Jan 2006 15:04 WIB")
	} else if len(order.CreatedAt) >= 16 {
		formattedDate = order.CreatedAt[:16] + " WIB"
	}

	// Build .txt content in memory
	var content strings.Builder
	content.WriteString("═══════════════════════════════════════\n")
	content.WriteString("  LEXAA STORE - INFORMASI AKUN\n")
	content.WriteString(fmt.Sprintf("  Order:   %s\n", order.OrderNumber))
	content.WriteString(fmt.Sprintf("  Tanggal: %s\n", formattedDate))
	content.WriteString("═══════════════════════════════════════\n\n")

	for i, s := range stocks {
		password, err := h.crypto.Decrypt(s.PasswordEncrypted)
		if err != nil {
			password = "[decryption error]"
		}

		content.WriteString(fmt.Sprintf("Account #%d\n", i+1))
		content.WriteString(fmt.Sprintf("  Email:    %s\n", s.Email))
		content.WriteString(fmt.Sprintf("  Password: %s\n", password))
		if s.AdditionalInfo != "" {
			content.WriteString(fmt.Sprintf("  Info:     %s\n", s.AdditionalInfo))
		}
		content.WriteString("\n")
	}

	content.WriteString("═══════════════════════════════════════\n")
	content.WriteString("    Simpan file ini dengan baik.\n")

	// Stream as downloadable .txt file
	filename := fmt.Sprintf("credentials_%s.txt", order.OrderNumber)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(content.String()))
}
