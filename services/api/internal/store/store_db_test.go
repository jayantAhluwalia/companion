//go:build db

// These tests run the store's SQL against a real Postgres. They create a
// throwaway schema, apply every migration to it and drop it afterwards, so
// they are safe to point at any database:  make dbtest
package store

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"companion/api/internal/ai"
)

var db *Store

func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		fmt.Println("TEST_DATABASE_URL not set; skipping database tests")
		os.Exit(0)
	}
	schema := fmt.Sprintf("test_%d", time.Now().UnixNano()%1_000_000_000)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	code := 1
	func() {
		defer func() {
			if err := DropSchema(context.Background(), url, schema); err != nil {
				fmt.Println("could not drop", schema, err)
			}
		}()
		if err := Migrate(ctx, url, schema); err != nil {
			fmt.Println("migrate:", err)
			return
		}
		var err error
		if db, err = Open(ctx, url, schema); err != nil {
			fmt.Println("open:", err)
			return
		}
		defer db.Close()
		code = m.Run()
	}()
	os.Exit(code)
}

var nextChat int64 = 1000

func newUser(t *testing.T) User {
	t.Helper()
	nextChat++
	u, err := db.GetOrCreateUser(t.Context(), nextChat, "Test")
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func newThought(t *testing.T, u User, text string, interpreted bool) Thought {
	t.Helper()
	th, created, err := db.CreateThought(t.Context(), u.ID, text, "test", 0)
	if err != nil || !created {
		t.Fatalf("create thought: created=%v err=%v", created, err)
	}
	if interpreted {
		in := ai.Interpretation{Type: ai.TypeActionable, Title: text, Confidence: 0.9, Urgency: ai.UrgencyWhenever}
		if _, err := db.AddInterpretation(t.Context(), th.ID, "ai", in, "m", "v"); err != nil {
			t.Fatal(err)
		}
	}
	return th
}

// sendNudge records a nudge as sent the given time ago, in the given lane.
func sendNudge(t *testing.T, u User, th Thought, lane, trigger string, ago time.Duration) int64 {
	t.Helper()
	ctx := t.Context()
	run, err := db.StartRun(ctx, u.ID, trigger)
	if err != nil {
		t.Fatal(err)
	}
	id, err := db.CreateNudge(ctx, Nudge{UserID: u.ID, ThoughtID: th.ID, DecisionRunID: run, NudgeType: "check_in",
		Message: "m", Channel: "test", Lane: lane, Window: "evening"})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.MarkNudgeSent(ctx, id, th.ID, "1"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(ctx, `update nudges set sent_at = now() - make_interval(secs => $2) where id = $1`, id, ago.Seconds()); err != nil {
		t.Fatal(err)
	}
	return id
}

func TestGetOrCreateUserIsIdempotent(t *testing.T) {
	a, err := db.GetOrCreateUser(t.Context(), 42, "A")
	if err != nil {
		t.Fatal(err)
	}
	b, err := db.GetOrCreateUser(t.Context(), 42, "A again")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != b.ID {
		t.Fatalf("same chat gave two users: %d and %d", a.ID, b.ID)
	}
	if _, err := db.GetPrefs(t.Context(), a.ID); err != nil {
		t.Fatalf("a new user should have preferences: %v", err)
	}
}

func TestSameMessageIsStoredOnce(t *testing.T) {
	u := newUser(t)
	ctx := t.Context()
	first, created, err := db.CreateThought(ctx, u.ID, "buy milk", "telegram", 777)
	if err != nil || !created {
		t.Fatalf("first: created=%v err=%v", created, err)
	}
	_, created, err = db.CreateThought(ctx, u.ID, "buy milk", "telegram", 777)
	if err != nil || created {
		t.Fatalf("a redelivered message must not create a second thought: created=%v err=%v", created, err)
	}
	// The same message number from someone else is a different message.
	other := newUser(t)
	if _, created, err = db.CreateThought(ctx, other.ID, "buy milk", "telegram", 777); err != nil || !created {
		t.Fatalf("another user's message 777: created=%v err=%v", created, err)
	}
	// No message id (a correction-path capture) never collides.
	for i := 0; i < 2; i++ {
		if _, created, err = db.CreateThought(ctx, u.ID, "no id", "telegram", 0); err != nil || !created {
			t.Fatalf("capture without a message id: created=%v err=%v", created, err)
		}
	}
	if got, err := db.GetThought(ctx, u.ID, first.ID); err != nil || got.RawContent != "buy milk" {
		t.Fatalf("read back: %+v %v", got, err)
	}
}

func TestUninterpretedIsRetriedUpToTheCap(t *testing.T) {
	u := newUser(t)
	ctx := t.Context()
	stuck := newThought(t, u, "never read", false)
	read := newThought(t, u, "read fine", true)
	if _, err := db.pool.Exec(ctx, `update thoughts set created_at = now() - interval '10 minutes' where id = any($1)`, []int64{stuck.ID, read.ID}); err != nil {
		t.Fatal(err)
	}
	has := func() bool {
		list, err := db.Uninterpreted(ctx, 2*time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, th := range list {
			if th.ID == read.ID {
				t.Fatal("an interpreted thought must not be retried")
			}
			found = found || th.ID == stuck.ID
		}
		return found
	}
	if !has() {
		t.Fatal("a ten-minute-old uninterpreted thought should be picked up")
	}
	for i := 0; i < MaxInterpretAttempts; i++ {
		if err := db.CountInterpretAttempt(ctx, stuck.ID); err != nil {
			t.Fatal(err)
		}
	}
	if has() {
		t.Fatal("after the attempt cap it is left alone")
	}
	fresh := newThought(t, u, "just now", false)
	list, _ := db.Uninterpreted(ctx, 2*time.Minute)
	for _, th := range list {
		if th.ID == fresh.ID {
			t.Fatal("a thought captured seconds ago is still being read; do not retry yet")
		}
	}
}

func TestExpireCountsOnlyAutomaticBudgetedNudgesAsIgnored(t *testing.T) {
	u := newUser(t)
	ctx := t.Context()
	quick := newThought(t, u, "quick", true)
	deadline := newThought(t, u, "deadline", true)
	asked := newThought(t, u, "asked for", true)
	recent := newThought(t, u, "recent", true)
	sendNudge(t, u, quick, "quick", "ticker", 30*time.Hour)
	sendNudge(t, u, deadline, "deadline", "ticker", 30*time.Hour)
	sendNudge(t, u, asked, "quick", "manual", 30*time.Hour)
	sendNudge(t, u, recent, "quick", "ticker", 2*time.Hour)

	n, err := db.ExpireNudges(ctx, u.ID, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("three nudges are past 24 hours, expired %d", n)
	}
	streak := func(th Thought) int {
		got, err := db.GetThought(ctx, u.ID, th.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.Status != "active" {
			t.Errorf("%q should stay active, is %s", th.RawContent, got.Status)
		}
		return got.IgnoredStreak
	}
	if streak(quick) != 1 {
		t.Error("an unanswered automatic quick nudge is one ignore")
	}
	if streak(deadline) != 0 {
		t.Error("an unanswered deadline notice is not an ignore")
	}
	if streak(asked) != 0 {
		t.Error("a nudge the user asked for is not an ignore")
	}
	if streak(recent) != 0 {
		t.Error("a two-hour-old nudge has not expired")
	}
	// Running it again changes nothing.
	if n, _ := db.ExpireNudges(ctx, u.ID, 24*time.Hour); n != 0 {
		t.Errorf("second pass expired %d more", n)
	}
}

func TestLoadCountsLanesSeparatelyAndAdoptsScheduled(t *testing.T) {
	u := newUser(t)
	ctx := t.Context()
	th := newThought(t, u, "x", true)
	now := time.Now()
	dayStart := now.Add(-6 * time.Hour)

	sendNudge(t, u, th, "quick", "ticker", 1*time.Hour)
	sendNudge(t, u, th, "opportunity", "ticker", 3*24*time.Hour)
	sendNudge(t, u, th, "deadline", "ticker", 2*time.Hour)
	sendNudge(t, u, th, "quick", "manual", 30*time.Minute) // /nudge: counts for nothing but the gap
	sendNudge(t, u, th, "quick", "ticker", 9*24*time.Hour) // outside the week

	l, err := db.NudgeLoad(ctx, u.ID, dayStart, now)
	if err != nil {
		t.Fatal(err)
	}
	if l.BudgetedToday != 1 || l.ObligationToday != 1 || l.BudgetedWeek != 2 || l.QuickWeek != 1 {
		t.Fatalf("load = %+v; want budgeted today 1, obligation today 1, budgeted week 2, quick week 1", l)
	}
	if l.LastSent == nil || now.Sub(*l.LastSent) > 40*time.Minute {
		t.Errorf("the last message of any kind was 30 minutes ago, got %v", l.LastSent)
	}

	// A nudge created but never marked sent (a crash after delivery) still counts.
	run, _ := db.StartRun(ctx, u.ID, "ticker")
	if _, err := db.CreateNudge(ctx, Nudge{UserID: u.ID, ThoughtID: th.ID, DecisionRunID: run, NudgeType: "check_in",
		Message: "m", Channel: "test", Lane: "opportunity"}); err != nil {
		t.Fatal(err)
	}
	l, _ = db.NudgeLoad(ctx, u.ID, dayStart, now)
	if l.BudgetedToday != 2 {
		t.Errorf("a scheduled nudge must count as sent, budgeted today = %d", l.BudgetedToday)
	}

	// Reviews: the scheduled one uses the budget, an on-request one is free.
	if _, err := db.CreateReview(ctx, u.ID, 3, "manual"); err != nil {
		t.Fatal(err)
	}
	if l, _ = db.NudgeLoad(ctx, u.ID, dayStart, now); l.ReviewsWeek != 0 {
		t.Errorf("/review must not use the budget, reviews this week = %d", l.ReviewsWeek)
	}
	if _, err := db.CreateReview(ctx, u.ID, 3, "scheduled"); err != nil {
		t.Fatal(err)
	}
	if l, _ = db.NudgeLoad(ctx, u.ID, dayStart, now); l.ReviewsWeek != 1 {
		t.Errorf("the weekly review counts once, got %d", l.ReviewsWeek)
	}
	last, err := db.LastReviews(ctx, u.ID, 5)
	if err != nil || len(last) != 1 {
		t.Errorf("only scheduled reviews decide when the next is due: %d %v", len(last), err)
	}
}

func TestIgnoredRunIsCountedFromTheNewest(t *testing.T) {
	u := newUser(t)
	ctx := t.Context()
	th := newThought(t, u, "x", true)
	answered := sendNudge(t, u, th, "quick", "ticker", 5*24*time.Hour)
	if err := db.RespondNudge(ctx, answered, "do_it"); err != nil {
		t.Fatal(err)
	}
	sendNudge(t, u, th, "quick", "ticker", 3*24*time.Hour)
	sendNudge(t, u, th, "opportunity", "ticker", 2*24*time.Hour)
	if _, err := db.ExpireNudges(ctx, u.ID, 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	l, err := db.NudgeLoad(ctx, u.ID, time.Now().Add(-time.Hour), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if l.IgnoredRun != 2 {
		t.Errorf("two unanswered after one answered: run = %d, want 2", l.IgnoredRun)
	}
}

func TestWindowTalliesUseOnlyFinalOutcomes(t *testing.T) {
	u := newUser(t)
	ctx := t.Context()
	th := newThought(t, u, "x", true)
	acted := sendNudge(t, u, th, "quick", "ticker", 3*24*time.Hour)
	if err := db.RespondNudge(ctx, acted, "do_it"); err != nil {
		t.Fatal(err)
	}
	sendNudge(t, u, th, "quick", "ticker", 2*24*time.Hour) // will expire
	later := sendNudge(t, u, th, "quick", "ticker", 4*24*time.Hour)
	if err := db.RespondNudge(ctx, later, "later"); err != nil {
		t.Fatal(err)
	}
	sendNudge(t, u, th, "deadline", "ticker", 5*24*time.Hour) // not a budgeted lane
	yesterday := sendNudge(t, u, th, "quick", "ticker", 3*time.Hour)
	if err := db.RespondNudge(ctx, yesterday, "do_it"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExpireNudges(ctx, u.ID, 24*time.Hour); err != nil {
		t.Fatal(err)
	}
	tallies, err := db.WindowTallies(ctx, u.ID, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	got := tallies["evening"]
	if got.Acted != 1 || got.Ignored != 1 {
		t.Errorf("evening = %+v; want 1 acted, 1 ignored ('later', deadlines and the last 24 hours are left out)", got)
	}
}

func TestFinishRunAlwaysStoresAList(t *testing.T) {
	u := newUser(t)
	ctx := t.Context()
	type scored struct{ ID int }
	var none []scored // a nil slice, as the engine passes for a blocked tick
	for _, scores := range []any{nil, none, []scored{}, []scored{{ID: 1}}} {
		run, err := db.StartRun(ctx, u.ID, "ticker")
		if err != nil {
			t.Fatal(err)
		}
		if err := db.FinishRun(ctx, run, 0, scores, nil, "blocked", "test"); err != nil {
			t.Fatalf("finish with %#v: %v", scores, err)
		}
		var kind string
		if err := db.pool.QueryRow(ctx, `select jsonb_typeof(scores) from decision_runs where id = $1`, run).Scan(&kind); err != nil {
			t.Fatal(err)
		}
		if kind != "array" {
			t.Errorf("scores for %#v stored as JSON %s, want a list", scores, kind)
		}
	}
}

func TestKVRoundTrip(t *testing.T) {
	ctx := t.Context()
	if v, err := db.GetKV(ctx, "missing"); err != nil || v != "" {
		t.Fatalf("a missing key is empty, got %q %v", v, err)
	}
	for _, v := range []string{"10", "11"} {
		if err := db.SetKV(ctx, "offset", v); err != nil {
			t.Fatal(err)
		}
	}
	if v, _ := db.GetKV(ctx, "offset"); v != "11" {
		t.Fatalf("want the latest value, got %q", v)
	}
}

func TestSchemaNameIsValidated(t *testing.T) {
	for _, bad := range []string{`x"; drop schema public cascade; --`, "Has-Dash", "1starts_with_digit", "has space"} {
		if _, err := setSchema(bad); err == nil {
			t.Errorf("schema name %q should be refused", bad)
		}
	}
	if err := DropSchema(t.Context(), os.Getenv("TEST_DATABASE_URL"), "public"); err == nil {
		t.Fatal("dropping public must be refused")
	}
}
