package handler

import (
	"encoding/json"
	"net/http"

	"my-digital-store/backend/internal/repository"
)

// RestockHandler handles restock subscription endpoints.
type RestockHandler struct {
	restockRepo *repository.RestockRepo
	productRepo *repository.ProductRepo
}

// NewRestockHandler creates a new RestockHandler.
func NewRestockHandler(restockRepo *repository.RestockRepo, productRepo *repository.ProductRepo) *RestockHandler {
	return &RestockHandler{
		restockRepo: restockRepo,
		productRepo: productRepo,
	}
}

// SubscribeRequest is the JSON body for POST /api/v1/restock/subscribe.
type SubscribeRequest struct {
	ProductSlug string `json:"product_slug"`
	Email       string `json:"email"`
}

// Subscribe handles POST /api/v1/restock/subscribe
// Registers an email for restock notifications on a specific product.
func (h *RestockHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	var req SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.ProductSlug == "" || req.Email == "" {
		writeError(w, http.StatusBadRequest, "product_slug and email are required")
		return
	}

	ctx := r.Context()

	// Verify product exists
	product, err := h.productRepo.GetBySlug(ctx, req.ProductSlug)
	if err != nil {
		writeError(w, http.StatusNotFound, "Product not found")
		return
	}

	// Subscribe (idempotent — won't duplicate)
	if err := h.restockRepo.Subscribe(ctx, product.ID, req.Email); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to subscribe")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"message": "You will be notified when this product is back in stock",
		"product": product.Title,
	})
}
