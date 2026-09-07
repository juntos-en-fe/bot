package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/jef/bot/internal/config"
	"github.com/jef/bot/internal/database"
	"github.com/jef/bot/internal/discord"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	db, err := database.Open(context.Background(), cfg.DatabasePath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer db.Close()

	bot, err := discord.New(cfg, logger, db)
	if err != nil {
		return fmt.Errorf("create Discord client: %w", err)
	}
	if err := bot.Open(); err != nil {
		return fmt.Errorf("connect to Discord: %w", err)
	}
	defer bot.Close()

	logger.Info("bot is running")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	logger.Info("shutdown requested")

	return nil
}
