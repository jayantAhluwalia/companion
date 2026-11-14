package store

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	RoleOwner  = "owner"
	RoleMember = "member"
)

type User struct {
	ID             int64
	TelegramChatID int64
	DisplayName    string
	Timezone       string
	Role           string
	CreatedAt      time.Time
}

func (u User) IsOwner() bool { return u.Role == RoleOwner }

const userColumns = `id, telegram_chat_id, coalesce(display_name, ''), timezone, role, created_at`

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.TelegramChatID, &u.DisplayName, &u.Timezone, &u.Role, &u.CreatedAt)
	return u, err
}

// UserByChat finds the user for a Telegram chat. A chat with no user is a
// stranger: nothing is created for them until they redeem an invite.
func (s *Store) UserByChat(ctx context.Context, chatID int64) (User, bool, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `select `+userColumns+` from users where telegram_chat_id = $1`, chatID))
	if errors.Is(err, pgx.ErrNoRows) {
		return u, false, nil
	}
	return u, err == nil, err
}

// EnsureOwner makes the configured chat the owner, creating the user if
// needed. It is idempotent and runs at every start.
func (s *Store) EnsureOwner(ctx context.Context, chatID int64, displayName string) (User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `
		insert into users (telegram_chat_id, display_name, role) values ($1, nullif($2, ''), 'owner')
		on conflict (telegram_chat_id) do update set role = 'owner'
		returning `+userColumns, chatID, displayName))
	if err != nil {
		return u, err
	}
	_, err = s.pool.Exec(ctx, `insert into user_preferences (user_id) values ($1) on conflict do nothing`, u.ID)
	return u, err
}

// GetUser loads a user by id.
func (s *Store) GetUser(ctx context.Context, userID int64) (User, error) {
	u, err := scanUser(s.pool.QueryRow(ctx, `select `+userColumns+` from users where id = $1`, userID))
	if errors.Is(err, pgx.ErrNoRows) {
		return u, ErrNotFound
	}
	return u, err
}

