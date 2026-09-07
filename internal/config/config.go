package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config holds startup-only application settings.
type Config struct {
	DiscordToken         string
	DiscordApplicationID string
	DiscordGuildID       string
	DatabasePath         string
	LogLevel             slog.Level
}

// Load reads .env when present, then validates environment configuration.
// Existing environment variables take precedence over values in .env.
func Load() (Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("read .env: %w", err)
	}

	cfg := Config{
		DiscordToken:         strings.TrimSpace(os.Getenv("DISCORD_TOKEN")),
		DiscordApplicationID: strings.TrimSpace(os.Getenv("DISCORD_APPLICATION_ID")),
		DiscordGuildID:       strings.TrimSpace(os.Getenv("DISCORD_GUILD_ID")),
		DatabasePath:         valueOr("DATABASE_PATH", "data/bot.db"),
		LogLevel:             slog.LevelInfo,
	}

	if cfg.DiscordToken == "" {
		return Config{}, errors.New("DISCORD_TOKEN is required")
	}
	if cfg.DiscordApplicationID == "" {
		return Config{}, errors.New("DISCORD_APPLICATION_ID is required")
	}

	level, err := parseLogLevel(strings.TrimSpace(os.Getenv("LOG_LEVEL")))
	if err != nil {
		return Config{}, err
	}
	cfg.LogLevel = level

	return cfg, nil
}

func parseLogLevel(value string) (slog.Level, error) {
	if value == "" {
		return slog.LevelInfo, nil
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(value)); err != nil {
		return 0, fmt.Errorf("invalid LOG_LEVEL %q: %w", value, err)
	}
	return level, nil
}

func valueOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
