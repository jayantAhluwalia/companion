package store

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// Clock is a time of day in minutes after midnight. Unset means "not given".
type Clock int

const Unset Clock = -1

func (c Clock) String() string {
	if c < 0 {
		return ""
	}
	return fmt.Sprintf("%02d:%02d", int(c)/60, int(c)%60)
}

// ClockOf returns the time of day of t in t's own location.
func ClockOf(t time.Time) Clock { return Clock(t.Hour()*60 + t.Minute()) }

// Between reports whether c lies in [start, end), where the window may wrap
// past midnight (23:00 to 07:00).
func (c Clock) Between(start, end Clock) bool {
	if start < 0 || end < 0 || start == end {
		return false
	}
	if start < end {
		return c >= start && c < end
	}
	return c >= start || c < end
}

// Prefs is the user's routine and nudge limits.
type Prefs struct {
	QuietStart, QuietEnd Clock // the sleep window
	WorkStart, WorkEnd   Clock // Unset when there are no fixed hours
	WorkDays, DaysOff    []int // ISO weekdays, 1 = Monday
	NudgeDuringWork      string
	DailyLimit           int
	MinIntervalMinutes   int
	NotificationsEnabled bool
	OnboardedAt          *time.Time
	HomeCity             string
	HomeCountry          string
	HomeLat, HomeLon     *float64
}

func (p Prefs) HasHome() bool { return p.HomeLat != nil && p.HomeLon != nil }

func (p Prefs) HasWorkHours() bool { return p.WorkStart >= 0 && p.WorkEnd >= 0 }

func (p Prefs) IsDayOff(isoWeekday int) bool {
	for _, d := range p.DaysOff {
		if d == isoWeekday {
			return true
		}
	}
	return false
}

func (p Prefs) IsWorkDay(isoWeekday int) bool {
	for _, d := range p.WorkDays {
		if d == isoWeekday {
			return true
		}
	}
	return false
}

func parseClock(s *string) Clock {
	if s == nil {
		return Unset
	}
	var h, m int
	if _, err := fmt.Sscanf(*s, "%d:%d", &h, &m); err != nil {
		return Unset
	}
	return Clock(h*60 + m)
}

func clockArg(c Clock) *string {
	if c < 0 {
		return nil
	}
	s := c.String()
	return &s
}

func (s *Store) GetPrefs(ctx context.Context, userID int64) (Prefs, error) {
	var p Prefs
	var qs, qe, ws, we *string
	var workDays, daysOff []int32
	var city, country *string
	err := s.pool.QueryRow(ctx, `
		select to_char(quiet_start, 'HH24:MI'), to_char(quiet_end, 'HH24:MI'),
		       to_char(work_start, 'HH24:MI'), to_char(work_end, 'HH24:MI'),
		       work_days, days_off, nudge_during_work, daily_limit, min_interval_minutes,
		       notifications_enabled, onboarded_at, home_city, home_country, home_lat, home_lon
		  from user_preferences where user_id = $1`, userID,
	).Scan(&qs, &qe, &ws, &we, &workDays, &daysOff, &p.NudgeDuringWork, &p.DailyLimit, &p.MinIntervalMinutes,
		&p.NotificationsEnabled, &p.OnboardedAt, &city, &country, &p.HomeLat, &p.HomeLon)
	if err != nil {
		return p, err
	}
	p.QuietStart, p.QuietEnd = parseClock(qs), parseClock(qe)
	p.WorkStart, p.WorkEnd = parseClock(ws), parseClock(we)
	for _, d := range workDays {
		p.WorkDays = append(p.WorkDays, int(d))
	}
	for _, d := range daysOff {
		p.DaysOff = append(p.DaysOff, int(d))
	}
	if city != nil {
		p.HomeCity = *city
	}
	if country != nil {
		p.HomeCountry = *country
	}
	return p, nil
}

