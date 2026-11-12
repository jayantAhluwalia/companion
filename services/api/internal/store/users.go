package store

import (
	"context"
	"time"
)

type User struct {
	ID             int64
	TelegramChatID int64
	DisplayName    string
	Timezone       string
	CreatedAt      time.Time
}

// GetOrCreateUser is idempotent: the bot calls it on every message.
func (s *Store) GetOrCreateUser(ctx context.Context, chatID int64, displayName string) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		with ins as (
			insert into users (telegram_chat_id, display_name)
			values ($1, nullif($2, ''))
			on conflict (telegram_chat_id) do nothing
			returning id, telegram_chat_id, display_name, timezone, created_at
		)
		select id, telegram_chat_id, coalesce(display_name, ''), timezone, created_at from ins
		union all
		select id, telegram_chat_id, coalesce(display_name, ''), timezone, created_at
		  from users where telegram_chat_id = $1
		limit 1`, chatID, displayName,
	).Scan(&u.ID, &u.TelegramChatID, &u.DisplayName, &u.Timezone, &u.CreatedAt)
	if err != nil {
		return u, err
	}
	// Preferences row with defaults, created once.
	_, err = s.pool.Exec(ctx, `insert into user_preferences (user_id) values ($1) on conflict do nothing`, u.ID)
	return u, err
}
