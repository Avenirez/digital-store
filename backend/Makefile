# ─── Digital Account Store Backend ───────────────────────────────

.PHONY: build run migrate test lint clean

# ─── Variables ───────────────────────────────────────
BINARY_NAME=digitalstore
BUILD_DIR=./bin
MAIN_PATH=./cmd/api

# ─── Build ───────────────────────────────────────────
# Compiles for Ubuntu 24.04 LTS (linux/amd64) as a static binary
build:
	@echo "→ Building $(BINARY_NAME) for linux/amd64..."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build \
		-ldflags="-s -w" \
		-o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "✓ Binary: $(BUILD_DIR)/$(BINARY_NAME)"

# Build for current OS (development)
build-dev:
	@echo "→ Building $(BINARY_NAME) for development..."
	go build -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PATH)
	@echo "✓ Binary: $(BUILD_DIR)/$(BINARY_NAME)"

# ─── Run ─────────────────────────────────────────────
run:
	@echo "→ Starting server..."
	go run $(MAIN_PATH)

# ─── Database ────────────────────────────────────────
# Run SQL migrations against the database
# Usage: make migrate DATABASE_URL="postgres://user:pass@localhost:5432/digitalstore"
migrate:
	@echo "→ Running migrations..."
	psql "$(DATABASE_URL)" -f ./migrations/001_init.sql
	@echo "✓ Migrations complete"

# ─── Testing & Linting ──────────────────────────────
test:
	@echo "→ Running tests..."
	go test -v -race -count=1 ./...

lint:
	@echo "→ Running linter..."
	go vet ./...

# ─── Dependencies ───────────────────────────────────
deps:
	@echo "→ Downloading dependencies..."
	go mod download
	go mod tidy
	@echo "✓ Dependencies ready"

# ─── Clean ──────────────────────────────────────────
clean:
	@echo "→ Cleaning build artifacts..."
	rm -rf $(BUILD_DIR)
	@echo "✓ Clean"
