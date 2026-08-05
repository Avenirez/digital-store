package config

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// Server
	ServerPort     string
	AllowedOrigins string

	// PostgreSQL
	DatabaseURL string

	// Redis
	RedisURL string

	// Security
	AESKey      []byte // 32-byte key for AES-256-GCM
	AdminAPIKey string

	// Duitku
	DuitkuMerchantCode string
	DuitkuAPIKey       string
	DuitkuIsProduction bool
	DuitkuCallbackURL  string
	DuitkuReturnURL    string

	// Resend (Email)
	ResendAPIKey   string
	ResendFromEmail string
	ResendFromName  string

	// Telegram
	TelegramBotToken string
	TelegramChatID   string
}

// Load reads configuration from environment variables.
// It attempts to load a .env file first (for local development) but does not
// fail if the file is missing (production uses real env vars).
func Load() (*Config, error) {
	// Best-effort .env loading — ignore error if file doesn't exist
	_ = godotenv.Load()

	aesKeyHex := getEnv("AES_KEY", "")
	if aesKeyHex == "" {
		return nil, fmt.Errorf("config: AES_KEY is required")
	}
	aesKey, err := hex.DecodeString(aesKeyHex)
	if err != nil {
		return nil, fmt.Errorf("config: AES_KEY must be valid hex: %w", err)
	}
	if len(aesKey) != 32 {
		return nil, fmt.Errorf("config: AES_KEY must be exactly 32 bytes (64 hex chars), got %d bytes", len(aesKey))
	}

	duitkuProd, _ := strconv.ParseBool(getEnv("DUITKU_IS_PRODUCTION", "false"))

	cfg := &Config{
		ServerPort:     getEnv("SERVER_PORT", "8080"),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "http://localhost:4321"),

		DatabaseURL: requireEnv("DATABASE_URL"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379/0"),

		AESKey:      aesKey,
		AdminAPIKey: getEnv("ADMIN_API_KEY", ""),

		DuitkuMerchantCode: getEnv("DUITKU_MERCHANT_CODE", ""),
		DuitkuAPIKey:       getEnv("DUITKU_API_KEY", ""),
		DuitkuIsProduction: duitkuProd,
		DuitkuCallbackURL:  getEnv("DUITKU_CALLBACK_URL", ""),
		DuitkuReturnURL:    getEnv("DUITKU_RETURN_URL", ""),

		ResendAPIKey:   getEnv("RESEND_API_KEY", ""),
		ResendFromEmail: getEnv("RESEND_FROM_EMAIL", "noreply@example.com"),
		ResendFromName:  getEnv("RESEND_FROM_NAME", "Digital Store"),

		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:   getEnv("TELEGRAM_CHAT_ID", ""),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}

	return cfg, nil
}

// getEnv returns the value of an environment variable, or a default value if unset.
func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

// requireEnv returns the value of an environment variable, or empty string if unset.
// Validation is handled by the caller.
func requireEnv(key string) string {
	return os.Getenv(key)
}
