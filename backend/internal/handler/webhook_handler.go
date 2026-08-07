package handler

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"

	"my-digital-store/backend/internal/repository"
	"my-digital-store/backend/internal/service"
)

// WebhookHandler handles payment gateway webhook callbacks.
type WebhookHandler struct {
	orderRepo   *repository.OrderRepo
	stockRepo   *repository.StockRepo
	productRepo *repository.ProductRepo
	duitku      *service.DuitkuService
	crypto      *service.CryptoService
	telegram    *service.TelegramService
	resend      *service.ResendService
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(
	orderRepo *repository.OrderRepo,
	stockRepo *repository.StockRepo,
	productRepo *repository.ProductRepo,
	duitku *service.DuitkuService,
	crypto *service.CryptoService,
	telegram *service.TelegramService,
	resend *service.ResendService,
) *WebhookHandler {
	return &WebhookHandler{
		orderRepo:   orderRepo,
		stockRepo:   stockRepo,
		productRepo: productRepo,
		duitku:      duitku,
		crypto:      crypto,
		telegram:    telegram,
		resend:      resend,
	}
}

// DuitkuCallback handles POST /api/v1/webhooks/duitku
// This endpoint receives payment notifications from Duitku.
// Content-Type: application/x-www-form-urlencoded
func (h *WebhookHandler) DuitkuCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Parse and verify the callback signature
	payload, err := h.duitku.ParseCallback(r)
	if err != nil {
		log.Printf("[WEBHOOK] ParseCallback error: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// 2. Find the order by merchantOrderId (our order_number)
	order, err := h.orderRepo.GetByOrderNumber(ctx, payload.MerchantOrderID)
	if err != nil {
		log.Printf("[WEBHOOK] Order not found: %s, error: %v", payload.MerchantOrderID, err)
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	// 3. Idempotency check — skip if already processed
	if order.Status == repository.OrderStatusPaid {
		log.Printf("[WEBHOOK] Order %s already PAID, skipping duplicate callback", order.OrderNumber)
		w.WriteHeader(http.StatusOK)
		return
	}

	// 4. Handle payment result
	if payload.IsSuccess() {
		// ─── Payment Successful ──────────────────────────────

		// Update order status to PAID
		if err := h.orderRepo.UpdateStatus(ctx, order.ID, repository.OrderStatusPaid); err != nil {
			log.Printf("[WEBHOOK] UpdateStatus error: %v", err)
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		// Mark reserved stocks as SOLD
		soldCount, err := h.stockRepo.MarkSoldByOrderID(ctx, order.ID)
		if err != nil {
			log.Printf("[WEBHOOK] MarkSoldByOrderID error: %v", err)
		}
		log.Printf("[WEBHOOK] Order %s PAID: %d stocks marked as SOLD", order.OrderNumber, soldCount)

		// ─── Async: Send credential email ────────────────────
		go h.sendCredentialEmail(order)

		// ─── Async: Telegram alert ───────────────────────────
		product, _ := h.productRepo.GetByID(ctx, order.ProductID)
		productTitle := "Unknown Product"
		if product != nil {
			productTitle = product.Title

			// Check low stock alert (< 5 remaining)
			remaining, _ := h.stockRepo.CountAvailable(ctx, product.ID)
			if remaining < 5 {
				h.telegram.AlertLowStock(product.Title, remaining)
			}
		}
		h.telegram.AlertNewOrder(order.OrderNumber, order.CustomerEmail, productTitle, order.Quantity, order.TotalAmount, order.Pin)

	} else {
		// ─── Payment Failed ──────────────────────────────────
		log.Printf("[WEBHOOK] Order %s payment FAILED (resultCode=%s)", order.OrderNumber, payload.ResultCode)

		if err := h.orderRepo.UpdateStatus(ctx, order.ID, repository.OrderStatusFailed); err != nil {
			log.Printf("[WEBHOOK] UpdateStatus (FAILED) error: %v", err)
		}

		// Release reserved stocks
		released, err := h.stockRepo.ReleaseByOrderID(ctx, order.ID)
		if err != nil {
			log.Printf("[WEBHOOK] ReleaseByOrderID error: %v", err)
		}
		log.Printf("[WEBHOOK] Order %s FAILED: %d stocks released", order.OrderNumber, released)
	}

	// 5. Return HTTP 200 OK (required by Duitku)
	w.WriteHeader(http.StatusOK)
}

// sendCredentialEmail decrypts credentials and sends them via Resend.
func (h *WebhookHandler) sendCredentialEmail(order *repository.Order) {
	if !h.resend.IsConfigured() {
		return
	}

	ctx := context.Background()

	stocks, err := h.stockRepo.GetByOrderID(ctx, order.ID)
	if err != nil {
		log.Printf("[WEBHOOK] Failed to fetch stocks for email: %v", err)
		return
	}

	product, _ := h.productRepo.GetByID(ctx, order.ProductID)
	productTitle := "Digital Account"
	if product != nil {
		productTitle = product.Title
	}

	var credentials []service.CredentialItem
	for _, s := range stocks {
		password, err := h.crypto.Decrypt(s.PasswordEncrypted)
		if err != nil {
			log.Printf("[WEBHOOK] Decrypt error for stock %s: %v", s.ID, err)
			password = "[error]"
		}
		credentials = append(credentials, service.CredentialItem{
			Email:    s.Email,
			Password: password,
		})
	}

	if err := h.resend.SendCredentials(order.CustomerEmail, order.OrderNumber, productTitle, credentials); err != nil {
		log.Printf("[WEBHOOK] Failed to send credential email: %v", err)
	}
}

// NotificationPayload struct for MacroDroid / App Listener HTTP POST
type NotificationPayload struct {
	Title string `json:"title"`
	Text  string `json:"text"`
	App   string `json:"app"`
}

// NotificationListener handles POST /api/v1/webhooks/notification
// It receives push notifications from MacroDroid on Android and matches payment amounts.
func (h *WebhookHandler) NotificationListener(w http.ResponseWriter, r *http.Request) {
	// Secure endpoint: verify notification secret if NOTIFICATION_SECRET is set
	expectedSecret := os.Getenv("NOTIFICATION_SECRET")
	if expectedSecret != "" {
		providedSecret := r.Header.Get("X-Notification-Secret")
		if providedSecret == "" {
			providedSecret = r.Header.Get("X-MacroDroid-Secret")
		}
		if subtle.ConstantTimeCompare([]byte(providedSecret), []byte(expectedSecret)) != 1 {
			log.Printf("[NOTIFICATION-LISTENER] Unauthorized request attempt from IP: %s", r.RemoteAddr)
			writeError(w, http.StatusUnauthorized, "Unauthorized notification request")
			return
		}
	}

	ctx := r.Context()

	var payload NotificationPayload
	// Support JSON decode
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		// Fallback parse form if sent as form-data
		r.ParseForm()
		payload.Title = r.FormValue("title")
		payload.Text = r.FormValue("text")
		payload.App = r.FormValue("app")
	}

	log.Printf("[NOTIFICATION-LISTENER] Received notification from %s | Title: %s | Text: %s", payload.App, payload.Title, payload.Text)

	fullContent := payload.Title + " " + payload.Text
	if fullContent == " " {
		http.Error(w, "Empty payload", http.StatusBadRequest)
		return
	}

	// Extract amount from text (e.g. "Rp 50.187", "Rp.50.187", "IDR 50.187", "sebesar 50187", "Rp50.187,00")
	re := regexp.MustCompile(`(?:Rp\.?|IDR|sebesar)\s?([0-9\.\,]+)`)
	matches := re.FindStringSubmatch(fullContent)
	if len(matches) < 2 {
		// Fallback regex to capture any formatted number with dots like 50.187
		reFallback := regexp.MustCompile(`\b([0-9]{1,3}(?:\.[0-9]{3})+)\b`)
		matches = reFallback.FindStringSubmatch(fullContent)
	}

	if len(matches) < 2 {
		log.Printf("[NOTIFICATION-LISTENER] No Rp or amount pattern found in notification text")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ignored","reason":"No valid amount found"}`))
		return
	}

	rawAmountStr := matches[1]
	// Handle case where string ends with trailing decimal cents like ,00
	if strings.HasSuffix(rawAmountStr, ",00") {
		rawAmountStr = strings.TrimSuffix(rawAmountStr, ",00")
	}
	// Clean formatting (dots to empty, remaining comma to dot)
	cleanedAmount := strings.ReplaceAll(rawAmountStr, ".", "")
	cleanedAmount = strings.ReplaceAll(cleanedAmount, ",", ".")
	paidAmount, err := strconv.ParseFloat(cleanedAmount, 64)
	if err != nil {
		log.Printf("[NOTIFICATION-LISTENER] Failed to parse float amount %s: %v", cleanedAmount, err)
		http.Error(w, "Invalid amount format", http.StatusBadRequest)
		return
	}

	log.Printf("[NOTIFICATION-LISTENER] Parsed incoming payment amount: Rp %.2f", paidAmount)

	// Search for PENDING order matching this total_amount (within exact match or 0.01 tolerance)
	var orderID, orderNumber, customerEmail, pin, productID string
	var quantity int
	var totalAmount float64
	query := `
		SELECT id, order_number, COALESCE(customer_email, 'Pembeli'), COALESCE(pin, '123456'), product_id::text, quantity, total_amount
		FROM orders
		WHERE status = 'PENDING' AND ABS(total_amount - $1) < 0.01
		ORDER BY created_at DESC
		LIMIT 1
	`
	err = h.orderRepo.GetDB().QueryRow(ctx, query, paidAmount).Scan(&orderID, &orderNumber, &customerEmail, &pin, &productID, &quantity, &totalAmount)
	if err != nil {
		log.Printf("[NOTIFICATION-LISTENER] No pending order found for amount Rp %.2f (or query err: %v)", paidAmount, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"not_found","message":"No matching pending order for this amount"}`))
		return
	}

	// Update order status to PAID
	if err := h.orderRepo.UpdateStatus(ctx, orderID, repository.OrderStatusPaid); err != nil {
		log.Printf("[NOTIFICATION-LISTENER] UpdateStatus error: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Mark reserved stocks as SOLD
	soldCount, err := h.stockRepo.MarkSoldByOrderID(ctx, orderID)
	if err != nil {
		log.Printf("[NOTIFICATION-LISTENER] MarkSoldByOrderID error: %v", err)
	}
	log.Printf("[NOTIFICATION-LISTENER] Order %s (Rp %.2f) SUCCESS via Notification Listener! %d stocks marked SOLD", orderNumber, totalAmount, soldCount)

	// Send notifications asynchronously
	orderObj := &repository.Order{
		ID:            orderID,
		OrderNumber:   orderNumber,
		CustomerEmail: customerEmail,
		ProductID:     productID,
		TotalAmount:   totalAmount,
	}
	go h.sendCredentialEmail(orderObj)

	product, _ := h.productRepo.GetByID(ctx, productID)
	productTitle := "Digital Account"
	if product != nil {
		productTitle = product.Title
	}
	h.telegram.AlertNewOrder(orderNumber, customerEmail, productTitle, quantity, totalAmount, pin)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success","order_number":"` + orderNumber + `"}`))
}
