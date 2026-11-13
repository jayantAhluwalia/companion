package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"companion/api/internal/ai"
	"companion/api/internal/signals"
	"companion/api/internal/store"
)

// Outgoing is a nudge ready for delivery. It is channel-neutral: Telegram
// renders it today, APNs will render the same fields later.
type Outgoing struct {
	ChatID    int64
	ThoughtID int64
	NudgeID   int64
	Type      ai.ThoughtType
	Title     string
	Message   string
	Step      string
	Footnote  string
	Links     []ai.Source
}

// Sender delivers a nudge and returns the channel's message id.
type Sender interface {
	SendNudge(ctx context.Context, n Outgoing) (string, error)
}

type Config struct {
	MonthlyBudgetUSD float64 // model spend above this stops proactive calls
	ScoutsPerDay     int     // web lookups allowed per day
}

type Engine struct {
	db      *store.Store
	ai      *ai.Client
	weather *signals.Weather
	sender  Sender
	log     *slog.Logger
	cfg     Config
	mu      sync.Mutex // one evaluation at a time in this process
}

func New(db *store.Store, aiClient *ai.Client, weather *signals.Weather, cfg Config, log *slog.Logger) *Engine {
	return &Engine{db: db, ai: aiClient, weather: weather, cfg: cfg, log: log}
}

func (e *Engine) SetSender(s Sender) { e.sender = s }

type Options struct {
	Trigger string // ticker | manual | message
	Force   bool   // skip gates, cooldowns and the threshold: the user asked for a nudge
	DryRun  bool   // decide, log, but do not send
}

type Result struct {
	RunID     int64    `json:"run_id"`
	Outcome   string   `json:"outcome"` // nudged | none | blocked | dry_run | error
	Reason    string   `json:"reason"`
	ThoughtID int64    `json:"thought_id,omitempty"`
	Message   string   `json:"message,omitempty"`
	Step      string   `json:"suggested_step,omitempty"`
	Scores    []Scored `json:"scores,omitempty"`
}

const (
	tickEvery        = 30 * time.Minute
	nudgeTTL         = 24 * time.Hour // unanswered after this counts as ignored
	modelBackoff     = 3 * time.Hour  // after the model said "none", do not ask again for a while
	maxCandidates    = 3
	scoutsPerRun     = 1
	scoutBudgetShare = 0.8        // lookups stop at 80% of the monthly budget; nudges keep the rest
	lockKey          = 0x636f6d70 // "comp"
)

// Run ticks on the half hour until ctx is cancelled.
func (e *Engine) Run(ctx context.Context) {
	for {
		now := time.Now()
		next := now.Truncate(tickEvery).Add(tickEvery).Add(5 * time.Second)
		select {
		case <-ctx.Done():
			return
		case <-time.After(next.Sub(now)):
		}
		e.tick(ctx)
	}
}

func (e *Engine) tick(ctx context.Context) {
	unlock, ok, err := e.db.TryLock(ctx, lockKey)
	if err != nil {
		e.log.Error("evaluator lock", "err", err)
		return
	}
	if !ok {
		e.log.Info("another process is evaluating; skipping this tick")
		return
	}
	defer unlock()

	users, err := e.db.Users(ctx)
	if err != nil {
		e.log.Error("list users", "err", err)
		return
	}
	for _, u := range users {
		tctx, cancel := context.WithTimeout(ctx, 6*time.Minute)
		res, err := e.Evaluate(tctx, u, Options{Trigger: "ticker"})
		cancel()
		if err != nil {
			e.log.Error("evaluate", "err", err, "user", u.ID)
			continue
		}
		e.log.Info("evaluated", "user", u.ID, "outcome", res.Outcome, "reason", res.Reason, "thought", res.ThoughtID)
	}
	_ = e.db.PruneBlockedRuns(ctx, 72*time.Hour)
}

