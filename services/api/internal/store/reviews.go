package store

import (
	"context"
	"time"
)

// ReviewItem is one thought offered in a weekly review.
type ReviewItem struct {
	ThoughtID int64
	Title     string
	Type      string
	AgeDays   int
}

// ReviewCandidates returns the thoughts that have had no attention lately:
// not touched for three days and not nudged for a week. Oldest attention
// first, so the review surfaces what has drifted furthest.
func (s *Store) ReviewCandidates(ctx context.Context, userID int64, limit int) ([]ReviewItem, error) {
	rows, err := s.pool.Query(ctx, `
		select t.id, i.title, i.type, extract(day from now() - t.created_at)::int
		  from thoughts t join thought_interpretations i on i.id = t.current_interpretation_id
		 where t.user_id = $1 and t.status = 'active' and i.type <> 'unclear'
		   and t.last_touched_at < now() - interval '3 days'
		   and (t.last_nudged_at is null or t.last_nudged_at < now() - interval '7 days')
		 order by t.last_touched_at limit $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReviewItem
	for rows.Next() {
		var it ReviewItem
		if err := rows.Scan(&it.ThoughtID, &it.Title, &it.Type, &it.AgeDays); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// ReviewCheckIns returns thoughts the user said "Let's do it" to in the last
// week that are still open. Asking "did it get done?" is the only way the
// app learns what happened after the tap.
func (s *Store) ReviewCheckIns(ctx context.Context, userID int64, limit int) ([]ReviewItem, error) {
	rows, err := s.pool.Query(ctx, `
		select t.id, i.title, i.type, extract(day from now() - t.created_at)::int
		  from thoughts t join thought_interpretations i on i.id = t.current_interpretation_id
		 where t.user_id = $1 and t.status = 'active'
		   and exists (select 1 from nudges n where n.thought_id = t.id and n.response_action in ('do_it', 'next_steps')
		                  and n.responded_at >= now() - interval '7 days')
		   and not exists (select 1 from thought_events e where e.thought_id = t.id and e.type in ('review_done', 'review_not_yet')
		                  and e.created_at >= now() - interval '6 days')
		 order by t.last_touched_at desc limit $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ReviewItem
	for rows.Next() {
		var it ReviewItem
		if err := rows.Scan(&it.ThoughtID, &it.Title, &it.Type, &it.AgeDays); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// WeekSummary is the plain record shown at the top of a review.
type WeekSummary struct {
	Sent, Acted, Done, Captured int
}

func (s *Store) WeekSummary(ctx context.Context, userID int64) (WeekSummary, error) {
	var w WeekSummary
	err := s.pool.QueryRow(ctx, `
		select (select count(*) from nudges where user_id = $1 and sent_at >= now() - interval '7 days'),
		       (select count(*) from nudges where user_id = $1 and sent_at >= now() - interval '7 days'
		           and response_action in ('do_it', 'next_steps')),
		       (select count(*) from thought_events e join thoughts t on t.id = e.thought_id
		         where t.user_id = $1 and e.type = 'status_done' and e.created_at >= now() - interval '7 days'),
		       (select count(*) from thoughts where user_id = $1 and created_at >= now() - interval '7 days'
		           and status <> 'archived')`, userID).Scan(&w.Sent, &w.Acted, &w.Done, &w.Captured)
	return w, err
}

// CreateReview records a review. trigger is "scheduled" for the weekly one
// and "manual" for /review; only scheduled reviews count against the budget.
func (s *Store) CreateReview(ctx context.Context, userID int64, itemCount int, trigger string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `insert into reviews (user_id, item_count, trigger) values ($1, $2, $3) returning id`,
		userID, itemCount, trigger).Scan(&id)
	return id, err
}

func (s *Store) DeleteReview(ctx context.Context, reviewID int64) error {
	_, err := s.pool.Exec(ctx, `delete from reviews where id = $1`, reviewID)
	return err
}

func (s *Store) CountReviewTap(ctx context.Context, reviewID int64) error {
	_, err := s.pool.Exec(ctx, `update reviews set tap_count = tap_count + 1 where id = $1`, reviewID)
	return err
}

// Review is a past weekly review and whether the user engaged with it.
type Review struct {
	SentAt    time.Time
	ItemCount int
	TapCount  int
}

// LastReviews returns the newest scheduled reviews first. Reviews the user
// asked for are left out: they must not delay or pause the weekly one.
func (s *Store) LastReviews(ctx context.Context, userID int64, limit int) ([]Review, error) {
	rows, err := s.pool.Query(ctx, `select sent_at, item_count, tap_count from reviews
		where user_id = $1 and trigger = 'scheduled' order by sent_at desc limit $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Review
	for rows.Next() {
		var r Review
		if err := rows.Scan(&r.SentAt, &r.ItemCount, &r.TapCount); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
