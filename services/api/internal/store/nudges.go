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

// NudgeStats feeds the daily cap and the minimum interval. A nudge the user
// asked for with /nudge does not use up the day's automatic one, but it
// does count for the interval, so two messages never arrive back to back.
func (s *Store) NudgeStats(ctx context.Context, userID int64, dayStart time.Time) (sentToday int, lastSent *time.Time, err error) {
	err = s.pool.QueryRow(ctx, `
		select count(*) filter (where n.sent_at >= $2 and coalesce(r.trigger, 'ticker') <> 'manual'), max(n.sent_at)
		  from nudges n left join decision_runs r on r.id = n.decision_run_id
		 where n.user_id = $1 and n.sent_at is not null`, userID, dayStart).Scan(&sentToday, &lastSent)
	return
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
}

func (s *Store) CreateNudge(ctx context.Context, n Nudge) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		insert into nudges (user_id, thought_id, decision_run_id, nudge_type, style, channel, message, suggested_action)
		values ($1, $2, $3, $4, $4, $5, $6, $7) returning id`,
		n.UserID, n.ThoughtID, n.DecisionRunID, n.NudgeType, n.Channel, n.Message, n.SuggestedStep).Scan(&id)
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

// StaleInterpretations lists active thoughts whose AI interpretation predates
// the given prompt version, so they can be re-read once to gain hooks.
func (s *Store) StaleInterpretations(ctx context.Context, promptVersion string) ([]Thought, error) {
	rows, err := s.pool.Query(ctx, thoughtSelect+`
		where t.status in ('active', 'snoozed') and i.source = 'ai' and coalesce(i.prompt_version, '') <> $1
		order by t.id`, promptVersion)
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