// Evaluate runs one decision for one user. Every run is logged, including
// the ones that send nothing, so "why didn't it nudge me?" always has an answer.
func (e *Engine) Evaluate(ctx context.Context, user store.User, opt Options) (Result, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	loc, err := time.LoadLocation(user.Timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)

	runID, err := e.db.StartRun(ctx, user.ID, opt.Trigger)
	if err != nil {
		return Result{}, err
	}
	res := Result{RunID: runID}
	var decisionLog any
	finish := func(outcome, reason string) (Result, error) {
		res.Outcome, res.Reason = outcome, reason
		return res, e.db.FinishRun(ctx, runID, len(res.Scores), res.Scores, decisionLog, outcome, reason)
	}

	prefs, err := e.db.GetPrefs(ctx, user.ID)
	if err != nil {
		return finish("error", "could not load preferences: "+err.Error())
	}

	// Housekeeping first, so the gates see current state.
	if _, err := e.db.ExpireNudges(ctx, user.ID, nudgeTTL); err != nil {
		e.log.Warn("expire nudges", "err", err)
	}
	if err := e.db.WakeSnoozed(ctx, user.ID); err != nil {
		e.log.Warn("wake snoozed", "err", err)
	}

	// 1. Gates: is now an acceptable moment at all?
	if !opt.Force {
		sentToday, lastSent, err := e.db.NudgeStats(ctx, user.ID, day(now))
		if err != nil {
			return finish("error", "could not read nudge history: "+err.Error())
		}
		if reason := Gate(now, prefs, sentToday, lastSent); reason != "" {
			return finish("blocked", reason)
		}
	}

	// 2. Candidates.
	thoughts, err := e.db.Candidates(ctx, user.ID)
	if err != nil {
		return finish("error", "could not load thoughts: "+err.Error())
	}
	if len(thoughts) == 0 {
		return finish("none", "no active thoughts")
	}
	ids := make([]int64, len(thoughts))
	for i, t := range thoughts {
		ids[i] = t.ID
	}
	stats, err := e.db.ThoughtStats(ctx, ids)
	if err != nil {
		return finish("error", "could not load thought stats: "+err.Error())
	}

	// 3. Signals and scores.
	budget := e.scoutBudget(ctx, now)
	byID := map[int64]store.Thought{}
	var eligible []Scored
	for _, t := range thoughts {
		byID[t.ID] = t
		if hold := onCooldown(t, now); hold != "" && !opt.Force {
			res.Scores = append(res.Scores, Scored{ThoughtID: t.ID, Title: t.Interpretation.Title, Held: hold})
			continue
		}
		hooks := e.checkHooks(ctx, user, prefs, t, now, budget)
		sc := score(t, stats[t.ID], hooks, prefs, now)
		if hold := tooFresh(t, hooks, now); hold != "" && !opt.Force {
			sc.Held = hold
			res.Scores = append(res.Scores, sc)
			continue
		}
		eligible = append(eligible, sc)
		res.Scores = append(res.Scores, sc)
	}
	sort.SliceStable(res.Scores, func(i, j int) bool { return res.Scores[i].Score > res.Scores[j].Score })
	sort.SliceStable(eligible, func(i, j int) bool { return eligible[i].Score > eligible[j].Score })
	if len(eligible) == 0 {
		return finish("none", "every thought is resting: recently captured, or recently nudged")
	}

	var short []Scored
	for _, sc := range eligible {
		if (sc.Score >= Threshold || opt.Force) && len(short) < maxCandidates {
			short = append(short, sc)
		}
	}
	if len(short) == 0 {
		best := eligible[0]
		return finish("none", fmt.Sprintf("nothing is timely enough yet (best: #%d at %.2f, bar is %.2f)", best.ThoughtID, best.Score, Threshold))
	}

	// 4. Spend guards before asking the model.
	if !opt.Force {
		if cost, _, err := e.db.MonthlyCostUSD(ctx); err == nil && cost >= e.cfg.MonthlyBudgetUSD {
			return finish("none", fmt.Sprintf("monthly AI budget reached ($%.2f of $%.2f)", cost, e.cfg.MonthlyBudgetUSD))
		}
		if last, err := e.db.LastModelPass(ctx, user.ID); err == nil && last != nil && now.Sub(*last) < modelBackoff {
			return finish("none", "the model passed recently; asking again after "+last.Add(modelBackoff).In(loc).Format("15:04"))
		}
	}

	// 5. The model proposes.
	in := ai.DecideInput{
		Now:     now.Format("Monday 2 January 2006, 15:04"),
		Routine: describeRoutine(prefs),
		Home:    prefs.HomeCity,
		Forced:  opt.Force,
	}
	for _, sc := range short {
		t := byID[sc.ThoughtID]
		st := stats[t.ID]
		c := ai.DecideCandidate{
			ThoughtID: t.ID, OriginalWords: t.RawContent, Type: t.Interpretation.Type, Title: t.Interpretation.Title,
			Description: t.Interpretation.Description, DesiredOutcome: t.Interpretation.DesiredOutcome,
			Timeframe: t.Interpretation.Timeframe, NextSteps: t.Interpretation.NextSteps,
			AgeDays: int(now.Sub(t.CreatedAt).Hours() / 24), NudgesSent: t.NudgeCount, IgnoredInARow: t.IgnoredStreak,
			LastResponse: st.LastResponse, Score: sc.Score, Signals: []ai.DecideSignal{},
		}
		if c.NextSteps == nil {
			c.NextSteps = []ai.NextStep{}
		}
		for _, h := range sc.Hooks {
			c.Signals = append(c.Signals, ai.DecideSignal{Kind: h.Kind, State: h.State, Summary: h.Summary, Facts: h.Facts})
		}
		in.Candidates = append(in.Candidates, c)
	}
	dctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	decision, info, err := e.ai.Decide(dctx, in)
	cancel()
	_ = e.db.LogLLMCall(ctx, store.LLMCall{
		UserID: &user.ID, Purpose: "decide", Model: info.Model, PromptVer: info.PromptVersion,
		InputTokens: info.InputTokens, OutputTokens: info.OutputTokens, CostUSD: info.CostUSD(),
		Latency: info.Latency, StopReason: info.StopReason, Err: err,
	})
	if err != nil {
		decisionLog = map[string]any{"error": err.Error()}
		return finish("error", "the model call failed: "+err.Error())
	}
	decisionLog = decision
	if decision.Action != "nudge" {
		return finish("none", "model: "+decision.Reason)
	}

	// 6. The backend disposes.
	t := byID[decision.ThoughtID]
	res.ThoughtID, res.Message, res.Step = t.ID, decision.Message, decision.SuggestedStep
	if opt.DryRun {
		return finish("dry_run", decision.Reason)
	}
	if e.sender == nil {
		return finish("error", "no delivery channel configured")
	}
	nudgeID, err := e.db.CreateNudge(ctx, store.Nudge{
		UserID: user.ID, ThoughtID: t.ID, DecisionRunID: runID, NudgeType: decision.NudgeType,
		Message: decision.Message, SuggestedStep: decision.SuggestedStep, Channel: "telegram",
	})
	if err != nil {
		return finish("error", "could not record the nudge: "+err.Error())
	}
	out := Outgoing{
		ChatID: user.TelegramChatID, ThoughtID: t.ID, NudgeID: nudgeID, Type: t.Interpretation.Type,
		Title: t.Interpretation.Title, Message: decision.Message, Step: decision.SuggestedStep,
	}
	for _, sc := range short {
		if sc.ThoughtID != t.ID {
			continue
		}
		for _, h := range sc.Hooks {
			// Only facts that fired (or half-fired) are shown as the reason.
			if h.State != StateFired && h.State != StateMixed {
				continue
			}
			if out.Footnote == "" {
				out.Footnote = h.Footnote
			}
			out.Links = append(out.Links, h.Links...)
		}
	}
	msgID, err := e.sender.SendNudge(ctx, out)
	if err != nil {
		_ = e.db.MarkNudgeSuppressed(ctx, nudgeID, "delivery failed: "+err.Error())
		return finish("error", "delivery failed: "+err.Error())
	}
	if err := e.db.MarkNudgeSent(ctx, nudgeID, t.ID, msgID); err != nil {
		e.log.Error("mark nudge sent", "err", err, "nudge", nudgeID)
	}
	return finish("nudged", decision.Reason)
}

