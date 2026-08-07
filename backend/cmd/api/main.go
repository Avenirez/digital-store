package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"my-digital-store/backend/internal/config"
	"my-digital-store/backend/internal/database"
	"my-digital-store/backend/internal/handler"
	"my-digital-store/backend/internal/middleware"
	"my-digital-store/backend/internal/repository"
	"my-digital-store/backend/internal/service"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func main() {
	// ─── Load Configuration ──────────────────────────────
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	log.Printf("Config loaded (port=%s)", cfg.ServerPort)

	// ─── Root context with cancellation ──────────────────
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ─── Initialize PostgreSQL ───────────────────────────
	pgPool, err := database.NewPostgresPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pgPool.Close()
	log.Println("PostgreSQL connected")

	// ─── Initialize Redis ────────────────────────────────
	redisClient, err := database.NewRedisClient(ctx, cfg.RedisURL)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer redisClient.Close()
	log.Println("Redis connected")

	// ─── Initialize Services ─────────────────────────────
	cryptoSvc, err := service.NewCryptoService(cfg.AESKey)
	if err != nil {
		log.Fatalf("Failed to init crypto service: %v", err)
	}

	duitkuSvc := service.NewDuitkuService(
		cfg.DuitkuMerchantCode,
		cfg.DuitkuAPIKey,
		cfg.DuitkuIsProduction,
		cfg.DuitkuCallbackURL,
		cfg.DuitkuReturnURL,
	)

	telegramSvc := service.NewTelegramService(cfg.TelegramBotToken, cfg.TelegramChatID)
	resendSvc := service.NewResendService(cfg.ResendAPIKey, cfg.ResendFromEmail, cfg.ResendFromName)

	// ─── Initialize Repositories ─────────────────────────
	productRepo := repository.NewProductRepo(pgPool)
	orderRepo := repository.NewOrderRepo(pgPool)
	stockRepo := repository.NewStockRepo(pgPool)
	restockRepo := repository.NewRestockRepo(pgPool)

	// ─── Initialize Handlers ─────────────────────────────
	productHandler := handler.NewProductHandler(productRepo)
	orderHandler := handler.NewOrderHandler(pgPool, orderRepo, stockRepo, productRepo, duitkuSvc, cryptoSvc, telegramSvc, resendSvc)
	webhookHandler := handler.NewWebhookHandler(orderRepo, stockRepo, productRepo, duitkuSvc, cryptoSvc, telegramSvc, resendSvc)
	adminHandler := handler.NewAdminHandler(stockRepo, restockRepo, productRepo, cryptoSvc, resendSvc)
	restockHandler := handler.NewRestockHandler(restockRepo, productRepo)

	// ─── Initialize Middleware ───────────────────────────
	checkoutRateLimiter := middleware.NewRateLimiter(redisClient, 10, 1800, "rl:checkout") // 10 per 30 min (1800 sec)
	lookupRateLimiter := middleware.NewRateLimiter(redisClient, 10, 1800, "rl:lookup")       // 10 per 30 min (1800 sec)

	// ─── Setup Chi Router ────────────────────────────────
	r := chi.NewRouter()

	// Global middleware
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(middleware.CORS(cfg.AllowedOrigins))

	// Health check endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"ok","time":"%s"}`, time.Now().Format(time.RFC3339))
	})

	// ─── API v1 Routes ───────────────────────────────────
	setupV1Routes := func(r chi.Router) {
		// Product endpoints
		r.Get("/products", productHandler.ListProducts)
		r.Get("/products/{slug}", productHandler.GetProduct)

		// Checkout (rate-limited)
		r.With(checkoutRateLimiter.Middleware).Post("/checkout", orderHandler.Checkout)

		// Order endpoints (rate-limited: 10 percobaan / 30 menit)
		r.With(lookupRateLimiter.Middleware).Get("/orders/lookup", orderHandler.OrderLookup)
		r.With(lookupRateLimiter.Middleware).Get("/orders/download", orderHandler.DownloadCredentials)
		r.Post("/orders/simulate-pay", orderHandler.SimulatePay)

		// Webhook endpoints (Duitku callback & App Listener — no rate limit, no CORS)
		r.Post("/webhooks/duitku", webhookHandler.DuitkuCallback)
		r.Post("/webhooks/notification", webhookHandler.NotificationListener)

		// Restock subscription
		r.Post("/restock/subscribe", restockHandler.Subscribe)

		// Admin endpoints (API key protected)
		r.Route("/admin", func(r chi.Router) {
			r.Use(middleware.AdminAuth(cfg.AdminAPIKey))
			r.Post("/stocks/bulk", adminHandler.BulkImport)
		})
	}

	r.Route("/api/v1", setupV1Routes)
	r.Route("/v1", setupV1Routes)

	// ─── Create HTTP Server ──────────────────────────────
	srv := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ─── Start Background Workers ────────────────────────
	expireWorker := service.NewExpireWorker(orderRepo, stockRepo, 1*time.Minute, 10*time.Minute)
	go expireWorker.Start(ctx)

	// ─── Start Server ────────────────────────────────────
	go func() {
		log.Printf("Server starting on :%s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// ─── Graceful Shutdown ───────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("Received signal %s, shutting down gracefully...", sig)

	// Cancel background workers
	cancel()

	// Shutdown HTTP server with 10-second deadline
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced shutdown: %v", err)
	}

	log.Println("Server exited cleanly")
}
