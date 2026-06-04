package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DatabaseURL string

	// Mock bank
	BankBaseURL string

	// Retry settings
	MaxRetries       int
	RetryBaseDelayMS int
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	cfg := &Config{
		Port:             getEnv("PORT", "8088"),
		DatabaseURL:      getEnv("DATABASE_URL", ""),
		BankBaseURL:      getEnv("BANK_BASE_URL", "http://localhost:8787"),
		MaxRetries:       getEnvInt("MAX_RETRIES", 3),
		RetryBaseDelayMS: getEnvInt("RETRY_BASE_DELAY_MS", 100),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.BankBaseURL == "" {
		return fmt.Errorf("BANK_BASE_URL is required")
	}
	return nil
}

// DSN returns the connection string
func (c *Config) DSN() string {
	return c.DatabaseURL
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	val := os.Getenv(key)
	if val == "" {
		return fallback
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return fallback
	}
	return i
}
