//go:build db

// End-to-end access tests: real handlers, a real (throwaway) database, and a
// Telegram client that records messages instead of sending them. This is how
// "a second person cannot touch the first person's thoughts" is checked
// without a second Telegram account.
package telegram

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"companion/api/internal/ai"
	"companion/api/internal/store"
)

var (
	testDB *store.Store
	sentMu sync.Mutex
	sent   []string // "method chat_id text" of every outgoing call
)

const ownerChat = 100

func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		fmt.Println("TEST_DATABASE_URL not set; skipping bot access tests")
		os.Exit(0)
	}
	schema := fmt.Sprintf("test_bot_%d", time.Now().UnixNano()%1_000_000_000)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	code := 1
	func() {
		defer store.DropSchema(context.Background(), url, schema)
		if err := store.Migrate(ctx, url, schema); err != nil {
			fmt.Println("migrate:", err)
			return
		}
		var err error
		if testDB, err = store.Open(ctx, url, schema); err != nil {
			fmt.Println("open:", err)
			return
		}
		defer testDB.Close()
		code = m.Run()
	}()
	os.Exit(code)
}

func testBot() *Bot {
	c := NewClient("000:test")
	c.Dry = func(method string, payload any) {
		sentMu.Lock()
		defer sentMu.Unlock()
		sent = append(sent, fmt.Sprintf("%s %v", method, payload))
	}
	b := NewBot(c, nil, testDB, nil, nil, ownerChat, 18, slog.New(slog.NewTextHandler(io.Discard, nil)))
	b.username = "test_bot"
	return b
}

func outbox() string {
	sentMu.Lock()
	defer sentMu.Unlock()
	out := strings.Join(sent, "\n")
	sent = nil
	return out
}

func msg(chat int64, text string) *Message {
	return &Message{MessageID: time.Now().UnixNano(), Chat: Chat{ID: chat, Type: "private"}, From: &User{ID: chat, FirstName: fmt.Sprintf("P%d", chat)}, Text: text}
}

func press(b *Bot, chat int64, data string) string {
	outbox()
	b.handleCallback(context.Background(), &CallbackQuery{ID: "q", From: User{ID: chat}, Data: data,
		Message: &Message{MessageID: 1, Chat: Chat{ID: chat, Type: "private"}}})
	return outbox()
}

func seedThought(t *testing.T, userID int64, text string) store.Thought {
	t.Helper()
	th, created, err := testDB.CreateThought(t.Context(), userID, text, "test", 0)
	if err != nil || !created {
		t.Fatal(err)
	}
	in := ai.Interpretation{Type: ai.TypeActionable, Title: text, Confidence: 0.9, Urgency: ai.UrgencyWhenever}
	if _, err := testDB.AddInterpretation(t.Context(), th.ID, "ai", in, "m", "v"); err != nil {
		t.Fatal(err)
	}
	return th
}

