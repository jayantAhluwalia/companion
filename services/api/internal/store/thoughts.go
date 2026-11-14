package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"companion/api/internal/ai"
)

type Thought struct {
	ID                int64
	Number            int // the person's own numbering, shown in chat; ID stays internal
	UserID            int64
	RawContent        string
	Status            string
	SnoozedUntil      *time.Time
	CreatedAt         time.Time
	LastTouchedAt     time.Time
	LastNudgedAt      *time.Time
	NudgeCount        int
	IgnoredStreak     int
	RemindAt          *time.Time // set when the user said when to bring it back
	InterpretAttempts int
	Interpretation    *Interpretation // current version, nil until interpreted
}

type Interpretation struct {
	ID            int64
	ThoughtID     int64
	Version       int
	Source        string
	Model         string
	PromptVersion string
	CreatedAt     time.Time
	ai.Interpretation
}

var ErrNotFound = errors.New("not found")

// CreateThought saves a capture. messageID is the channel's id for the
// message it came from (0 if none); a second capture of the same message
// returns created = false and saves nothing, so a redelivered update after a
// restart cannot produce a duplicate thought.
func (s *Store) CreateThought(ctx context.Context, userID int64, raw, source string, messageID int64) (t Thought, created bool, err error) {
	var mid *int64
	if messageID != 0 {
		mid = &messageID
	}
	// The number is the person's next one. Two captures arriving at the same
	// instant could pick the same number; the unique index rejects the
	// second and it is tried again.
	for attempt := 0; ; attempt++ {
		err = s.pool.QueryRow(ctx, `
			insert into thoughts (user_id, raw_content, source, source_message_id, number)
			values ($1, $2, $3, $4, (select coalesce(max(number), 0) + 1 from thoughts where user_id = $1))
			on conflict (user_id, source_message_id) where source_message_id is not null do nothing
			returning id, number, user_id, raw_content, status, created_at, last_touched_at`,
			userID, raw, source, mid,
		).Scan(&t.ID, &t.Number, &t.UserID, &t.RawContent, &t.Status, &t.CreatedAt, &t.LastTouchedAt)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "thoughts_user_number_idx" && attempt < 5 {
			continue
		}
		break
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return t, false, nil
	}
	if err != nil {
		return t, false, err
	}
	return t, true, s.AddEvent(ctx, t.ID, nil, "created", nil)
}

// MaxInterpretAttempts is how many times a thought is sent to the model
// before the app gives up and keeps the person's original words.
const MaxInterpretAttempts = 3

// CountInterpretAttempt records that an interpretation is about to be tried.
func (s *Store) CountInterpretAttempt(ctx context.Context, thoughtID int64) error {
	_, err := s.pool.Exec(ctx, `update thoughts set interpret_attempts = interpret_attempts + 1 where id = $1`, thoughtID)
	return err
}

