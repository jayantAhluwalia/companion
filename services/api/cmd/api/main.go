package main

import (
	"cmp"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // the alpine image has no zoneinfo; local time needs it

	"companion/api/internal/ai"
	"companion/api/internal/config"
	"companion/api/internal/engine"
	apihttp "companion/api/internal/http"
	"companion/api/internal/signals"
	"companion/api/internal/store"
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

	migCtx, cancelMig := context.WithTimeout(ctx, 60*time.Second)
	if err := store.Migrate(migCtx, cfg.DatabaseURL, cfg.DBSchema); err != nil {
		log.Error("migrate", "err", err)
		os.Exit(1)
	}
	cancelMig()
	db, err := store.Open(ctx, cfg.DatabaseURL, cfg.DBSchema)
	if err != nil {
		log.Error("db", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	log.Info("database ready", "schema", cmp.Or(cfg.DBSchema, "public"))

	aiClient := ai.New(cfg.AnthropicKey, cfg.Model)
	weather := signals.NewWeather()
	eng := engine.New(db, aiClient, weather, engine.Config{MonthlyBudgetUSD: cfg.MonthlyBudget, ScoutsPerDay: cfg.ScoutsPerDay}, log)
	tg := telegram.NewClient(cfg.TelegramToken)
	if cfg.DrySend {
		tg.Dry = func(method string, payload any) { log.Info("dry send", "method", method, "payload", payload) }
		log.Warn("DRY_SEND is on: outgoing Telegram messages are logged, not sent")
	}
	bot := telegram.NewBot(tg, aiClient, db, eng, weather, cfg.TelegramChatID, log)
	eng.SetSender(bot)
	eng.SetAlert(bot.Alert)
	go eng.Backfill(ctx)
	if cfg.BotEnabled {
		go bot.Run(ctx)
		go eng.Run(ctx)
	} else {
		// A local run must not poll Telegram or nudge on a timer: the
		// deployed process owns both. The admin endpoints still work.
		log.Warn("telegram polling and the nudge ticker are disabled (BOT_ENABLED=false)")
	}

	started := time.Now()
	health := func(ctx context.Context) map[string]string {
		problems := map[string]string{}
		if err := db.Ping(ctx); err != nil {
			problems["database"] = "not answering"
		}
		if !cfg.BotEnabled {
			return problems // a local run neither polls nor ticks
		}
		// Allow a grace period after start before calling either one stale.
		if last := bot.LastPoll(); time.Since(started) > 2*time.Minute && time.Since(last) > 2*time.Minute {
			problems["telegram"] = "no successful poll for over two minutes"
		}
		if last := eng.LastTick(); time.Since(started) > 45*time.Minute && time.Since(last) > 45*time.Minute {
			problems["engine"] = "no completed tick for over 45 minutes"
		}
		return problems
	}
	srv := apihttp.New(cfg.APIToken, aiClient, bot, health, log)
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
