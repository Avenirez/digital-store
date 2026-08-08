package handler

import (
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

// WebhookHandler handles payment gateway webhook callbacks and app notification listeners.
type WebhookHandler struct {
	orderRepo   *repository.OrderRepo
	stockRepo   *repository.StockRepo
	productRepo *repository.ProductRepo
	crypto      *service.CryptoService
	telegram    *service.TelegramService
}

// NewWebhookHandler creates a new WebhookHandler.
func NewWebhookHandler(
	orderRepo *repository.OrderRepo,
	stockRepo *repository.StockRepo,
	productRepo *repository.ProductRepo,
	crypto *service.CryptoService,
	telegram *service.TelegramService,
) *WebhookHandler {
	return &WebhookHandler{
		orderRepo:   orderRepo,
		stockRepo:   stockRepo,
		productRepo: productRepo,
		crypto:      crypto,
		telegram:    telegram,
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
	// Secure endpoint: verify notification secret strictly
	expectedSecret := os.Getenv("NOTIFICATION_SECRET")
	providedSecret := r.Header.Get("X-Notification-Secret")
	if providedSecret == "" {
		providedSecret = r.Header.Get("X-MacroDroid-Secret")
	}

	if expectedSecret == "" || subtle.ConstantTimeCompare([]byte(providedSecret), []byte(expectedSecret)) != 1 {
		log.Printf("[NOTIFICATION-LISTENER] Unauthorized request attempt from IP: %s", r.RemoteAddr)
		writeError(w, http.StatusUnauthorized, "Unauthorized notification request")
		return
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