// Uninterpreted returns captures that still have no interpretation after
// the grace period: the first attempt failed, timed out or was cut off by a
// restart. Without this they would sit unread forever.
func (s *Store) Uninterpreted(ctx context.Context, olderThan time.Duration) ([]Thought, error) {
	rows, err := s.pool.Query(ctx, thoughtSelect+`
		where t.current_interpretation_id is null and t.status = 'active'
		  and t.interpret_attempts < $1 and t.created_at < now() - make_interval(secs => $2)
		order by t.id limit 20`, MaxInterpretAttempts, olderThan.Seconds())
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

// GetKV and SetKV hold small durable state for the process itself.
func (s *Store) GetKV(ctx context.Context, key string) (string, error) {
	var v string
	err := s.pool.QueryRow(ctx, `select value from kv where key = $1`, key).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (s *Store) SetKV(ctx context.Context, key, value string) error {
	_, err := s.pool.Exec(ctx, `
		insert into kv (key, value) values ($1, $2)
		on conflict (key) do update set value = excluded.value, updated_at = now()`, key, value)
	return err
}

// AddInterpretation stores a new version and makes it current. source is
// "ai" or "user"; a user correction is just another version.
func (s *Store) AddInterpretation(ctx context.Context, thoughtID int64, source string, in ai.Interpretation, model, promptVersion string) (Interpretation, error) {
	steps, _ := json.Marshal(in.NextSteps)
	if in.NextSteps == nil {
		steps = []byte("[]")
	}
	assumptions, _ := json.Marshal(in.Assumptions)
	if in.Assumptions == nil {
		assumptions = []byte("[]")
	}
	hooks, _ := json.Marshal(in.Hooks)
	if in.Hooks == nil {
		hooks = []byte("[]")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Interpretation{}, err
	}
	defer tx.Rollback(ctx)

	out := Interpretation{ThoughtID: thoughtID, Source: source, Model: model, PromptVersion: promptVersion, Interpretation: in}
	err = tx.QueryRow(ctx, `
		insert into thought_interpretations
		  (thought_id, version, source, type, title, description, desired_outcome, next_steps,
		   confidence, clarification_needed, clarification_question, assumptions, model, prompt_version,
		   timeframe, hooks, urgency)
		values ($1, (select coalesce(max(version), 0) + 1 from thought_interpretations where thought_id = $1),
		        $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, nullif($12, ''), nullif($13, ''), $14, $15, $16)
		returning id, version, created_at`,
		thoughtID, source, string(in.Type), in.Title, in.Description, in.DesiredOutcome, steps,
		in.Confidence, in.ClarificationNeeded, in.ClarificationQuestion, assumptions, model, promptVersion,
		in.Timeframe, hooks, in.Urgency,
	).Scan(&out.ID, &out.Version, &out.CreatedAt)
	if err != nil {
		return out, err
	}
	if _, err := tx.Exec(ctx, `update thoughts set current_interpretation_id = $2, updated_at = now() where id = $1`, thoughtID, out.ID); err != nil {
		return out, err
	}
	payload, _ := json.Marshal(map[string]any{"version": out.Version, "source": source, "type": in.Type, "confidence": in.Confidence})
	if _, err := tx.Exec(ctx, `insert into thought_events (thought_id, type, payload) values ($1, 'interpreted', $2)`, thoughtID, payload); err != nil {
		return out, err
	}
	return out, tx.Commit(ctx)
}

func (s *Store) AddEvent(ctx context.Context, thoughtID int64, nudgeID *int64, typ string, payload map[string]any) error {
	raw := []byte("{}")
	if payload != nil {
		raw, _ = json.Marshal(payload)
	}
	_, err := s.pool.Exec(ctx, `insert into thought_events (thought_id, nudge_id, type, payload) values ($1, $2, $3, $4)`, thoughtID, nudgeID, typ, raw)
	return err
}

// Touch records user attention, which feeds the recency score.
func (s *Store) Touch(ctx context.Context, thoughtID int64) error {
	_, err := s.pool.Exec(ctx, `update thoughts set last_touched_at = now(), updated_at = now() where id = $1`, thoughtID)
	return err
}

func (s *Store) SetStatus(ctx context.Context, thoughtID int64, status string, snoozedUntil *time.Time) error {
	tag, err := s.pool.Exec(ctx, `
		update thoughts set status = $2, snoozed_until = $3, last_touched_at = now(), updated_at = now()
		where id = $1`, thoughtID, status, snoozedUntil)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	payload := map[string]any{"status": status}
	if snoozedUntil != nil {
		payload["until"] = snoozedUntil.Format(time.RFC3339)
	}
	return s.AddEvent(ctx, thoughtID, nil, "status_"+status, payload)
}

// SnoozeWithReminder parks a thought until the given time and asks the
// engine to bring it back then: the user chose the moment themselves.
func (s *Store) SnoozeWithReminder(ctx context.Context, thoughtID int64, until time.Time) error {
	if err := s.SetStatus(ctx, thoughtID, "snoozed", &until); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `update thoughts set remind_at = $2 where id = $1`, thoughtID, until)
	return err
}

func (s *Store) ClearReminder(ctx context.Context, thoughtID int64) error {
	_, err := s.pool.Exec(ctx, `update thoughts set remind_at = null where id = $1`, thoughtID)
	return err
}

const thoughtSelect = `
	select t.id, t.number, t.user_id, t.raw_content, t.status, t.snoozed_until, t.created_at, t.last_touched_at,
	       t.last_nudged_at, t.nudge_count, t.ignored_streak, t.remind_at, t.interpret_attempts,
	       i.id, i.version, i.source, coalesce(i.model, ''), coalesce(i.prompt_version, ''), i.created_at,
	       i.type, i.title, i.description, i.desired_outcome, i.next_steps, i.confidence,
	       i.clarification_needed, i.clarification_question, i.assumptions, i.timeframe, i.hooks, i.urgency
	  from thoughts t
	  left join thought_interpretations i on i.id = t.current_interpretation_id`

func scanThought(row pgx.Row) (Thought, error) {
	var t Thought
	var iID *int64
	var iVersion *int
	var iSource, iModel, iPrompt, iType, iTitle, iDesc, iOutcome, iQuestion, iTimeframe, iUrgency *string
	var iCreated *time.Time
	var iSteps, iAssumptions, iHooks []byte
	var iConf *float64
	var iClar *bool
	err := row.Scan(&t.ID, &t.Number, &t.UserID, &t.RawContent, &t.Status, &t.SnoozedUntil, &t.CreatedAt, &t.LastTouchedAt,
		&t.LastNudgedAt, &t.NudgeCount, &t.IgnoredStreak, &t.RemindAt, &t.InterpretAttempts,
		&iID, &iVersion, &iSource, &iModel, &iPrompt, &iCreated,
		&iType, &iTitle, &iDesc, &iOutcome, &iSteps, &iConf, &iClar, &iQuestion, &iAssumptions, &iTimeframe, &iHooks, &iUrgency)
	if err != nil {
		return t, err
	}
	if iID != nil {
		in := Interpretation{ID: *iID, ThoughtID: t.ID, Version: *iVersion, Source: *iSource, Model: *iModel, PromptVersion: *iPrompt, CreatedAt: *iCreated}
		in.Type = ai.ThoughtType(*iType)
		in.Title, in.Description, in.DesiredOutcome = *iTitle, *iDesc, *iOutcome
		in.Confidence, in.ClarificationNeeded, in.ClarificationQuestion = *iConf, *iClar, *iQuestion
		_ = json.Unmarshal(iSteps, &in.NextSteps)
		_ = json.Unmarshal(iAssumptions, &in.Assumptions)
		_ = json.Unmarshal(iHooks, &in.Hooks)
		in.Timeframe, in.Urgency = *iTimeframe, *iUrgency
		t.Interpretation = &in
	}
	return t, nil
}

func (s *Store) GetThought(ctx context.Context, userID, thoughtID int64) (Thought, error) {
	t, err := scanThought(s.pool.QueryRow(ctx, thoughtSelect+` where t.id = $1 and t.user_id = $2`, thoughtID, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

// GetThoughtByNumber finds a thought by the number its owner sees.
func (s *Store) GetThoughtByNumber(ctx context.Context, userID int64, number int) (Thought, error) {
	t, err := scanThought(s.pool.QueryRow(ctx, thoughtSelect+` where t.number = $1 and t.user_id = $2`, number, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return t, ErrNotFound
	}
	return t, err
}

// ListThoughts returns the user's thoughts in the given statuses, newest first.
func (s *Store) ListThoughts(ctx context.Context, userID int64, statuses []string, limit int) ([]Thought, error) {
	rows, err := s.pool.Query(ctx, thoughtSelect+`
		where t.user_id = $1 and t.status = any($2)
		order by t.created_at desc limit $3`, userID, statuses, limit)
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

type LLMCall struct {
	UserID       *int64
	ThoughtID    *int64
	Purpose      string
	Model        string
	PromptVer    string
	InputTokens  int64
	OutputTokens int64
	WebSearches  int64
	CostUSD      float64
	Latency      time.Duration
	StopReason   string
	Err          error
}

func (s *Store) LogLLMCall(ctx context.Context, c LLMCall) error {
	var errText *string
	if c.Err != nil {
		e := c.Err.Error()
		errText = &e
	}
	_, err := s.pool.Exec(ctx, `
		insert into llm_calls (user_id, thought_id, purpose, model, prompt_version, input_tokens, output_tokens,
		                       cost_usd, latency_ms, stop_reason, error, web_searches)
		values ($1, $2, $3, $4, $5, $6, $7, $8, $9, nullif($10, ''), $11, $12)`,
		c.UserID, c.ThoughtID, c.Purpose, c.Model, c.PromptVer, c.InputTokens, c.OutputTokens,
		c.CostUSD, c.Latency.Milliseconds(), c.StopReason, errText, c.WebSearches)
	if err != nil {
		return fmt.Errorf("log llm call: %w", err)
	}
	return nil
}

// Stats for the /why and cost views; cheap aggregate for now.
func (s *Store) MonthlyCostUSD(ctx context.Context) (float64, int, error) {
	var cost float64
	var n int
	err := s.pool.QueryRow(ctx, `
		select coalesce(sum(cost_usd), 0), count(*) from llm_calls
		where created_at >= date_trunc('month', now())`).Scan(&cost, &n)
	return cost, n, err
}