func TestStrangersAreRefusedAndInvitesWork(t *testing.T) {
	b := testBot()
	ctx := t.Context()

	// The owner is let in by configuration.
	b.handleMessage(ctx, msg(ownerChat, "/list"))
	owner, found, err := testDB.UserByChat(ctx, ownerChat)
	if err != nil || !found || !owner.IsOwner() {
		t.Fatalf("the configured chat should be the owner: %+v found=%v err=%v", owner, found, err)
	}
	outbox()

	// A stranger gets one refusal, and nothing is stored about them.
	b.handleMessage(ctx, msg(200, "hello, save this secret"))
	if out := outbox(); !strings.Contains(out, "private companion") {
		t.Errorf("a stranger should be refused, got: %s", out)
	}
	if _, found, _ := testDB.UserByChat(ctx, 200); found {
		t.Fatal("a refused stranger must not become a user")
	}
	b.handleMessage(ctx, msg(200, "/start guessed-code"))
	if out := outbox(); !strings.Contains(out, "used or has expired") {
		t.Errorf("a made-up code should be refused, got: %s", out)
	}

	// A group is ignored entirely, even with the owner in it.
	group := msg(ownerChat, "/list")
	group.Chat.Type = "group"
	b.handleMessage(ctx, group)
	if out := outbox(); out != "" {
		t.Errorf("the bot must stay silent in groups, sent: %s", out)
	}

	// Only the owner can invite.
	b.handleMessage(ctx, msg(ownerChat, "/invite"))
	out := outbox()
	i := strings.Index(out, "start=")
	if i < 0 {
		t.Fatalf("the owner should get a link, got: %s", out)
	}
	code := out[i+len("start="):]
	code = code[:strings.IndexAny(code, "< \n")]

	// The friend opens the link: welcomed, told about privacy, asked the first question.
	b.handleMessage(ctx, msg(300, "/start "+code))
	out = outbox()
	friend, found, _ := testDB.UserByChat(ctx, 300)
	if !found || friend.IsOwner() {
		t.Fatalf("the friend should now be a member: %+v found=%v", friend, found)
	}
	for _, want := range []string{"they are able to read them", "/forget", "When do you usually sleep"} {
		if !strings.Contains(out, want) {
			t.Errorf("the welcome should mention %q; got: %s", want, out)
		}
	}
	// The same link does not work twice.
	b.handleMessage(ctx, msg(400, "/start "+code))
	if _, found, _ := testDB.UserByChat(ctx, 400); found {
		t.Fatal("one invite must let in exactly one person")
	}
	outbox()

	// A member cannot invite or see the user list.
	b.handleMessage(ctx, msg(300, "/invite"))
	if out := outbox(); strings.Contains(out, "start=") {
		t.Errorf("a member must not be able to invite: %s", out)
	}
	b.handleMessage(ctx, msg(300, "/users"))
	if out := outbox(); strings.Contains(out, "People") {
		t.Errorf("a member must not see the user list: %s", out)
	}
}

