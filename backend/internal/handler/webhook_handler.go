package handler

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
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
	Title  string `json:"title"`
	Text   string `json:"text"`
	App    string `json:"app"`
	Secret string `json:"secret"`
}

// parsePaymentAmount extracts paid IDR amount from notification title/text.
func parsePaymentAmount(fullContent string) (float64, bool) {
	// Primary regex matching monetary keywords and formats
	rePrimary := regexp.MustCompile(`(?i)(?:Rp\.?|IDR|sebesar|nominal|jumlah|total|kredit|masuk|berhasil|dana|gopay|ovo|qris|seabank|bca|mandiri|bri|bni)\s?:?\s?([0-9\.\,]+)`)
	matches := rePrimary.FindStringSubmatch(fullContent)

	if len(matches) < 2 {
		// Fallback regex matching formatted numbers like 50.187
		reFallback := regexp.MustCompile(`\b([0-9]{1,3}(?:\.[0-9]{3})+)\b`)
		matches = reFallback.FindStringSubmatch(fullContent)
	}

	if len(matches) < 2 {
		// Fallback matching raw numeric strings (e.g. 50187)
		reRaw := regexp.MustCompile(`\b([1-9][0-9]{3,6})\b`)
		matches = reRaw.FindStringSubmatch(fullContent)
	}

	if len(matches) < 2 {
		return 0, false
	}

	raw := strings.TrimSpace(matches[1])
	// Trim trailing punctuation like dots, commas, hyphens
	raw = strings.TrimRight(raw, ".,- ")

	// Trim common cent suffixes
	for _, suffix := range []string{",00", ".00", ",0", ".0"} {
		if strings.HasSuffix(raw, suffix) {
			raw = strings.TrimSuffix(raw, suffix)
			break
		}
	}

	var cleaned string
	if strings.Contains(raw, ".") && strings.Contains(raw, ",") {
		// Format e.g. 50.187,00 -> remove dot, replace comma
		cleaned = strings.ReplaceAll(raw, ".", "")
		cleaned = strings.ReplaceAll(cleaned, ",", ".")
	} else if strings.Contains(raw, ".") {
		// Format e.g. 50.187 -> remove dot
		cleaned = strings.ReplaceAll(raw, ".", "")
	} else if strings.Contains(raw, ",") {
		// Format e.g. 50187,00 -> replace comma with dot
		cleaned = strings.ReplaceAll(raw, ",", ".")
	} else {
		cleaned = raw
	}

	amount, err := strconv.ParseFloat(cleaned, 64)
	if err != nil || amount <= 0 {
		return 0, false
	}

	return amount, true
}

