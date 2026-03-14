package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	TelegramBotToken string
	TelegramChatID   string
	ScanIntervalSec  int
	MinVolume24H     float64
	DashboardPort    int
}

func Load() (*Config, error) {
	if err := loadEnvFile(".env"); err != nil {
		return nil, fmt.Errorf("failed to load .env: %w", err)
	}

	cfg := &Config{
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID:   os.Getenv("TELEGRAM_CHAT_ID"),
		ScanIntervalSec:  getEnvInt("SCAN_INTERVAL_SECONDS", 300),
		MinVolume24H:     getEnvFloat("MIN_VOLUME_24H_USD", 10_000_000),
		DashboardPort:    getEnvInt("DASHBOARD_PORT", 8081),
	}

	if cfg.TelegramBotToken == "" || cfg.TelegramBotToken == "your_bot_token_here" {
		return nil, fmt.Errorf("TELEGRAM_BOT_TOKEN is not set")
	}
	if cfg.TelegramChatID == "" || cfg.TelegramChatID == "your_chat_id_here" {
		return nil, fmt.Errorf("TELEGRAM_CHAT_ID is not set")
	}

	return cfg, nil
}

func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no .env file is okay, use system env
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
	return scanner.Err()
}

func getEnvInt(key string, def int) int {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return def
	}
	return n
}

func getEnvFloat(key string, def float64) float64 {
	val := os.Getenv(key)
	if val == "" {
		return def
	}
	f, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return def
	}
	return f
}
