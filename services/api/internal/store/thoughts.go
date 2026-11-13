package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"companion/api/internal/ai"
)

type Thought struct {
	ID             int64
	UserID         int64
	RawContent     string
	Status         string
	SnoozedUntil   *time.Time
	CreatedAt      time.Time
	LastTouchedAt  time.Time
	LastNudgedAt   *time.Time
	NudgeCount     int
	IgnoredStreak  int
	Interpretation *Interpretation // current version, nil until interpreted
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

func (s *Store) CreateThought(ctx context.Context, userID int64, raw, source string) (Thought, error) {
	var t Thought
	err := s.pool.QueryRow(ctx, `
		insert into thoughts (user_id, raw_content, source) values ($1, $2, $3)
		returning id, user_id, raw_content, status, created_at, last_touched_at`,
		userID, raw, source,
	).Scan(&t.ID, &t.UserID, &t.RawContent, &t.Status, &t.CreatedAt, &t.LastTouchedAt)
	if err != nil {
		return t, err
	}
	return t, s.AddEvent(ctx, t.ID, nil, "created", nil)
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
		   timeframe, hooks)
		values ($1, (select coalesce(max(version), 0) + 1 from thought_interpretations where thought_id = $1),
		        $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, nullif($12, ''), nullif($13, ''), $14, $15)
		returning id, version, created_at`,
		thoughtID, source, string(in.Type), in.Title, in.Description, in.DesiredOutcome, steps,
		in.Confidence, in.ClarificationNeeded, in.ClarificationQuestion, assumptions, model, promptVersion,
		in.Timeframe, hooks,
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

const thoughtSelect = `
	select t.id, t.user_id, t.raw_content, t.status, t.snoozed_until, t.created_at, t.last_touched_at,
	       t.last_nudged_at, t.nudge_count, t.ignored_streak,
	       i.id, i.version, i.source, coalesce(i.model, ''), coalesce(i.prompt_version, ''), i.created_at,
	       i.type, i.title, i.description, i.desired_outcome, i.next_steps, i.confidence,
	       i.clarification_needed, i.clarification_question, i.assumptions, i.timeframe, i.hooks
	  from thoughts t
	  left join thought_interpretations i on i.id = t.current_interpretation_id`

func scanThought(row pgx.Row) (Thought, error) {
	var t Thought
	var iID *int64
	var iVersion *int
	var iSource, iModel, iPrompt, iType, iTitle, iDesc, iOutcome, iQuestion, iTimeframe *string
	var iCreated *time.Time
	var iSteps, iAssumptions, iHooks []byte
	var iConf *float64
	var iClar *bool
	err := row.Scan(&t.ID, &t.UserID, &t.RawContent, &t.Status, &t.SnoozedUntil, &t.CreatedAt, &t.LastTouchedAt,
		&t.LastNudgedAt, &t.NudgeCount, &t.IgnoredStreak,
		&iID, &iVersion, &iSource, &iModel, &iPrompt, &iCreated,
		&iType, &iTitle, &iDesc, &iOutcome, &iSteps, &iConf, &iClar, &iQuestion, &iAssumptions, &iTimeframe, &iHooks)
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
		in.Timeframe = *iTimeframe
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
