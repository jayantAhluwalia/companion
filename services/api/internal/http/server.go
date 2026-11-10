// Package http exposes the admin/debug API. The user-facing surface is the
// Telegram bot; these endpoints exist for curl-driven testing and, later,
// the manual evaluator trigger.
package http

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"companion/api/internal/ai"
	"companion/api/internal/telegram"
)

type Server struct {
	token string
	ai    *ai.Client
	bot   *telegram.Bot
	log   *slog.Logger
}

func New(token string, aiClient *ai.Client, bot *telegram.Bot, log *slog.Logger) *Server {
	return &Server{token: token, ai: aiClient, bot: bot, log: log}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"ok": "true"}) })
	mux.Handle("POST /v1/ai/interpret", s.auth(http.HandlerFunc(s.interpret)))
	mux.Handle("POST /v1/admin/nudge", s.auth(http.HandlerFunc(s.sampleNudge)))
	return s.logging(mux)
}

// auth is a constant-time comparison against the single shared bearer token.
func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(s.token)) != 1 {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.log.Info("request", "method", r.Method, "path", r.URL.Path, "ms", time.Since(start).Milliseconds())
	})
}

func (s *Server) interpret(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RawContent string `json:"raw_content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.RawContent) == "" {
		writeJSON(w, 400, map[string]string{"error": "raw_content required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	out, info, err := s.ai.Interpret(ctx, in.RawContent, time.Now())
	if err != nil {
		s.log.Error("interpret failed", "err", err, "stop", info.StopReason)
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"interpretation": out, "call": info})
}

// sampleNudge lets a curl call push a message to the user's chat, the same
// path the evaluator will use in Phase 2.
func (s *Server) sampleNudge(w http.ResponseWriter, r *http.Request) {
	if s.bot.AllowedChat() == 0 {
		writeJSON(w, 409, map[string]string{"error": "TELEGRAM_CHAT_ID not configured"})
		return
	}
	s.bot.SendSampleNudge(r.Context())
	writeJSON(w, 200, map[string]string{"status": "sent"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
