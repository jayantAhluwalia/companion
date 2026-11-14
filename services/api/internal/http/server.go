// Package http exposes the admin/debug API. The user-facing surface is the
// Telegram bot; these endpoints exist for curl-driven testing and for
// triggering the nudge engine by hand.
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
	"companion/api/internal/engine"
	"companion/api/internal/telegram"
)

// Health reports whether the parts that matter are alive. Each returns ""
// when well or a short description of what is wrong.
type Health func(ctx context.Context) map[string]string

type Server struct {
	token   string
	ai      *ai.Client
	bot     *telegram.Bot
	log     *slog.Logger
	health_ Health // trailing underscore: "health" is the handler
}

func New(token string, aiClient *ai.Client, bot *telegram.Bot, health Health, log *slog.Logger) *Server {
	return &Server{token: token, ai: aiClient, bot: bot, health_: health, log: log}
}

// health answers 200 only when the database, the Telegram poll and the
// engine's ticker are all alive. A process that is up but deaf or stuck
// reports 503, so the platform's check and any outside monitor can see it.
func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	problems := map[string]string{}
	if s.health_ != nil {
		problems = s.health_(ctx)
	}
	if len(problems) == 0 {
		writeJSON(w, 200, map[string]string{"ok": "true"})
		return
	}
	problems["ok"] = "false"
	writeJSON(w, http.StatusServiceUnavailable, problems)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.health)
	mux.Handle("POST /v1/ai/interpret", s.auth(http.HandlerFunc(s.interpret)))
	mux.Handle("POST /v1/admin/nudge", s.auth(http.HandlerFunc(s.evaluate)))
	mux.Handle("POST /v1/admin/evaluate", s.auth(http.HandlerFunc(s.evaluate)))
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
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.RawContent) == "" {
		writeJSON(w, 400, map[string]string{"error": "raw_content required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	out, info, err := s.ai.Interpret(ctx, in.RawContent, ai.InterpretContext{Today: time.Now()})
	if err != nil {
		s.log.Error("interpret failed", "err", err, "stop", info.StopReason)
		writeJSON(w, 502, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, map[string]any{"interpretation": out, "call": info})
}

// evaluate runs the nudge engine once, now.
//
//	POST /v1/admin/evaluate            a normal check: gates, cooldowns and the score bar apply
//	POST /v1/admin/evaluate?dry=1      decide and log, but send nothing
//	POST /v1/admin/evaluate?force=1    skip the gates, as /nudge does
//	POST /v1/admin/nudge               same as force=1
func (s *Server) evaluate(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	opt := engine.Options{
		Trigger: "manual",
		Force:   q.Get("force") == "1" || strings.HasSuffix(r.URL.Path, "/nudge"),
		DryRun:  q.Get("dry") == "1",
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	res, err := s.bot.EvaluateNow(ctx, opt)
	if err != nil {
		s.log.Error("evaluate", "err", err)
		writeJSON(w, 409, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, res)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
