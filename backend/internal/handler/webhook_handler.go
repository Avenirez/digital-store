package handler

import (
	"log"
	"net/http"

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
		h.telegram.AlertNewOrder(order.OrderNumber, order.CustomerEmail, order.TotalAmount, productTitle)

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

	stocks, err := h.stockRepo.GetByOrderID(nil, order.ID)
	if err != nil {
		log.Printf("[WEBHOOK] Failed to fetch stocks for email: %v", err)
		return
	}

	product, _ := h.productRepo.GetByID(nil, order.ProductID)
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
