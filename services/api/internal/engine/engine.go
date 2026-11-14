package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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
	Lane      string
}

// ReviewMessage is the weekly review, ready for delivery.
type ReviewMessage struct {
	ChatID   int64
	ReviewID int64
	Week     store.WeekSummary
	Items    []store.ReviewItem // thoughts that have had no attention lately
	CheckIns []store.ReviewItem // "you said let's do it; did it get done?"
}

// Sender delivers to the user and returns the channel's message id.
type Sender interface {
	SendNudge(ctx context.Context, n Outgoing) (string, error)
	SendReview(ctx context.Context, r ReviewMessage) error
}

type Config struct {
	MonthlyBudgetUSD float64 // service-wide model spend above which everything stops; each person also has their own
	ScoutsPerDay     int     // web lookups allowed per day across everyone
}

type Engine struct {
	db      *store.Store
	ai      *ai.Client
	weather *signals.Weather
	sender  Sender
	log     *slog.Logger
	cfg     Config
	mu      sync.Mutex            // guards locks
	locks   map[int64]*sync.Mutex // one evaluation at a time per person, never across people

	lastTick atomic.Int64            // unix time the ticker last completed, for the health check
	alert    func(kind, text string) // tells the owner something broke; nil = log only
}

// SetAlert wires the owner alert. The engine cannot message anyone itself.
func (e *Engine) SetAlert(fn func(kind, text string)) { e.alert = fn }

// LastTick is when the ticker last completed a pass; zero before the first.
func (e *Engine) LastTick() time.Time {
	if v := e.lastTick.Load(); v != 0 {
		return time.Unix(v, 0)
	}
	return time.Time{}
}

func New(db *store.Store, aiClient *ai.Client, weather *signals.Weather, cfg Config, log *slog.Logger) *Engine {
	return &Engine{db: db, ai: aiClient, weather: weather, cfg: cfg, log: log, locks: map[int64]*sync.Mutex{}}
}

func (e *Engine) SetSender(s Sender) { e.sender = s }

// lockUser serialises work for one person. A slow lookup for one user must
// not hold up anyone else, so there is no engine-wide lock.
func (e *Engine) lockUser(userID int64) func() {
	e.mu.Lock()
	l, ok := e.locks[userID]
	if !ok {
		l = &sync.Mutex{}
		e.locks[userID] = l
	}
	e.mu.Unlock()
	l.Lock()
	return l.Unlock
}

type Options struct {
	Trigger string // ticker | manual | message
	Force   bool   // skip every rule: the user asked for a nudge with /nudge
	DryRun  bool   // decide, log, but do not send
	// Present means the user has just been messaging the app. Sleep, work
	// and window rules do not apply; the caps and the budget still do.
	Present bool
}

type Result struct {
	RunID     int64    `json:"run_id"`
	Outcome   string   `json:"outcome"` // nudged | none | blocked | dry_run | error
	Reason    string   `json:"reason"`
	ThoughtID int64    `json:"thought_id,omitempty"`
	Lane      string   `json:"lane,omitempty"`
	Window    string   `json:"window,omitempty"`
	Message   string   `json:"message,omitempty"`
	Step      string   `json:"suggested_step,omitempty"`
	Permit    *Permit  `json:"permit,omitempty"`
	Scores    []Scored `json:"scores,omitempty"`
}

const (
	tickEvery     = 30 * time.Minute
	nudgeTTL      = 24 * time.Hour // unanswered after this counts as ignored
	modelBackoff  = 3 * time.Hour  // after the model said "none", do not ask again for a while
	maxCandidates = 3
	scoutsPerRun  = 1
	// scoutsPerUserPerDay keeps one person from taking every lookup of the
	// day; Config.ScoutsPerDay is the total across everyone.
	scoutsPerUserPerDay = 1
	scoutBudgetShare    = 0.8        // lookups stop at 80% of the monthly budget; nudges keep the rest
	lockKey             = 0x636f6d70 // "comp"
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
		// The weekly review, when due, is this tick's message: nothing else
		// is stacked on top of it.
		if sent, why, err := e.Review(tctx, u, false); err != nil {
			e.log.Error("review", "err", err, "user", u.ID)
		} else if sent {
			e.log.Info("review sent", "user", u.ID)
			cancel()
			continue
		} else if why != "" {
			e.log.Debug("review not sent", "user", u.ID, "why", why)
		}
		res, err := e.Evaluate(tctx, u, Options{Trigger: "ticker"})
		cancel()
		if err != nil {
			e.log.Error("evaluate", "err", err, "user", u.ID)
			continue
		}
		e.log.Info("evaluated", "user", u.ID, "outcome", res.Outcome, "reason", res.Reason, "thought", res.ThoughtID)
	}
	if err := e.db.PruneRuns(ctx); err != nil {
		e.log.Warn("prune decision log", "err", err)
	}
	e.lastTick.Store(time.Now().Unix())
}

