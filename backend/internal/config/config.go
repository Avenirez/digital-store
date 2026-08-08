package config

import (
	"encoding/hex"
	"fmt"
	"os"

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
	AESKey             []byte // 32-byte key for AES-256-GCM
	AdminAPIKey        string
	NotificationSecret string

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

	cfg := &Config{
		ServerPort:     getEnv("SERVER_PORT", "8080"),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "http://localhost:4321"),

		DatabaseURL: requireEnv("DATABASE_URL"),
		RedisURL:    getEnv("REDIS_URL", "redis://localhost:6379/0"),

		AESKey:             aesKey,
		AdminAPIKey:        getEnv("ADMIN_API_KEY", ""),
		NotificationSecret: getEnv("NOTIFICATION_SECRET", ""),

		TelegramBotToken: getEnv("TELEGRAM_BOT_TOKEN", ""),
		TelegramChatID:   getEnv("TELEGRAM_CHAT_ID", ""),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("config: DATABASE_URL is required")
	}

	if cfg.NotificationSecret == "" {
		return nil, fmt.Errorf("config: NOTIFICATION_SECRET is required for security")
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
func requireEnv(key string) string {
	return os.Getenv(key)
}
