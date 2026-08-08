package handler

import (
	"bufio"
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
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(
	stockRepo *repository.StockRepo,
	restockRepo *repository.RestockRepo,
	productRepo *repository.ProductRepo,
	crypto *service.CryptoService,
) *AdminHandler {
	return &AdminHandler{
		stockRepo:   stockRepo,
		restockRepo: restockRepo,
		productRepo: productRepo,
		crypto:      crypto,
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
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // Batas 10MB
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

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"inserted":     inserted,
		"parse_errors": parseErrors,
		"product":      product.Title,
	})
}

// ─── Fix Passwords (Re-encrypt existing stocks) ─────────────────

// FixPasswordsRequest is the JSON body for POST /api/v1/admin/stocks/fix-passwords.
type FixPasswordsRequest struct {
	// RawData is the text content with one credential per line:
	// email|password (updates ALL stocks matching each email)
	RawData string `json:"raw_data"`
}

// FixPasswords handles POST /api/v1/admin/stocks/fix-passwords
// Re-encrypts passwords for existing stocks (any status: SOLD, RESERVED, AVAILABLE).
// This fixes stocks that were inserted with invalid ciphertext.
func (h *AdminHandler) FixPasswords(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 10<<20)
	var req FixPasswordsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.RawData == "" {
		writeError(w, http.StatusBadRequest, "raw_data is required (format: email|password per line)")
		return
	}

	ctx := r.Context()

	var results []map[string]interface{}
	var totalUpdated int64

	scanner := bufio.NewScanner(strings.NewReader(req.RawData))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "|", 2)
		if len(parts) < 2 {
			results = append(results, map[string]interface{}{
				"line":   lineNum,
				"status": "error",
				"error":  "expected email|password format",
			})
			continue
		}

		email := strings.TrimSpace(parts[0])
		password := strings.TrimSpace(parts[1])

		if email == "" || password == "" {
			results = append(results, map[string]interface{}{
				"line":   lineNum,
				"email":  email,
				"status": "error",
				"error":  "email and password cannot be empty",
			})
			continue
		}

		// Encrypt password with current AES key
		encrypted, err := h.crypto.Encrypt(password)
		if err != nil {
			results = append(results, map[string]interface{}{
				"line":   lineNum,
				"email":  email,
				"status": "error",
				"error":  fmt.Sprintf("encryption failed: %v", err),
			})
			continue
		}

		// Update ALL stocks matching this email (any status)
		tag, err := h.stockRepo.UpdatePasswordByEmail(ctx, email, encrypted)
		if err != nil {
			results = append(results, map[string]interface{}{
				"line":   lineNum,
				"email":  email,
				"status": "error",
				"error":  fmt.Sprintf("database update failed: %v", err),
			})
			continue
		}

		results = append(results, map[string]interface{}{
			"line":         lineNum,
			"email":        email,
			"status":       "ok",
			"rows_updated": tag,
		})
		totalUpdated += tag
	}

	log.Printf("[ADMIN] FixPasswords: %d total rows updated across %d accounts", totalUpdated, len(results))

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_updated": totalUpdated,
		"details":       results,
	})
}