// Evaluate runs one decision for one user. Every run is logged, including
// the ones that send nothing, so "why didn't it nudge me?" always has an answer.
func (e *Engine) Evaluate(ctx context.Context, user store.User, opt Options) (Result, error) {
	defer e.lockUser(user.ID)()

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
		if outcome == "error" && e.alert != nil {
			e.alert("engine", "The nudge engine hit an error: "+reason)
		}
		// Recorded on a context that outlives a shutdown or timeout, so the
		// log never shows a run as "pending" when it in fact finished.
		fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		defer cancel()
		return res, e.db.FinishRun(fctx, runID, len(res.Scores), res.Scores, decisionLog, outcome, reason)
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

	// 1. Hard gates: may the app message this person at all right now?
	ws := Windows(now, prefs)
	pm := Permit{Obligation: true, Budgeted: true, Quick: true, Window: currentWindow(ws, now)}
	if !opt.Force {
		if reason := hardGate(now, prefs, opt.Present); reason != "" {
			return finish("blocked", reason)
		}
		// 2. What may go out: obligations, budgeted nudges, both or neither.
		load, err := e.db.NudgeLoad(ctx, user.ID, day(now), now)
		if err != nil {
			return finish("error", "could not read nudge history: "+err.Error())
		}
		tallies, err := e.db.WindowTallies(ctx, user.ID, day(now))
		if err != nil {
			e.log.Warn("window tallies", "err", err)
		}
		pm = permit(now, prefs, load, currentWindow(ws, now), chooseWindow(ws, tallies, daySeed(user.ID, now)), opt.Present)
		res.Permit = &pm
		if !pm.Obligation && !pm.Budgeted {
			return finish("blocked", pm.Reason)
		}
	}

	// 3. Candidates.
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
	touches, err := e.db.DeadlineTouches(ctx, user.ID)
	if err != nil {
		return finish("error", "could not load deadline history: "+err.Error())
	}

	// 4. Signals, scores and lanes.
	lookups := e.scoutBudget(ctx, user, prefs, now)
	if !pm.Budgeted {
		lookups = &scoutBudget{why: "no budgeted nudge can go out right now"} // a paid lookup could not lead to a send
	}
	none := &scoutBudget{why: "it could not be sent right now anyway"}
	byID := map[int64]store.Thought{}
	var eligible []Scored
	for _, t := range thoughts {
		byID[t.ID] = t
		b := lookups
		// No paid lookup for a thought that could not be sent whatever it
		// found: resting after a nudge, or moved to the weekly review.
		if !opt.Force && (onCooldown(t, now) != "" || t.IgnoredStreak >= reviewOnlyAfter) {
			b = none
		}
		hooks := e.checkHooks(ctx, user, prefs, t, now, b)
		sc := score(t, stats[t.ID], hooks, prefs, now)
		sc.Lane = laneOf(t, sc, now)
		if !opt.Force {
			sc.Held = holdReason(t, sc, touches[t.ID], pm, ws, now)
		}
		res.Scores = append(res.Scores, sc)
		if sc.Held == "" {
			eligible = append(eligible, sc)
		}
	}
	sort.SliceStable(res.Scores, func(i, j int) bool { return res.Scores[i].Score > res.Scores[j].Score })

	short, obligations := shortlist(eligible, opt.Force)
	if len(short) == 0 {
		if !pm.Budgeted {
			res.Scores = nil // an ordinary blocked tick; keep the log light
			return finish("blocked", pm.Reason)
		}
		return finish("none", "nothing can be acted on right now; the rest waits for its trigger or the weekly review")
	}

	// 5. Spend guards before asking the model. Obligations skip them: a
	// deadline is not something to save two cents on.
	if !opt.Force && !obligations {
		if cost, _, err := e.db.UserMonthlyCostUSD(ctx, user.ID); err == nil && cost >= prefs.MonthlyBudgetUSD {
			return finish("none", fmt.Sprintf("this month's AI budget is used ($%.2f of $%.2f)", cost, prefs.MonthlyBudgetUSD))
		}
		if last, err := e.db.LastModelPass(ctx, user.ID); err == nil && last != nil && now.Sub(*last) < modelBackoff {
			return finish("none", "the model passed recently; asking again after "+last.Add(modelBackoff).In(loc).Format("15:04"))
		}
	}

	// The breaker for the whole service applies to everyone and everything,
	// obligations included: it is what stands between a fault and the bill.
	if cost, _, err := e.db.MonthlyCostUSD(ctx); err == nil && cost >= e.cfg.MonthlyBudgetUSD && !opt.Force {
		if e.alert != nil {
			e.alert("budget", fmt.Sprintf("The service-wide AI budget is used up ($%.2f of $%.2f). Nudges are off until next month or until it is raised.", cost, e.cfg.MonthlyBudgetUSD))
		}
		return finish("none", "the service-wide AI budget is used up")
	}

	// 6. The model proposes.
	recent, err := e.db.RecentNudgeMessages(ctx, user.ID, 3)
	if err != nil {
		recent = []string{}
	}
	in := ai.DecideInput{
		Now:          now.Format("Monday 2 January 2006, 15:04"),
		Routine:      describeRoutine(prefs),
		Home:         prefs.HomeCity,
		Forced:       opt.Force || obligations,
		Piggyback:    opt.Present,
		RecentNudges: recent,
	}
	for _, sc := range short {
		t := byID[sc.ThoughtID]
		st := stats[t.ID]
		c := ai.DecideCandidate{
			ThoughtID: t.ID, Lane: sc.Lane, Urgency: t.Interpretation.Urgency,
			OriginalWords: t.RawContent, Type: t.Interpretation.Type, Title: t.Interpretation.Title,
			Description: t.Interpretation.Description, DesiredOutcome: t.Interpretation.DesiredOutcome,
			Timeframe: t.Interpretation.Timeframe, NextSteps: t.Interpretation.NextSteps,
			AgeDays: int(now.Sub(t.CreatedAt).Hours() / 24), NudgesSent: t.NudgeCount, IgnoredInARow: t.IgnoredStreak,
			LastResponse: st.LastResponse, Score: sc.Score, Signals: []ai.DecideSignal{},
		}
		if c.NextSteps == nil {
			c.NextSteps = []ai.NextStep{}
		}
		for _, h := range sc.Hooks {
			sig := ai.DecideSignal{Kind: h.Kind, State: h.State, Summary: h.Summary}
			// Details are handed over only for signals that hold. A lookup
			// that found nothing timely still has specifics in it (a price
			// on some listing), and a writer given them will use them.
			if h.State == StateFired || h.State == StateMixed {
				sig.Facts = h.Facts
			}
			c.Signals = append(c.Signals, sig)
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

	// 7. The backend disposes.
	t := byID[decision.ThoughtID]
	var chosen Scored
	for _, sc := range short {
		if sc.ThoughtID == t.ID {
			chosen = sc
		}
	}
	window := pm.Window
	if opt.Present {
		window = WindowChat
	}
	res.ThoughtID, res.Message, res.Step = t.ID, decision.Message, decision.SuggestedStep
	res.Lane, res.Window = chosen.Lane, window
	if opt.DryRun {
		return finish("dry_run", decision.Reason)
	}
	if e.sender == nil {
		return finish("error", "no delivery channel configured")
	}
	lane := chosen.Lane
	if lane == LaneReview || lane == "" {
		lane = LaneQuick // only reachable with /nudge, which may pick anything
	}
	// The thoughts were read before the lookups and the model call, which
	// can take minutes. Read this one again: the user may have finished,
	// dropped or snoozed it in the meantime.
	if !opt.Force {
		if fresh, err := e.db.GetThought(ctx, user.ID, t.ID); err != nil || fresh.Status != "active" {
			return finish("none", fmt.Sprintf("thought #%d changed while I was deciding, so nothing was sent", t.Number))
		}
	}
	nudgeID, err := e.db.CreateNudge(ctx, store.Nudge{
		UserID: user.ID, ThoughtID: t.ID, DecisionRunID: runID, NudgeType: decision.NudgeType,
		Message: decision.Message, SuggestedStep: decision.SuggestedStep, Channel: "telegram",
		Lane: lane, Window: window,
	})
	if err != nil {
		return finish("error", "could not record the nudge: "+err.Error())
	}
	out := Outgoing{
		ChatID: user.TelegramChatID, ThoughtID: t.ID, NudgeID: nudgeID, Type: t.Interpretation.Type,
		Title: t.Interpretation.Title, Message: decision.Message, Step: decision.SuggestedStep, Lane: lane,
	}
	for _, h := range chosen.Hooks {
		// Only facts that fired (or half-fired) are shown as the reason.
		if h.State != StateFired && h.State != StateMixed {
			continue
		}
		if out.Footnote == "" {
			out.Footnote = h.Footnote
		}
		out.Links = append(out.Links, h.Links...)
	}
	msgID, err := e.sender.SendNudge(ctx, out)
	if err != nil {
		_ = e.db.MarkNudgeSuppressed(ctx, nudgeID, "delivery failed: "+err.Error())
		return finish("error", "delivery failed: "+err.Error())
	}
	// The message is out. Recording that must not be lost to a shutdown or
	// a timeout, or the nudge would not count against any limit and another
	// could follow it. (If every attempt fails, housekeeping adopts the
	// "scheduled" row as sent ten minutes later.)
	bctx, cancelB := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
	defer cancelB()
	for attempt := 1; attempt <= 3; attempt++ {
		if err = e.db.MarkNudgeSent(bctx, nudgeID, t.ID, msgID); err == nil {
			break
		}
		e.log.Error("mark nudge sent", "err", err, "nudge", nudgeID, "attempt", attempt)
		time.Sleep(time.Duration(attempt) * time.Second)
	}
	if t.RemindAt != nil {
		_ = e.db.ClearReminder(bctx, t.ID) // the requested reminder has been delivered
	}
	return finish("nudged", decision.Reason)
}

// shortlist picks what the model is shown, at most maxCandidates. When any
// obligation is ready, only obligations are shown: a deadline never competes
// with an errand. Otherwise opportunities come before quick reminders, and
// within a lane the higher score wins.
func shortlist(eligible []Scored, force bool) (short []Scored, obligations bool) {
	rank := func(lane string) int {
		switch lane {
		case LaneRequested:
			return 0
		case LaneDeadline:
			return 1
		case LaneOpportunity:
			return 2
		default:
			return 3
		}
	}
	sorted := append([]Scored(nil), eligible...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if !force && rank(sorted[i].Lane) != rank(sorted[j].Lane) {
			return rank(sorted[i].Lane) < rank(sorted[j].Lane)
		}
		return sorted[i].Score > sorted[j].Score
	})
	if len(sorted) > 0 && !force && isObligation(sorted[0].Lane) {
		obligations = true
	}
	for _, sc := range sorted {
		if len(short) == maxCandidates {
			break
		}
		if !force {
			if obligations != isObligation(sc.Lane) || sc.Lane == LaneReview {
				continue
			}
		}
		short = append(short, sc)
	}
	return short, obligations
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

// scoutBudget works out how many web lookups this run may make. Lookups are
// the one expensive signal, so they are rationed three ways: by this
// person's monthly budget, by their own daily count, and by a daily total
// across everyone.
func (e *Engine) scoutBudget(ctx context.Context, user store.User, p store.Prefs, now time.Time) *scoutBudget {
	if cost, _, err := e.db.UserMonthlyCostUSD(ctx, user.ID); err != nil || cost >= scoutBudgetShare*p.MonthlyBudgetUSD {
		return &scoutBudget{why: "this month's lookup budget is used"}
	}
	if cost, _, err := e.db.MonthlyCostUSD(ctx); err != nil || cost >= scoutBudgetShare*e.cfg.MonthlyBudgetUSD {
		return &scoutBudget{why: "the service-wide lookup budget is used"}
	}
	if n, err := e.db.UserCallsSince(ctx, user.ID, "scout", day(now)); err != nil || n >= scoutsPerUserPerDay {
		return &scoutBudget{why: "today's lookup is used"}
	}
	if n, err := e.db.LLMCallsSince(ctx, "scout", now.Add(-24*time.Hour)); err != nil || n >= e.cfg.ScoutsPerDay {
		return &scoutBudget{why: "the service-wide daily lookup limit is reached"}
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