func (s *Store) SetSleep(ctx context.Context, userID int64, start, end Clock) error {
	_, err := s.pool.Exec(ctx, `update user_preferences set quiet_start = $2::time, quiet_end = $3::time, updated_at = now()
		where user_id = $1`, userID, start.String(), end.String())
	return err
}

// SetWork stores working hours; Unset for both means no fixed hours.
func (s *Store) SetWork(ctx context.Context, userID int64, start, end Clock) error {
	_, err := s.pool.Exec(ctx, `update user_preferences set work_start = $2::time, work_end = $3::time, updated_at = now()
		where user_id = $1`, userID, clockArg(start), clockArg(end))
	return err
}

func (s *Store) SetNudgeDuringWork(ctx context.Context, userID int64, mode string) error {
	_, err := s.pool.Exec(ctx, `update user_preferences set nudge_during_work = $2, updated_at = now() where user_id = $1`, userID, mode)
	return err
}

// SetDaysOff stores the days off and makes every other day a work day.
func (s *Store) SetDaysOff(ctx context.Context, userID int64, daysOff []int) error {
	off := map[int]bool{}
	for _, d := range daysOff {
		off[d] = true
	}
	offArr, workArr := []int32{}, []int32{}
	for d := 1; d <= 7; d++ {
		if off[d] {
			offArr = append(offArr, int32(d))
		} else {
			workArr = append(workArr, int32(d))
		}
	}
	_, err := s.pool.Exec(ctx, `update user_preferences set days_off = $2, work_days = $3, updated_at = now() where user_id = $1`,
		userID, offArr, workArr)
	return err
}

// SetHome stores where the user lives and, when known, moves their timezone
// with it, since quiet hours are evaluated in local time.
func (s *Store) SetHome(ctx context.Context, userID int64, city, country string, lat, lon float64, timezone string) error {
	if _, err := s.pool.Exec(ctx, `update user_preferences
		set home_city = $2, home_country = nullif($3, ''), home_lat = $4, home_lon = $5, updated_at = now()
		where user_id = $1`, userID, city, country, lat, lon); err != nil {
		return err
	}
	if timezone == "" {
		return nil
	}
	if _, err := time.LoadLocation(timezone); err != nil {
		return nil // keep the old zone rather than store one Go cannot load
	}
	_, err := s.pool.Exec(ctx, `update users set timezone = $2 where id = $1`, userID, timezone)
	return err
}

func (s *Store) MarkOnboarded(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `update user_preferences set onboarded_at = coalesce(onboarded_at, now()) where user_id = $1`, userID)
	return err
}

func (s *Store) SetNotifications(ctx context.Context, userID int64, on bool) error {
	_, err := s.pool.Exec(ctx, `update user_preferences set notifications_enabled = $2, updated_at = now() where user_id = $1`, userID, on)
	return err
}

// Pending is the one piece of conversational state: the bot asked a question
// whose answer is typed rather than tapped, and the next message is that answer.
type Pending struct {
	Kind      string    `json:"kind"` // retitle | clarify | city | sleep | work
	ThoughtID int64     `json:"thought_id,omitempty"`
	Chain     bool      `json:"chain,omitempty"` // onboarding: continue to the next question
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Store) SetPending(ctx context.Context, userID int64, p *Pending) error {
	var raw []byte
	if p != nil {
		raw, _ = json.Marshal(p)
	}
	_, err := s.pool.Exec(ctx, `update users set pending = $2 where id = $1`, userID, raw)
	return err
}

// TakePending returns the live pending question, if any, and clears it.
func (s *Store) TakePending(ctx context.Context, userID int64) (*Pending, error) {
	var raw []byte
	err := s.pool.QueryRow(ctx, `
		with old as (select pending from users where id = $1 for update)
		update users u set pending = null from old where u.id = $1 returning old.pending`, userID).Scan(&raw)
	if err != nil || raw == nil {
		return nil, err
	}
	var p Pending
	if json.Unmarshal(raw, &p) != nil || time.Now().After(p.ExpiresAt) {
		return nil, nil
	}
	return &p, nil
}