func (e *Engine) checkHooks(ctx context.Context, user store.User, p store.Prefs, t store.Thought, now time.Time, budget *scoutBudget) []HookState {
	var out []HookState
	for _, h := range t.Interpretation.Hooks {
		switch h.Kind {
		case ai.HookWeather:
			out = append(out, e.weatherState(ctx, h, p, now))
		case ai.HookWeb:
			out = append(out, e.webState(ctx, user, p, t, h, now, budget))
		case ai.HookDate:
			out = append(out, dateState(h, now))
		case ai.HookFreeTime:
			out = append(out, freeTimeState(h, p, now))
		}
	}
	return out
}

// scoutBudget works out how many web lookups this run may make.
func (e *Engine) scoutBudget(ctx context.Context, now time.Time) *scoutBudget {
	if cost, _, err := e.db.MonthlyCostUSD(ctx); err != nil || cost >= scoutBudgetShare*e.cfg.MonthlyBudgetUSD {
		return &scoutBudget{why: "monthly lookup budget reached"}
	}
	if n, err := e.db.LLMCallsSince(ctx, "scout", day(now)); err != nil || n >= e.cfg.ScoutsPerDay {
		return &scoutBudget{why: "daily lookup limit reached"}
	}
	return &scoutBudget{left: scoutsPerRun}
}

