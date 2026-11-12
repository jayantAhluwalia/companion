// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	Port           string
	APIToken       string // static bearer token for the HTTP API (admin/debug endpoints)
	AnthropicKey   string
	Model          string
	TelegramToken  string
	TelegramChatID int64 // the one chat the bot serves; 0 until configured
	DatabaseURL    string
	BotEnabled     bool // false for local runs so two pollers never fight over one bot token
}

func Load() (Config, error) {
	c := Config{
		Port:          getenv("PORT", "8080"),
		APIToken:      os.Getenv("API_TOKEN"),
		AnthropicKey:  os.Getenv("ANTHROPIC_API_KEY"),
		Model:         getenv("ANTHROPIC_MODEL", "claude-opus-5-5"),
		TelegramToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		BotEnabled:    getenv("BOT_ENABLED", "true") != "false",
	}
	if v := os.Getenv("TELEGRAM_CHAT_ID"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return c, fmt.Errorf("TELEGRAM_CHAT_ID must be an integer: %w", err)
		}
		c.TelegramChatID = id
	}
	for name, v := range map[string]string{
		"API_TOKEN": c.APIToken, "ANTHROPIC_API_KEY": c.AnthropicKey,
		"TELEGRAM_BOT_TOKEN": c.TelegramToken, "DATABASE_URL": c.DatabaseURL,
	} {
		if v == "" {
			return c, fmt.Errorf("%s must be set", name)
		}
	}
	return c, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
