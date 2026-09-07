package config

import (
	"log/slog"
	"testing"
)

func TestValueOr(t *testing.T) {
	t.Setenv("TEST_CONFIG_VALUE", "configured")
	if got := valueOr("TEST_CONFIG_VALUE", "fallback"); got != "configured" {
		t.Fatalf("valueOr returned %q, want configured", got)
	}
}

func TestParseLogLevel(t *testing.T) {
	level, err := parseLogLevel("debug")
	if err != nil {
		t.Fatalf("parseLogLevel returned an error: %v", err)
	}
	if level != slog.LevelDebug {
		t.Fatalf("parseLogLevel returned %v, want %v", level, slog.LevelDebug)
	}

	if _, err := parseLogLevel("verbose"); err == nil {
		t.Fatal("parseLogLevel accepted an invalid log level")
	}
}