// Backfill re-reads thoughts interpreted by an older prompt so they gain
// hooks and a timeframe. Type, title and steps are kept as they were. It
// runs at startup, finds nothing once every thought is current, and is
// silent to the user.
func (e *Engine) Backfill(ctx context.Context) {
	stale, err := e.db.StaleInterpretations(ctx, ai.PromptVersionsWithoutHooks)
	if err != nil || len(stale) == 0 {
		return
	}
	if len(stale) > 25 {
		stale = stale[:25]
	}
	for _, t := range stale {
		if ctx.Err() != nil {
			return
		}
		home := ""
		if p, err := e.db.GetPrefs(ctx, t.UserID); err == nil {
			home = p.HomeCity
		}
		ictx, cancel := context.WithTimeout(ctx, 60*time.Second)
		interp, info, err := e.ai.Interpret(ictx, t.RawContent, ai.InterpretContext{Today: time.Now(), Home: home})
		cancel()
		_ = e.db.LogLLMCall(ctx, store.LLMCall{
			UserID: &t.UserID, ThoughtID: &t.ID, Purpose: "reinterpret", Model: info.Model, PromptVer: info.PromptVersion,
			InputTokens: info.InputTokens, OutputTokens: info.OutputTokens, CostUSD: info.CostUSD(),
			Latency: info.Latency, StopReason: info.StopReason, Err: err,
		})
		if err != nil {
			e.log.Warn("backfill interpret", "err", err, "thought", t.ID)
			continue
		}
		// Only the new fields are taken. The existing reading may have been
		// confirmed by the user, and a second opinion must not overwrite it.
		merged := t.Interpretation.Interpretation
		merged.Hooks, merged.Timeframe = interp.Hooks, interp.Timeframe
		if _, err := e.db.AddInterpretation(ctx, t.ID, "ai", merged, info.Model, info.PromptVersion); err != nil {
			e.log.Warn("backfill store", "err", err, "thought", t.ID)
			continue
		}
		e.log.Info("backfilled hooks", "thought", t.ID, "hooks", len(interp.Hooks))
	}
}

var dayNames = []string{"", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}

// DayList renders ISO weekdays as "Sat, Sun".
func DayList(days []int) string {
	names := make([]string, 0, len(days))
	for _, d := range days {
		if d >= 1 && d <= 7 {
			names = append(names, dayNames[d])
		}
	}
	return strings.Join(names, ", ")
}

func describeRoutine(p store.Prefs) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Sleeps %s to %s.", p.QuietStart, p.QuietEnd)
	if p.HasWorkHours() {
		fmt.Fprintf(&sb, " Works %s to %s on %s.", p.WorkStart, p.WorkEnd, DayList(p.WorkDays))
	} else {
		sb.WriteString(" No fixed working hours.")
	}
	if len(p.DaysOff) > 0 {
		fmt.Fprintf(&sb, " Days off: %s.", DayList(p.DaysOff))
	}
	return sb.String()
}

// ErrNoUser is returned when an evaluation is requested before anyone has
// talked to the bot.
var ErrNoUser = errors.New("no user yet")
