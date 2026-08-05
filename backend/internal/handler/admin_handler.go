package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"my-digital-store/backend/internal/repository"
	"my-digital-store/backend/internal/service"
)

// AdminHandler handles admin-only HTTP endpoints.
type AdminHandler struct {
	stockRepo   *repository.StockRepo
	restockRepo *repository.RestockRepo
	productRepo *repository.ProductRepo
	crypto      *service.CryptoService
	resend      *service.ResendService
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(
	stockRepo *repository.StockRepo,
	restockRepo *repository.RestockRepo,
	productRepo *repository.ProductRepo,
	crypto *service.CryptoService,
	resend *service.ResendService,
) *AdminHandler {
	return &AdminHandler{
		stockRepo:   stockRepo,
		restockRepo: restockRepo,
		productRepo: productRepo,
		crypto:      crypto,
		resend:      resend,
	}
}

// ─── Bulk Stock Import ───────────────────────────────────────────

// BulkImportRequest is the JSON body for POST /api/v1/admin/stocks/bulk.
type BulkImportRequest struct {
	ProductID string `json:"product_id"`
	// RawData is the text content with one credential per line:
	// email|password|additional_info (additional_info is optional)
	RawData string `json:"raw_data"`
}

// BulkImport handles POST /api/v1/admin/stocks/bulk
// Parses raw text input, encrypts passwords, and batch-inserts stocks.
func (h *AdminHandler) BulkImport(w http.ResponseWriter, r *http.Request) {
	var req BulkImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ProductID == "" || req.RawData == "" {
		writeError(w, http.StatusBadRequest, "product_id and raw_data are required")
		return
	}

	ctx := r.Context()

	// Verify product exists
	product, err := h.productRepo.GetByID(ctx, req.ProductID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Product not found")
		return
	}

	// Parse raw data: email|password|additional_info
	var items []repository.BulkInsertItem
	var parseErrors []string
	lineNum := 0

	scanner := bufio.NewScanner(strings.NewReader(req.RawData))
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue // Skip empty lines and comments
		}

		parts := strings.SplitN(line, "|", 3)
		if len(parts) < 2 {
			parseErrors = append(parseErrors, fmt.Sprintf("line %d: expected email|password format", lineNum))
			continue
		}

		email := strings.TrimSpace(parts[0])
		password := strings.TrimSpace(parts[1])
		additionalInfo := ""
		if len(parts) == 3 {
			additionalInfo = strings.TrimSpace(parts[2])
		}

		if email == "" || password == "" {
			parseErrors = append(parseErrors, fmt.Sprintf("line %d: email and password cannot be empty", lineNum))
			continue
		}

		// Encrypt password with AES-256-GCM
		encrypted, err := h.crypto.Encrypt(password)
		if err != nil {
			parseErrors = append(parseErrors, fmt.Sprintf("line %d: encryption failed: %v", lineNum, err))
			continue
		}

		items = append(items, repository.BulkInsertItem{
			Email:             email,
			PasswordEncrypted: encrypted,
			AdditionalInfo:    additionalInfo,
		})
	}

	if len(items) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":        "No valid items to import",
			"parse_errors": parseErrors,
		})
		return
	}

	// Batch insert into database
	inserted, err := h.stockRepo.BulkInsert(ctx, req.ProductID, items)
	if err != nil {
		log.Printf("[ADMIN] BulkInsert error: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to insert stocks")
		return
	}

	log.Printf("[ADMIN] Bulk import: %d items inserted for product %s (%s)", inserted, req.ProductID, product.Title)

	// Trigger restock notification worker asynchronously
	go h.triggerRestockAlerts(req.ProductID, product.Title, product.Slug)

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"inserted":     inserted,
		"parse_errors": parseErrors,
		"product":      product.Title,
	})
}

// triggerRestockAlerts sends email notifications to all pending subscribers.
func (h *AdminHandler) triggerRestockAlerts(productID, productTitle, productSlug string) {
	if !h.resend.IsConfigured() {
		return
	}

	ctx := context.Background()
	_ = ctx

	subs, err := h.restockRepo.GetPendingByProduct(nil, productID)
	if err != nil {
		log.Printf("[RESTOCK] Failed to fetch subscribers: %v", err)
		return
	}

	if len(subs) == 0 {
		return
	}

	log.Printf("[RESTOCK] Sending alerts to %d subscribers for %s", len(subs), productTitle)

	// TODO: Build proper product URL from frontend config
	productURL := fmt.Sprintf("/produk/%s", productSlug)

	var notifiedIDs []string
	for _, sub := range subs {
		if err := h.resend.SendRestockAlert(sub.Email, productTitle, productURL); err != nil {
			log.Printf("[RESTOCK] Failed to email %s: %v", sub.Email, err)
			continue
		}
		notifiedIDs = append(notifiedIDs, sub.ID)
	}

	if len(notifiedIDs) > 0 {
		if err := h.restockRepo.MarkNotified(nil, notifiedIDs); err != nil {
			log.Printf("[RESTOCK] Failed to mark notified: %v", err)
		}
		log.Printf("[RESTOCK] %d subscribers notified for %s", len(notifiedIDs), productTitle)
	}
}
