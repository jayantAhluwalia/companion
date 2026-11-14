package telegram

import "testing"

func TestInviteCode(t *testing.T) {
	if code, ok := inviteCode("/start abc123xyz"); !ok || code != "abc123xyz" {
		t.Errorf("a start link carries its code: %q %v", code, ok)
	}
	for _, text := range []string{"/start", "/start   ", "start abc", "hello", "/list abc"} {
		if _, ok := inviteCode(text); ok {
			t.Errorf("%q is not an invite", text)
		}
	}
}

// Every button the bot sends must be listed in callbackIDs with the ids it
// carries, in the right positions. This table is the contract: if a button
// is added or its data reshaped, this test is where that gets decided.
func TestCallbackIDs(t *testing.T) {
	split := func(data string) (string, []string) {
		parts := []string{}
		cur := ""
		for _, r := range data {
			if r == ':' && len(parts) < 3 {
				parts, cur = append(parts, cur), ""
				continue
			}
			cur += string(r)
		}
		parts = append(parts, cur)
		for len(parts) < 4 {
			parts = append(parts, "")
		}
		return parts[0], parts
	}
	cases := []struct {
		data                   string
		thought, nudge, review int64
	}{
		{"ok:12", 12, 0, 0},
		{"wrong:12", 12, 0, 0},
		{"keep:12", 12, 0, 0},
		{"nr:12", 12, 0, 0},
		{"ct:12:a", 12, 0, 0},
		{"cn:12", 12, 0, 0},
		{"cx:12", 12, 0, 0},
		{"lt:12:h", 12, 0, 0},
		{"do_it:12:7", 12, 7, 0},
		{"next_steps:12:7", 12, 7, 0},
		{"later:12:7", 12, 7, 0},
		{"not_interested:12:7", 12, 7, 0},
		{"do_it:12:0", 12, 0, 0},
		{"rv:5:12:k", 12, 0, 5},
		{"ob:c:sl:23:00-07:00", 0, 0, 0},
		{"rt:ct", 0, 0, 0},
		{"fg:yes", 0, 0, 0},
	}
	for _, c := range cases {
		action, parts := split(c.data)
		thought, nudge, review, known := callbackIDs(action, parts)
		if !known || thought != c.thought || nudge != c.nudge || review != c.review {
			t.Errorf("%s: thought=%d nudge=%d review=%d known=%v; want %d %d %d", c.data, thought, nudge, review, known, c.thought, c.nudge, c.review)
		}
	}
	for _, data := range []string{"delete:12", "admin:1", "", "OK:12"} {
		action, parts := split(data)
		if _, _, _, known := callbackIDs(action, parts); known {
			t.Errorf("%q is not a button this bot sends and must be refused", data)
		}
	}
}

func TestIsBlocked(t *testing.T) {
	for _, msg := range []string{
		"telegram sendMessage: Forbidden: bot was blocked by the user",
		"telegram sendMessage: Forbidden: user is deactivated",
		"telegram sendMessage: Bad Request: chat not found",
	} {
		if !IsBlocked(errString(msg)) {
			t.Errorf("%q means the person cannot be reached", msg)
		}
	}
	if IsBlocked(errString("telegram sendMessage: Post: context deadline exceeded")) || IsBlocked(nil) {
		t.Error("a timeout is not a block")
	}
}

type errString string

func (e errString) Error() string { return string(e) }
