package telegram

import (
	"context"
	"strings"
	"testing"
	"time"
)

// A network failure must never carry the bot token into an error message:
// errors are logged, stored and sometimes shown to the user.
func TestTransportErrorsDoNotLeakTheToken(t *testing.T) {
	const token = "123456:SECRET-TOKEN-VALUE"
	c := NewClient(token)
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(time.Millisecond) // the context has expired, so the request fails before any network use
	_, err := c.SendMessage(ctx, 1, "hi", nil)
	if err == nil {
		t.Fatal("expected a transport error")
	}
	if strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), "api.telegram.org") {
		t.Fatalf("the error exposes the request URL: %v", err)
	}
	if !strings.Contains(err.Error(), "sendMessage") {
		t.Errorf("the error should still say which call failed: %v", err)
	}
}
