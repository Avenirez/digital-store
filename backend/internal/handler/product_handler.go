package handler

import (
	"encoding/json"
	"net/http"

	"my-digital-store/backend/internal/repository"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// ProductHandler handles product-related HTTP endpoints.
type ProductHandler struct {
	productRepo *repository.ProductRepo
}

// NewProductHandler creates a new ProductHandler.
func NewProductHandler(productRepo *repository.ProductRepo) *ProductHandler {
	return &ProductHandler{productRepo: productRepo}
}

// ListProducts handles GET /api/v1/products
// Returns all active products with their live stock counts.
func (h *ProductHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.productRepo.ListActive(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to fetch products")
		return
	}

	if products == nil {
		products = []repository.Product{}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"products": products,
		"count":    len(products),
	})
}

// GetProduct handles GET /api/v1/products/{slug}
// Returns a single product with stock count.
func (h *ProductHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		writeError(w, http.StatusBadRequest, "Product slug is required")
		return
	}

	product, err := h.productRepo.GetBySlug(r.Context(), slug)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "Product not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to fetch product")
		return
	}

	writeJSON(w, http.StatusOK, product)
}

// ─── Response Helpers ────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
