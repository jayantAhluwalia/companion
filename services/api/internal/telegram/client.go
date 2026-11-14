// Package telegram is a minimal Bot API client: only the handful of methods
// this product needs, over plain HTTP, so there is no dependency to track.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	token string
	http  *http.Client
	// Dry makes every outgoing call a logged no-op. Development runs use it
	// so a local experiment can never message a real person.
	Dry func(method string, payload any)
}

func NewClient(token string) *Client {
	// Long polling holds a request open for up to 30s, so the client timeout
	// must comfortably exceed that.
	return &Client{token: token, http: &http.Client{Timeout: 50 * time.Second}}
}

// --- wire types (subset of the Bot API) ---

type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

type Message struct {
	MessageID int64  `json:"message_id"`
	From      *User  `json:"from,omitempty"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text,omitempty"`
	// ReplyMarkup is the inline keyboard currently under the message; a
	// callback uses it to remove just the row that was answered.
	ReplyMarkup *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username,omitempty"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	From    User     `json:"from"`
	Message *Message `json:"message,omitempty"`
	Data    string   `json:"data,omitempty"`
}

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type BotCommand struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// --- methods ---

func (c *Client) call(ctx context.Context, method string, payload any, out any) error {
	if c.Dry != nil && method != "getMe" && method != "getUpdates" {
		c.Dry(method, payload)
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	u := fmt.Sprintf("https://api.telegram.org/bot%s/%s", url.PathEscape(c.token), method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("telegram %s: %w", method, withoutURL(err))
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var env struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return fmt.Errorf("telegram %s: bad response (%d): %.200s", method, resp.StatusCode, raw)
	}
	if !env.OK {
		return fmt.Errorf("telegram %s: %s", method, env.Description)
	}
	if out != nil {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}

// withoutURL strips the request URL from a transport error. The bot token is
// part of that URL, and Go prints the whole URL in such errors, so without
// this a network failure would write the token into logs, the database and
// any message that shows an error reason.
func withoutURL(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("%s: %w", ue.Op, ue.Err)
	}
	return err
}

func (c *Client) GetMe(ctx context.Context) (User, error) {
	var u User
	err := c.call(ctx, "getMe", map[string]any{}, &u)
	return u, err
}

// GetUpdates long-polls for up to timeoutSec seconds.
func (c *Client) GetUpdates(ctx context.Context, offset int64, timeoutSec int) ([]Update, error) {
	var updates []Update
	err := c.call(ctx, "getUpdates", map[string]any{
		"offset":          offset,
		"timeout":         timeoutSec,
		"allowed_updates": []string{"message", "callback_query"},
	}, &updates)
	return updates, err
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, text string, keyboard *InlineKeyboardMarkup) (Message, error) {
	// All outgoing text is HTML: callers escape dynamic content with Esc().
	payload := map[string]any{"chat_id": chatID, "text": text, "parse_mode": "HTML",
		"link_preview_options": map[string]any{"is_disabled": true}}
	if keyboard != nil {
		payload["reply_markup"] = keyboard
	}
	var m Message
	err := c.call(ctx, "sendMessage", payload, &m)
	return m, err
}

// AnswerCallbackQuery shows a short toast to the user and stops the button spinner.
func (c *Client) AnswerCallbackQuery(ctx context.Context, id, text string) error {
	return c.call(ctx, "answerCallbackQuery", map[string]any{"callback_query_id": id, "text": text}, nil)
}

// ClearKeyboard removes the inline buttons from a message once one was pressed.
func (c *Client) ClearKeyboard(ctx context.Context, chatID, messageID int64) error {
	return c.call(ctx, "editMessageReplyMarkup", map[string]any{
		"chat_id": chatID, "message_id": messageID,
		"reply_markup": InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{}},
	}, nil)
}

// EditKeyboard swaps the inline buttons under an existing message.
func (c *Client) EditKeyboard(ctx context.Context, chatID, messageID int64, keyboard InlineKeyboardMarkup) error {
	return c.call(ctx, "editMessageReplyMarkup", map[string]any{
		"chat_id": chatID, "message_id": messageID, "reply_markup": keyboard,
	}, nil)
}

// EditText replaces a message's text. With a nil keyboard the buttons are
// removed, so an answered question reads as a record of the answer.
func (c *Client) EditText(ctx context.Context, chatID, messageID int64, text string, keyboard *InlineKeyboardMarkup) error {
	payload := map[string]any{
		"chat_id": chatID, "message_id": messageID, "text": text, "parse_mode": "HTML",
		"link_preview_options": map[string]any{"is_disabled": true},
	}
	if keyboard != nil {
		payload["reply_markup"] = keyboard
	}
	return c.call(ctx, "editMessageText", payload, nil)
}

func (c *Client) SetMyCommands(ctx context.Context, cmds []BotCommand) error {
	return c.call(ctx, "setMyCommands", map[string]any{"commands": cmds}, nil)
}

// Esc makes user- or model-written text safe inside an HTML message.
func Esc(s string) string { return html.EscapeString(s) }
