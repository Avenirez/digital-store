package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// TelegramService sends asynchronous alerts via the Telegram Bot API.
type TelegramService struct {
	botToken   string
	chatID     string
	httpClient *http.Client
}

// NewTelegramService creates a new TelegramService.
// If botToken or chatID is empty, alerts will be silently skipped.
func NewTelegramService(botToken, chatID string) *TelegramService {
	return &TelegramService{
		botToken: botToken,
		chatID:   chatID,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// IsConfigured returns true if Telegram credentials are set.
func (t *TelegramService) IsConfigured() bool {
	return t.botToken != "" && t.chatID != ""
}

// sendMessagePayload is the JSON body for Telegram's sendMessage API.
type sendMessagePayload struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
}

// SendAlert sends a message to the configured Telegram chat asynchronously.
// This method launches a goroutine and does not block.
func (t *TelegramService) SendAlert(message string) {
	if !t.IsConfigured() {
		return
	}

	go func() {
		if err := t.sendMessage(message); err != nil {
			log.Printf("[TELEGRAM] Failed to send alert: %v", err)
		}
	}()
}

// sendMessage sends a message synchronously via the Telegram Bot API.
func (t *TelegramService) sendMessage(message string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.botToken)

	payload := sendMessagePayload{
		ChatID:    t.chatID,
		Text:      message,
		ParseMode: "HTML",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("telegram.sendMessage: marshal error: %w", err)
	}

	resp, err := t.httpClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("telegram.sendMessage: HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("telegram.sendMessage: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// ─── Pre-built Alert Templates ───────────────────────────────────

// AlertNewOrder sends an alert when a new order is paid.
func (t *TelegramService) AlertNewOrder(orderNumber, customerEmail string, amount float64, productTitle string) {
	msg := fmt.Sprintf(
		"💰 <b>New Order Paid!</b>\n\n"+
			"📦 Order: <code>%s</code>\n"+
			"📧 Customer: %s\n"+
			"💵 Amount: Rp %,.0f\n"+
			"🏷️ Product: %s",
		orderNumber, customerEmail, amount, productTitle,
	)
	t.SendAlert(msg)
}

// AlertLowStock sends an alert when product inventory drops below threshold.
func (t *TelegramService) AlertLowStock(productTitle string, remaining int) {
	msg := fmt.Sprintf(
		"⚠️ <b>Low Stock Warning!</b>\n\n"+
			"🏷️ Product: %s\n"+
			"📊 Remaining: <b>%d</b> items\n\n"+
			"Please restock soon!",
		productTitle, remaining,
	)
	t.SendAlert(msg)
}
