package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"companion/api/internal/ai"
	"companion/api/internal/config"
	apihttp "companion/api/internal/http"
	"companion/api/internal/telegram"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	aiClient := ai.New(cfg.AnthropicKey, cfg.Model)
	bot := telegram.NewBot(telegram.NewClient(cfg.TelegramToken), aiClient, cfg.TelegramChatID, log)
	go bot.Run(ctx)

	srv := apihttp.New(cfg.APIToken, aiClient, bot, log)
	httpServer := &http.Server{Addr: ":" + cfg.Port, Handler: srv.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Info("listening", "port", cfg.Port, "model", cfg.Model, "telegram_chat_locked", cfg.TelegramChatID != 0)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("serve", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	log.Info("stopped")
}