// Users lists every user; the ticker evaluates each in turn.
func (s *Store) Users(ctx context.Context) ([]User, error) {
	rows, err := s.pool.Query(ctx, `select `+userColumns+` from users order by id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// --- invites ---

const inviteTTL = 7 * 24 * time.Hour

// CreateInvite returns a single-use code that lets one new person in.
func (s *Store) CreateInvite(ctx context.Context, ownerID int64) (string, error) {
	raw := make([]byte, 10)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	code := strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw))
	_, err := s.pool.Exec(ctx, `insert into invites (code, created_by, expires_at) values ($1, $2, now() + make_interval(secs => $3))`,
		code, ownerID, inviteTTL.Seconds())
	return code, err
}

var ErrBadInvite = errors.New("invite not valid")

// RedeemInvite creates a member for the chat if the code is unused and
// unexpired. The check and the claim are one statement, so two people
// cannot both get in on the same code.
func (s *Store) RedeemInvite(ctx context.Context, code string, chatID int64, displayName string) (User, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)
	var inviter int64
	err = tx.QueryRow(ctx, `
		update invites set used_at = now()
		 where code = $1 and used_at is null and expires_at > now()
		returning created_by`, strings.ToLower(strings.TrimSpace(code))).Scan(&inviter)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrBadInvite
	}
	if err != nil {
		return User{}, err
	}
	u, err := scanUser(tx.QueryRow(ctx, `
		insert into users (telegram_chat_id, display_name, role, invited_by) values ($1, nullif($2, ''), 'member', $3)
		returning `+userColumns, chatID, displayName, inviter))
	if err != nil {
		return User{}, err
	}
	if _, err := tx.Exec(ctx, `insert into user_preferences (user_id) values ($1)`, u.ID); err != nil {
		return User{}, err
	}
	if _, err := tx.Exec(ctx, `update invites set used_by = $2 where code = $1`, strings.ToLower(strings.TrimSpace(code)), u.ID); err != nil {
		return User{}, err
	}
	return u, tx.Commit(ctx)
}

// --- ownership ---
//
// Ids arrive from button presses, and button data must be treated as
// something the sender chose. These three answer the only question that
// matters before acting on one: is it theirs?

func (s *Store) OwnsThought(ctx context.Context, userID, thoughtID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `select exists (select 1 from thoughts where id = $1 and user_id = $2)`, thoughtID, userID).Scan(&ok)
	return ok, err
}

// OwnsNudge also checks the nudge is about the given thought, so a button
// cannot pair one thought with another thought's nudge.
func (s *Store) OwnsNudge(ctx context.Context, userID, nudgeID, thoughtID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `select exists (select 1 from nudges where id = $1 and user_id = $2 and thought_id = $3)`,
		nudgeID, userID, thoughtID).Scan(&ok)
	return ok, err
}

func (s *Store) OwnsReview(ctx context.Context, userID, reviewID int64) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `select exists (select 1 from reviews where id = $1 and user_id = $2)`, reviewID, userID).Scan(&ok)
	return ok, err
}

// --- per-person usage ---

// UserMonthlyCostUSD is what one person's model calls have cost this month.
func (s *Store) UserMonthlyCostUSD(ctx context.Context, userID int64) (float64, int, error) {
	var cost float64
	var n int
	err := s.pool.QueryRow(ctx, `
		select coalesce(sum(cost_usd), 0), count(*) from llm_calls
		 where user_id = $1 and created_at >= date_trunc('month', now())`, userID).Scan(&cost, &n)
	return cost, n, err
}

// UserCallsSince counts one person's model calls of one purpose.
func (s *Store) UserCallsSince(ctx context.Context, userID int64, purpose string, since time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `select count(*) from llm_calls where user_id = $1 and purpose = $2 and created_at >= $3`,
		userID, purpose, since).Scan(&n)
	return n, err
}

// CapturesSince counts the thoughts one person has sent since a moment.
func (s *Store) CapturesSince(ctx context.Context, userID int64, since time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `select count(*) from thoughts where user_id = $1 and created_at >= $2`, userID, since).Scan(&n)
	return n, err
}

// ManualRunsSince counts the nudges one person asked for with /nudge.
func (s *Store) ManualRunsSince(ctx context.Context, userID int64, since time.Time) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `select count(*) from decision_runs where user_id = $1 and trigger = 'manual' and started_at >= $2`,
		userID, since).Scan(&n)
	return n, err
}

// SetBlocked records that Telegram refused delivery because the person
// blocked the bot, and stops proactive messages to them.
func (s *Store) SetBlocked(ctx context.Context, userID int64) error {
	if _, err := s.pool.Exec(ctx, `update users set blocked_at = now() where id = $1`, userID); err != nil {
		return err
	}
	return s.SetNotifications(ctx, userID, false)
}

// UserSummary is one line of the owner's /users view. It holds counts only:
// the owner's view never shows what anyone wrote.
type UserSummary struct {
	User
	Thoughts  int
	MonthCost float64
	LastSeen  *time.Time
}

func (s *Store) UserSummaries(ctx context.Context) ([]UserSummary, error) {
	rows, err := s.pool.Query(ctx, `
		select u.id, u.telegram_chat_id, coalesce(u.display_name, ''), u.timezone, u.role, u.created_at,
		       (select count(*) from thoughts t where t.user_id = u.id),
		       coalesce((select sum(cost_usd) from llm_calls c where c.user_id = u.id and c.created_at >= date_trunc('month', now())), 0),
		       (select max(created_at) from thoughts t where t.user_id = u.id)
		  from users u order by u.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UserSummary
	for rows.Next() {
		var us UserSummary
		if err := rows.Scan(&us.ID, &us.TelegramChatID, &us.DisplayName, &us.Timezone, &us.Role, &us.CreatedAt,
			&us.Thoughts, &us.MonthCost, &us.LastSeen); err != nil {
			return nil, err
		}
		out = append(out, us)
	}
	return out, rows.Err()
}

// --- a person's own data ---

// ForgetUser deletes everything held about one person. Thoughts,
// interpretations, events, nudges, reviews, decisions and preferences go;
// the model-call log keeps its cost rows with the person removed from them.
func (s *Store) ForgetUser(ctx context.Context, userID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `update llm_calls set thought_id = null where user_id = $1`, userID); err != nil {
		return err
	}
	// Deleting the user cascades to everything they own.
	tag, err := tx.Exec(ctx, `delete from users where id = $1`, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}

// ExportUser returns everything held about one person as JSON.
func (s *Store) ExportUser(ctx context.Context, userID int64) ([]byte, error) {
	var out json.RawMessage
	err := s.pool.QueryRow(ctx, `
		select jsonb_build_object(
		  'exported_at', now(),
		  'profile', (select to_jsonb(u) - 'id' - 'invited_by' from users u where u.id = $1),
		  'routine', (select to_jsonb(p) - 'user_id' from user_preferences p where p.user_id = $1),
		  'thoughts', coalesce((
		     select jsonb_agg(jsonb_build_object(
		       'number', t.number, 'your_words', t.raw_content, 'status', t.status, 'captured_at', t.created_at,
		       'readings', coalesce((select jsonb_agg(to_jsonb(i) - 'id' - 'thought_id' order by i.version)
		                               from thought_interpretations i where i.thought_id = t.id), '[]'::jsonb),
		       'history', coalesce((select jsonb_agg(jsonb_build_object('what', e.type, 'at', e.created_at, 'detail', e.payload) order by e.created_at)
		                               from thought_events e where e.thought_id = t.id), '[]'::jsonb),
		       'nudges', coalesce((select jsonb_agg(jsonb_build_object('sent_at', n.sent_at, 'message', n.message,
		                                 'suggested_step', n.suggested_action, 'kind', n.lane, 'your_response', n.response_action) order by n.created_at)
		                               from nudges n where n.thought_id = t.id), '[]'::jsonb)
		     ) order by t.number) from thoughts t where t.user_id = $1), '[]'::jsonb)
		)`, userID).Scan(&out)
	return out, err
}
