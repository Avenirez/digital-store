package service

import (
	"context"
	"log"
	"time"

	"my-digital-store/backend/internal/repository"
)

// ExpireWorker runs a background ticker that automatically expires pending
// orders older than the threshold and releases their reserved stocks.
type ExpireWorker struct {
	orderRepo *repository.OrderRepo
	stockRepo *repository.StockRepo
	interval  time.Duration
	threshold time.Duration
}

// NewExpireWorker creates a new ExpireWorker.
// interval: how often to check (e.g., 1 minute)
// threshold: how old a pending order must be to expire (e.g., 15 minutes)
func NewExpireWorker(orderRepo *repository.OrderRepo, stockRepo *repository.StockRepo, interval, threshold time.Duration) *ExpireWorker {
	return &ExpireWorker{
		orderRepo: orderRepo,
		stockRepo: stockRepo,
		interval:  interval,
		threshold: threshold,
	}
}

// Start begins the expire worker loop. It runs until the context is cancelled.
// Call this in a goroutine: go expireWorker.Start(ctx)
func (w *ExpireWorker) Start(ctx context.Context) {
	log.Printf("[EXPIRE-WORKER] Started (interval=%s, threshold=%s)", w.interval, w.threshold)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("[EXPIRE-WORKER] Shutting down")
			return
		case <-ticker.C:
			w.run(ctx)
		}
	}
}

// run executes one tick of the expire worker.
func (w *ExpireWorker) run(ctx context.Context) {
	// 1. Expire pending orders older than threshold
	expiredIDs, err := w.orderRepo.ExpirePendingOrders(ctx, w.threshold)
	if err != nil {
		log.Printf("[EXPIRE-WORKER] ExpirePendingOrders error: %v", err)
		return
	}

	if len(expiredIDs) == 0 {
		return // Nothing to do
	}

	log.Printf("[EXPIRE-WORKER] Expired %d pending orders", len(expiredIDs))

	// 2. Release reserved stocks for each expired order
	var totalReleased int64
	for _, orderID := range expiredIDs {
		released, err := w.stockRepo.ReleaseByOrderID(ctx, orderID)
		if err != nil {
			log.Printf("[EXPIRE-WORKER] ReleaseByOrderID error for %s: %v", orderID, err)
			continue
		}
		totalReleased += released
	}

	if totalReleased > 0 {
		log.Printf("[EXPIRE-WORKER] Released %d reserved stocks back to AVAILABLE", totalReleased)
	}
}