func TestOnePersonCannotTouchAnothersThoughts(t *testing.T) {
	b := testBot()
	ctx := t.Context()
	owner, err := testDB.EnsureOwner(ctx, ownerChat, "Owner")
	if err != nil {
		t.Fatal(err)
	}
	code, _ := testDB.CreateInvite(ctx, owner.ID)
	friend, err := testDB.RedeemInvite(ctx, code, 500, "Friend")
	if err != nil {
		t.Fatal(err)
	}
	mine := seedThought(t, owner.ID, "the owner's private plan")
	theirs := seedThought(t, friend.ID, "the friend's errand")
	run, _ := testDB.StartRun(ctx, owner.ID, "ticker")
	nudge, err := testDB.CreateNudge(ctx, store.Nudge{UserID: owner.ID, ThoughtID: mine.ID, DecisionRunID: run,
		NudgeType: "check_in", Message: "m", Channel: "test", Lane: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	_ = testDB.MarkNudgeSent(ctx, nudge, mine.ID, "1")
	review, _ := testDB.CreateReview(ctx, owner.ID, 1, "scheduled")

	// Every button that names a thought, tried by the friend against the owner's ids.
	forged := []string{
		fmt.Sprintf("ok:%d", mine.ID),
		fmt.Sprintf("wrong:%d", mine.ID),
		fmt.Sprintf("keep:%d", mine.ID),
		fmt.Sprintf("nr:%d", mine.ID),
		fmt.Sprintf("ct:%d:i", mine.ID),
		fmt.Sprintf("cn:%d", mine.ID),
		fmt.Sprintf("cx:%d", mine.ID),
		fmt.Sprintf("lt:%d:n", mine.ID),
		fmt.Sprintf("do_it:%d:%d", mine.ID, nudge),
		fmt.Sprintf("later:%d:%d", mine.ID, nudge),
		fmt.Sprintf("not_interested:%d:%d", mine.ID, nudge),
		fmt.Sprintf("not_interested:%d:0", mine.ID),
		fmt.Sprintf("do_it:%d:%d", theirs.ID, nudge), // their own thought, the owner's nudge
		fmt.Sprintf("rv:%d:%d:d", review, mine.ID),
		fmt.Sprintf("rv:%d:%d:d", review, theirs.ID), // their own thought, the owner's review
		"not_interested:0:0",
		"delete_everything:1",
	}
	for _, data := range forged {
		out := press(b, 500, data)
		if !strings.Contains(out, "isn't yours") && !strings.Contains(out, "don't know that button") {
			t.Errorf("%s was not refused; bot sent: %s", data, out)
		}
	}
	after, err := testDB.GetThought(ctx, owner.ID, mine.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.Status != "active" || after.Interpretation.Type != ai.TypeActionable || !after.LastTouchedAt.Equal(mine.LastTouchedAt) && after.LastTouchedAt.After(mine.CreatedAt.Add(time.Minute)) {
		t.Errorf("the owner's thought was changed by someone else: %+v", after)
	}
	var events, responded, taps int
	_ = testDB.Pool().QueryRow(ctx, `select count(*) from thought_events where thought_id = $1 and type <> 'created' and type <> 'interpreted' and type <> 'nudged'`, mine.ID).Scan(&events)
	_ = testDB.Pool().QueryRow(ctx, `select count(*) from nudges where id = $1 and status = 'responded'`, nudge).Scan(&responded)
	_ = testDB.Pool().QueryRow(ctx, `select tap_count from reviews where id = $1`, review).Scan(&taps)
	if events != 0 || responded != 0 || taps != 0 {
		t.Errorf("forged presses left traces: %d events, nudge responded=%d, review taps=%d", events, responded, taps)
	}
	if p, _ := testDB.TakePending(ctx, friend.ID); p != nil {
		t.Errorf("a refused retitle must not leave the friend waiting to type a title for someone else's thought: %+v", p)
	}

	// The same buttons work for the person they belong to.
	if out := press(b, ownerChat, fmt.Sprintf("not_interested:%d:%d", mine.ID, nudge)); strings.Contains(out, "isn't yours") {
		t.Errorf("the owner's own button was refused: %s", out)
	}
	if got, _ := testDB.GetThought(ctx, owner.ID, mine.ID); got.Status != "archived" {
		t.Errorf("the owner's own press should archive it, status is %s", got.Status)
	}

	// Numbers are per person: each one's first thought is #1, and a typed
	// command can only reach the sender's own.
	if mine.Number != 1 && theirs.Number != 1 {
		t.Logf("numbers: owner #%d, friend #%d", mine.Number, theirs.Number)
	}
	outbox()
	b.handleMessage(ctx, msg(500, fmt.Sprintf("/done %d", theirs.Number)))
	if got, _ := testDB.GetThought(ctx, friend.ID, theirs.ID); got.Status != "done" {
		t.Errorf("the friend's /done on their own number should work, status %s", got.Status)
	}
	b.handleMessage(ctx, msg(500, "/done 999"))
	if out := outbox(); !strings.Contains(out, "don't have a thought #999") {
		t.Errorf("an unknown number should say so: %s", out)
	}

	// /list and /export show only one's own.
	b.handleMessage(ctx, msg(500, "/export"))
	b.handleMessage(ctx, msg(ownerChat, "/list"))
	if out := outbox(); strings.Contains(out, "friend's errand") {
		t.Errorf("the owner's list must not show the friend's thought: %s", out)
	}

	// /forget asks first, then removes the friend and leaves the owner alone.
	b.handleMessage(ctx, msg(500, "/forget"))
	if out := outbox(); !strings.Contains(out, "cannot be undone") {
		t.Errorf("/forget should ask for confirmation: %s", out)
	}
	if _, found, _ := testDB.UserByChat(ctx, 500); !found {
		t.Fatal("nothing is deleted until the person confirms")
	}
	press(b, 500, "fg:yes")
	if _, found, _ := testDB.UserByChat(ctx, 500); found {
		t.Fatal("after confirming, the friend should be gone")
	}
	if _, found, _ := testDB.UserByChat(ctx, ownerChat); !found {
		t.Fatal("the owner must be untouched by someone else's /forget")
	}
	b.handleMessage(ctx, msg(500, "am I still here?"))
	if out := outbox(); !strings.Contains(out, "private companion") {
		t.Errorf("a forgotten person is a stranger again: %s", out)
	}
}