// NotificationListener handles POST /api/v1/webhooks/notification
// It receives push notifications from MacroDroid on Android and matches payment amounts.
func (h *WebhookHandler) NotificationListener(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// 1. Read body safely into byte buffer to avoid double-read issues
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[NOTIFICATION-LISTENER] Error reading body: %v", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var payload NotificationPayload
	if len(bodyBytes) > 0 {
		_ = json.Unmarshal(bodyBytes, &payload)
	}

	// Fallback form parsing if JSON decode didn't set fields
	if payload.Title == "" && payload.Text == "" {
		if formVals, parseErr := url.ParseQuery(string(bodyBytes)); parseErr == nil {
			payload.Title = formVals.Get("title")
			payload.Text = formVals.Get("text")
			payload.App = formVals.Get("app")
			if payload.Secret == "" {
				payload.Secret = formVals.Get("secret")
			}
		}
	}

	// 2. Flexible Authentication Secret Verification
	expectedSecret := os.Getenv("NOTIFICATION_SECRET")
	providedSecret := r.Header.Get("X-Notification-Secret")
	if providedSecret == "" {
		providedSecret = r.Header.Get("X-MacroDroid-Secret")
	}
	if providedSecret == "" {
		providedSecret = r.Header.Get("X-Secret")
	}
	if providedSecret == "" {
		providedSecret = r.URL.Query().Get("secret")
	}
	if providedSecret == "" {
		providedSecret = r.URL.Query().Get("notification_secret")
	}
	if providedSecret == "" {
		providedSecret = payload.Secret
	}
	if providedSecret == "" {
		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			providedSecret = strings.TrimPrefix(authHeader, "Bearer ")
		}
	}

	if expectedSecret == "" || subtle.ConstantTimeCompare([]byte(providedSecret), []byte(expectedSecret)) != 1 {
		log.Printf("[NOTIFICATION-LISTENER] Unauthorized request attempt from IP: %s (Provided secret: '%s')", r.RemoteAddr, providedSecret)
		writeError(w, http.StatusUnauthorized, "Unauthorized notification request")
		return
	}

	log.Printf("[NOTIFICATION-LISTENER] Received notification from %s | Title: %s | Text: %s", payload.App, payload.Title, payload.Text)

	fullContent := strings.TrimSpace(payload.Title + " " + payload.Text)
	if fullContent == "" {
		http.Error(w, "Empty payload", http.StatusBadRequest)
		return
	}

	// 3. Extract amount from text
	paidAmount, ok := parsePaymentAmount(fullContent)
	if !ok {
		log.Printf("[NOTIFICATION-LISTENER] No valid payment amount found in content: '%s'", fullContent)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ignored","reason":"No valid amount found"}`))
		return
	}

	log.Printf("[NOTIFICATION-LISTENER] Parsed incoming payment amount: Rp %.2f", paidAmount)

	// 4. Search for PENDING order matching this total_amount (with 0.5 tolerance)
	var orderID, orderNumber, customerEmail, pin, productID string
	var quantity int
	var totalAmount float64

	queryPending := `
		SELECT id, order_number, COALESCE(customer_email, 'Pembeli'), COALESCE(pin, '123456'), product_id::text, quantity, total_amount
		FROM orders
		WHERE status = 'PENDING' AND ABS(total_amount - $1) < 0.5
		ORDER BY created_at DESC
		LIMIT 1
	`
	err = h.orderRepo.GetDB().QueryRow(ctx, queryPending, paidAmount).Scan(&orderID, &orderNumber, &customerEmail, &pin, &productID, &quantity, &totalAmount)

	// If not found in PENDING, search recently EXPIRED orders (within 15 mins) for recovery
	isRecovered := false
	if err != nil {
		queryExpired := `
			SELECT id, order_number, COALESCE(customer_email, 'Pembeli'), COALESCE(pin, '123456'), product_id::text, quantity, total_amount
			FROM orders
			WHERE status = 'EXPIRED' AND ABS(total_amount - $1) < 0.5 AND updated_at >= NOW() - INTERVAL '15 minutes'
			ORDER BY created_at DESC
			LIMIT 1
		`
		errExpired := h.orderRepo.GetDB().QueryRow(ctx, queryExpired, paidAmount).Scan(&orderID, &orderNumber, &customerEmail, &pin, &productID, &quantity, &totalAmount)
		if errExpired == nil {
			isRecovered = true
			log.Printf("[NOTIFICATION-LISTENER] Recovered EXPIRED order %s for amount Rp %.2f!", orderNumber, paidAmount)
		} else {
			log.Printf("[NOTIFICATION-LISTENER] No matching pending/expired order found for amount Rp %.2f", paidAmount)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"not_found","message":"No matching pending order for this amount"}`))
			return
		}
	}

	// 5. Update order status to PAID
	if err := h.orderRepo.UpdateStatus(ctx, orderID, repository.OrderStatusPaid); err != nil {
		log.Printf("[NOTIFICATION-LISTENER] UpdateStatus error: %v", err)
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Mark stocks as SOLD
	soldCount, err := h.stockRepo.MarkSoldByOrderID(ctx, orderID)
	if err != nil || soldCount == 0 {
		log.Printf("[NOTIFICATION-LISTENER] MarkSoldByOrderID result: count=%d, err=%v", soldCount, err)
	}

	log.Printf("[NOTIFICATION-LISTENER] Order %s (Rp %.2f) SUCCESS via Notification Listener! (Recovered=%v, Stocks sold=%d)", orderNumber, totalAmount, isRecovered, soldCount)

	// Send Telegram alert
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

