package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
)

// Stats are the per-thought facts the importance score needs.
type Stats struct {
	Confirmed    bool   // the user tapped "Looks right"
	UserEdits    int    // corrections and clarifications
	Laters       int    // times a nudge was answered with "later"
	LastResponse string // do_it | next_steps | later | not_interested | ignored | ""
}

// Candidates are the thoughts the engine may nudge about: active,
// interpreted and understood.
func (s *Store) Candidates(ctx context.Context, userID int64) ([]Thought, error) {
	rows, err := s.pool.Query(ctx, thoughtSelect+`
		where t.user_id = $1 and t.status = 'active' and t.current_interpretation_id is not null
		  and i.type <> 'unclear'
		order by t.created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Thought
	for rows.Next() {
		t, err := scanThought(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Store) ThoughtStats(ctx context.Context, thoughtIDs []int64) (map[int64]Stats, error) {
	out := map[int64]Stats{}
	rows, err := s.pool.Query(ctx, `
		select t.id,
		       exists (select 1 from thought_events e where e.thought_id = t.id and e.type = 'interpretation_confirmed'),
		       (select count(*) from thought_interpretations i where i.thought_id = t.id and i.source = 'user')
		         + (select count(*) from thought_events e where e.thought_id = t.id and e.type = 'clarified'),
		       (select count(*) from nudges n where n.thought_id = t.id and n.response_action = 'later'),
		       coalesce((select case when n.status = 'expired' then 'ignored' else n.response_action end
		                   from nudges n where n.thought_id = t.id and n.status in ('responded', 'expired')
		                  order by n.sent_at desc limit 1), '')
		  from thoughts t where t.id = any($1)`, thoughtIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var st Stats
		if err := rows.Scan(&id, &st.Confirmed, &st.UserEdits, &st.Laters, &st.LastResponse); err != nil {
			return nil, err
		}
		out[id] = st
	}
	return out, rows.Err()
}

// WakeSnoozed returns snoozed thoughts to active once their time is up.
func (s *Store) WakeSnoozed(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `
		update thoughts set status = 'active', snoozed_until = null, updated_at = now()
		where user_id = $1 and status = 'snoozed' and snoozed_until <= now()`, userID)
	return err
}

// FadeAfter is how many nudges in a row may go unanswered before a thought
// is quietly set aside.
const FadeAfter = 4

// ExpireNudges closes nudges that got no response within the window. Each
// one counts as an ignore, which lengthens that thought's cooldown; after
// FadeAfter in a row the thought fades.
func (s *Store) ExpireNudges(ctx context.Context, userID int64, window time.Duration) (int, error) {
	rows, err := s.pool.Query(ctx, `
		update nudges set status = 'expired'
		where user_id = $1 and status = 'sent' and sent_at < now() - make_interval(secs => $2)
		returning id, thought_id`, userID, window.Seconds())
	if err != nil {
		return 0, err
	}
	type pair struct{ nudge, thought int64 }
	var expired []pair
	for rows.Next() {
		var p pair
		if err := rows.Scan(&p.nudge, &p.thought); err != nil {
			rows.Close()
			return 0, err
		}
		expired = append(expired, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	for _, p := range expired {
		var streak int
		if err := s.pool.QueryRow(ctx, `
			update thoughts set ignored_streak = ignored_streak + 1, updated_at = now()
			where id = $1 returning ignored_streak`, p.thought).Scan(&streak); err != nil {
			return 0, err
		}
		_ = s.AddEvent(ctx, p.thought, &p.nudge, "nudge_ignored", map[string]any{"streak": streak})
		if streak >= FadeAfter {
			_, _ = s.pool.Exec(ctx, `update thoughts set status = 'faded', updated_at = now() where id = $1 and status = 'active'`, p.thought)
			_ = s.AddEvent(ctx, p.thought, &p.nudge, "status_faded", map[string]any{"after_ignores": streak})
		}
	}
	return len(expired), nil
}

// Load is how much the user has been sent lately. Deadline and requested
// reminders are counted apart from the rest: they are obligations, and the
// budget exists to ration everything else. Nudges the user asked for with
// /nudge count for nothing except the gap between messages.
type Load struct {
	BudgetedToday   int        // opportunity + quick nudges sent today
	ObligationToday int        // deadline + requested nudges sent today
	BudgetedWeek    int        // opportunity + quick nudges in the last 7 days
	QuickWeek       int        // quick nudges in the last 7 days
	ReviewsWeek     int        // weekly reviews in the last 7 days
	LastSent        *time.Time // any nudge at all
	IgnoredRun      int        // how many of the most recent budgeted nudges went unanswered, in a row
}

func (s *Store) NudgeLoad(ctx context.Context, userID int64, dayStart, now time.Time) (Load, error) {
	var l Load
	weekStart := now.Add(-7 * 24 * time.Hour)
	err := s.pool.QueryRow(ctx, `
		with n as (
			select n.sent_at, n.lane, coalesce(r.trigger, 'ticker') <> 'manual' as auto
			  from nudges n left join decision_runs r on r.id = n.decision_run_id
			 where n.user_id = $1 and n.sent_at is not null
		)
		select count(*) filter (where auto and sent_at >= $2 and lane in ('opportunity', 'quick')),
		       count(*) filter (where auto and sent_at >= $2 and lane in ('deadline', 'requested')),
		       count(*) filter (where auto and sent_at >= $3 and lane in ('opportunity', 'quick')),
		       count(*) filter (where auto and sent_at >= $3 and lane = 'quick'),
		       max(sent_at),
		       (select count(*) from reviews where user_id = $1 and sent_at >= $3)
		  from n`, userID, dayStart, weekStart,
	).Scan(&l.BudgetedToday, &l.ObligationToday, &l.BudgetedWeek, &l.QuickWeek, &l.LastSent, &l.ReviewsWeek)
	if err != nil {
		return l, err
	}
	rows, err := s.pool.Query(ctx, `
		select n.status from nudges n left join decision_runs r on r.id = n.decision_run_id
		 where n.user_id = $1 and n.lane in ('opportunity', 'quick') and n.status in ('responded', 'expired')
		   and coalesce(r.trigger, 'ticker') <> 'manual'
		 order by n.sent_at desc limit 8`, userID)
	if err != nil {
		return l, err
	}
	defer rows.Close()
	counting := true
	for rows.Next() {
		var status string
		if err := rows.Scan(&status); err != nil {
			return l, err
		}
		if counting && status == "expired" {
			l.IgnoredRun++
		} else {
			counting = false
		}
	}
	return l, rows.Err()
}

// DeadlineTouch is what a thought has already been sent about its deadline.
type DeadlineTouch struct {
	Count int
	Last  time.Time
}

// DeadlineTouches returns recent deadline reminders per thought, so each
// deadline gets at most two: a heads-up and the moment itself.
func (s *Store) DeadlineTouches(ctx context.Context, userID int64) (map[int64]DeadlineTouch, error) {
	out := map[int64]DeadlineTouch{}
	rows, err := s.pool.Query(ctx, `
		select thought_id, count(*), max(sent_at) from nudges
		 where user_id = $1 and lane = 'deadline' and sent_at >= now() - interval '4 days'
		 group by thought_id`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var t DeadlineTouch
		if err := rows.Scan(&id, &t.Count, &t.Last); err != nil {
			return nil, err
		}
		out[id] = t
	}
	return out, rows.Err()
}

// Tally is how nudges sent in one window of the day have gone.
type Tally struct {
	Acted   int // "Let's do it" or "Next steps"
	Ignored int // no response within a day
}

// WindowTallies counts outcomes per send window over the last six months,
// using only nudges sent before the given time so that a day's choice of
// window does not shift while the day is in progress. "Later" and "Not
// interested" are left out: the first is neutral, the second is about the
// thought, not the timing.
func (s *Store) WindowTallies(ctx context.Context, userID int64, before time.Time) (map[string]Tally, error) {
	out := map[string]Tally{}
	rows, err := s.pool.Query(ctx, `
		select send_window,
		       count(*) filter (where response_action in ('do_it', 'next_steps')),
		       count(*) filter (where status = 'expired')
		  from nudges
		 where user_id = $1 and send_window is not null and lane in ('opportunity', 'quick')
		   and sent_at < $2 and sent_at >= $2 - interval '180 days'
		 group by send_window`, userID, before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var w string
		var t Tally
		if err := rows.Scan(&w, &t.Acted, &t.Ignored); err != nil {
			return nil, err
		}
		out[w] = t
	}
	return out, rows.Err()
}

// RecentNudgeMessages returns the last few messages sent, newest first, so
// the writer can avoid repeating itself.
func (s *Store) RecentNudgeMessages(ctx context.Context, userID int64, limit int) ([]string, error) {
	rows, err := s.pool.Query(ctx, `select message from nudges where user_id = $1 and sent_at is not null order by sent_at desc limit $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

type Nudge struct {
	ID            int64
	UserID        int64
	ThoughtID     int64
	DecisionRunID int64
	NudgeType     string
	Message       string
	SuggestedStep string
	Channel       string
	Lane          string // deadline | requested | opportunity | quick
	Window        string // which window of the day it went out in; "" if none
}

func (s *Store) CreateNudge(ctx context.Context, n Nudge) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		insert into nudges (user_id, thought_id, decision_run_id, nudge_type, style, channel, message, suggested_action, lane, send_window)
		values ($1, $2, $3, $4, $4, $5, $6, $7, $8, nullif($9, '')) returning id`,
		n.UserID, n.ThoughtID, n.DecisionRunID, n.NudgeType, n.Channel, n.Message, n.SuggestedStep, n.Lane, n.Window).Scan(&id)
	return id, err
}

// MarkNudgeSent records delivery and updates the thought's nudge history.
func (s *Store) MarkNudgeSent(ctx context.Context, nudgeID, thoughtID int64, channelMessageID string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `update nudges set status = 'sent', sent_at = now(), channel_message_id = nullif($2, '') where id = $1`,
		nudgeID, channelMessageID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `update thoughts set last_nudged_at = now(), nudge_count = nudge_count + 1, updated_at = now() where id = $1`,
		thoughtID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `insert into thought_events (thought_id, nudge_id, type) values ($1, $2, 'nudged')`, thoughtID, nudgeID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) MarkNudgeSuppressed(ctx context.Context, nudgeID int64, reason string) error {
	_, err := s.pool.Exec(ctx, `update nudges set status = 'suppressed', suppressed_reason = $2 where id = $1`, nudgeID, reason)
	return err
}

// RespondNudge records which button was pressed. Any response, even
// "later", proves the user saw it, so the ignore streak resets.
func (s *Store) RespondNudge(ctx context.Context, nudgeID int64, action string) error {
	var thoughtID int64
	err := s.pool.QueryRow(ctx, `
		update nudges set status = 'responded', responded_at = now(), response_action = $2
		where id = $1 returning thought_id`, nudgeID, action).Scan(&thoughtID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `update thoughts set ignored_streak = 0, updated_at = now() where id = $1`, thoughtID)
	return err
}

func (s *Store) SetNudgeFeedback(ctx context.Context, nudgeID int64, feedback string) error {
	_, err := s.pool.Exec(ctx, `update nudges set feedback = $2 where id = $1`, nudgeID, feedback)
	return err
}

// --- decision log ---

func (s *Store) StartRun(ctx context.Context, userID int64, trigger string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `insert into decision_runs (user_id, trigger) values ($1, $2) returning id`, userID, trigger).Scan(&id)
	return id, err
}

func (s *Store) FinishRun(ctx context.Context, runID int64, candidateCount int, scores, llmDecision any, outcome, reason string) error {
	scoresJSON := []byte("[]")
	if scores != nil {
		scoresJSON, _ = json.Marshal(scores)
	}
	var decisionJSON []byte
	if llmDecision != nil {
		decisionJSON, _ = json.Marshal(llmDecision)
	}
	_, err := s.pool.Exec(ctx, `
		update decision_runs set finished_at = now(), candidate_count = $2, scores = $3, llm_decision = $4, outcome = $5, reason = $6
		where id = $1`, runID, candidateCount, scoresJSON, decisionJSON, outcome, reason)
	return err
}

type Run struct {
	ID             int64
	StartedAt      time.Time
	Trigger        string
	CandidateCount int
	Scores         json.RawMessage
	LLMDecision    json.RawMessage
	Outcome        string
	Reason         string
}

// LastRuns returns the newest finished runs, optionally only those that got
// past the gates (anything but "blocked").
func (s *Store) LastRuns(ctx context.Context, userID int64, limit int, pastGatesOnly bool) ([]Run, error) {
	rows, err := s.pool.Query(ctx, `
		select id, started_at, trigger, candidate_count, scores, llm_decision, outcome, reason
		  from decision_runs
		 where user_id = $1 and finished_at is not null and ($3 = false or outcome <> 'blocked')
		 order by id desc limit $2`, userID, limit, pastGatesOnly)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Run
	for rows.Next() {
		var r Run
		if err := rows.Scan(&r.ID, &r.StartedAt, &r.Trigger, &r.CandidateCount, &r.Scores, &r.LLMDecision, &r.Outcome, &r.Reason); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LastModelPass is when the engine last asked the model and got "none" or an
// error. The engine backs off after one so a quiet day costs one call, not twenty.
func (s *Store) LastModelPass(ctx context.Context, userID int64) (*time.Time, error) {
	var at *time.Time
	err := s.pool.QueryRow(ctx, `
		select max(finished_at) from decision_runs
		 where user_id = $1 and llm_decision is not null and outcome in ('none', 'error') and trigger = 'ticker'`, userID).Scan(&at)
	return at, err
}

// PruneBlockedRuns keeps the decision log readable: gate-blocked ticks are
// only interesting for a few days.
func (s *Store) PruneBlockedRuns(ctx context.Context, olderThan time.Duration) error {
	_, err := s.pool.Exec(ctx, `delete from decision_runs where outcome = 'blocked' and started_at < now() - make_interval(secs => $1)
		and not exists (select 1 from nudges n where n.decision_run_id = decision_runs.id)`, olderThan.Seconds())
	return err
}

// --- signals (cached web findings) ---

type Signal struct {
	ThoughtID int64
	HookKey   string
	Kind      string
	State     string
	Summary   string
	Facts     json.RawMessage
	FetchedAt time.Time
	ExpiresAt time.Time
}

func (s *Store) GetSignal(ctx context.Context, thoughtID int64, hookKey string) (*Signal, error) {
	sig := Signal{ThoughtID: thoughtID, HookKey: hookKey}
	err := s.pool.QueryRow(ctx, `
		select kind, state, summary, facts, fetched_at, expires_at from signals
		 where thought_id = $1 and hook_key = $2`, thoughtID, hookKey,
	).Scan(&sig.Kind, &sig.State, &sig.Summary, &sig.Facts, &sig.FetchedAt, &sig.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &sig, err
}

func (s *Store) UpsertSignal(ctx context.Context, sig Signal) error {
	_, err := s.pool.Exec(ctx, `
		insert into signals (thought_id, hook_key, kind, state, summary, facts, fetched_at, expires_at)
		values ($1, $2, $3, $4, $5, $6, now(), $7)
		on conflict (thought_id, hook_key) do update
		   set state = excluded.state, summary = excluded.summary, facts = excluded.facts,
		       fetched_at = now(), expires_at = excluded.expires_at`,
		sig.ThoughtID, sig.HookKey, sig.Kind, sig.State, sig.Summary, sig.Facts, sig.ExpiresAt)
	return err
}

// LLMCallsSince counts calls of one purpose, for per-day spending limits.
func (s *Store) LLMCallsSince(ctx context.Context, purpose string, since time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `select count(*) from llm_calls where purpose = $1 and created_at >= $2`, purpose, since).Scan(&n)
	return n, err
}

// StaleInterpretations lists active thoughts whose AI interpretation was
// written by one of the given prompt versions, so they can be re-read once.
func (s *Store) StaleInterpretations(ctx context.Context, staleVersions []string) ([]Thought, error) {
	rows, err := s.pool.Query(ctx, thoughtSelect+`
		where t.status in ('active', 'snoozed') and i.source = 'ai' and coalesce(i.prompt_version, '') = any($1)
		order by t.id`, staleVersions)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Thought
	for rows.Next() {
		t, err := scanThought(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// GetUser loads a user by id.
func (s *Store) GetUser(ctx context.Context, userID int64) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `select id, telegram_chat_id, coalesce(display_name, ''), timezone, created_at from users where id = $1`, userID).
		Scan(&u.ID, &u.TelegramChatID, &u.DisplayName, &u.Timezone, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

// Users lists every user; the ticker evaluates each in turn.
func (s *Store) Users(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `select id, telegram_chat_id, coalesce(display_name, ''), timezone, created_at from users order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.TelegramChatID, &u.DisplayName, &u.Timezone, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// TryLock takes a Postgres advisory lock so two processes (a deploy in
// flight) never evaluate at once. The lock lives on one pinned connection;
// call unlock to release both.
func (s *Store) TryLock(ctx context.Context, key int64) (unlock func(), ok bool, err error) {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return nil, false, err
	}
	if err := conn.QueryRow(ctx, `select pg_try_advisory_lock($1)`, key).Scan(&ok); err != nil || !ok {
		conn.Release()
		return nil, false, err
	}
	return func() {
		_, _ = conn.Exec(context.Background(), `select pg_advisory_unlock($1)`, key)
		conn.Release()
	}, true, nil
}
