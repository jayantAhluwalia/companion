package telegram

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"companion/api/internal/store"
)

// Access. The bot serves its owner and the people the owner invites, each
// in a private chat. Everyone else gets one polite refusal and nothing is
// stored about them.

// inviteCode extracts the code from "/start <code>", which is what Telegram
// sends when someone opens a t.me/<bot>?start=<code> link.
func inviteCode(text string) (string, bool) {
	cmd, arg, _ := strings.Cut(strings.TrimSpace(text), " ")
	arg = strings.TrimSpace(arg)
	if cmd != "/start" || arg == "" || len(arg) > 64 {
		return "", false
	}
	return arg, true
}

// resolveUser returns the user behind a chat, letting the owner in by
// configuration and a newcomer in by invite. ok is false when the message
// has been fully dealt with (refused, or the newcomer was welcomed).
func (b *Bot) resolveUser(ctx context.Context, m *Message) (store.User, bool) {
	chat := m.Chat.ID
	if m.Chat.Type != "private" {
		return store.User{}, false // never in groups: a group would become one shared "person"
	}
	name := ""
	if m.From != nil {
		name = m.From.FirstName
	}
	user, found, err := b.db.UserByChat(ctx, chat)
	if err != nil {
		b.log.Error("find user", "err", err)
		b.reply(ctx, chat, "Something went wrong on my side. Try again in a moment.")
		return user, false
	}
	if found {
		return user, true
	}
	switch {
	case b.allowedChat == 0:
		// First-time setup: no owner configured yet.
		b.reply(ctx, chat, fmt.Sprintf("This bot is private. Chat id: %d\nSet TELEGRAM_CHAT_ID to this value on the server to enable it.", chat))
	case chat == b.allowedChat:
		owner, err := b.db.EnsureOwner(ctx, chat, name)
		if err != nil {
			b.log.Error("ensure owner", "err", err)
			return owner, false
		}
		return owner, true
	default:
		code, ok := inviteCode(m.Text)
		if !ok {
			b.reply(ctx, chat, "🔒  This is a private companion. You need an invite link from its owner to use it.")
			return user, false
		}
		member, err := b.db.RedeemInvite(ctx, code, chat, name)
		if errors.Is(err, store.ErrBadInvite) {
			b.reply(ctx, chat, "🔒  That invite link has been used or has expired. Ask for a fresh one.")
			return user, false
		}
		if err != nil {
			b.log.Error("redeem invite", "err", err)
			b.reply(ctx, chat, "Something went wrong letting you in. Try the link again in a moment.")
			return user, false
		}
		b.log.Info("invite redeemed", "user", member.ID)
		b.Alert("joined:"+strconv.FormatInt(member.ID, 10), fmt.Sprintf("%s joined with an invite link.", fallback(name, "Someone")))
		b.welcome(ctx, member)
	}
	return user, false
}

func fallback(s, alt string) string {
	if strings.TrimSpace(s) == "" {
		return alt
	}
	return s
}

// welcome greets an invited person. It says plainly where their thoughts go
// before they send any, because that is theirs to decide.
func (b *Bot) welcome(ctx context.Context, user store.User) {
	chat := user.TelegramChatID
	b.reply(ctx, chat, "👋  <b>Hi. I'm a companion for the things you mean to do.</b>\n\nSend me anything on your mind: something to do, try, learn or look into. I'll remember it and bring it up when the moment is right: a dry weekend for that ride, a sale on the thing you wanted.")
	b.reply(ctx, chat, "🔐  <b>Before you start, what happens to what you send</b>\n\n•  This is a private test run by the person who invited you. Your thoughts are stored on their server, and they are able to read them.\n•  Each thought is sent to Anthropic's Claude to be understood. Some are turned into a general web search (\"are any sales on running shoes?\") with nothing personal in it.\n•  /export sends you everything I hold about you. /forget deletes all of it, for good.\n\n<i>If that isn't for you, just don't send anything, or send /forget.</i>")
	b.reply(ctx, chat, "First, five quick questions so I pick good moments.")
	b.ask(ctx, user, questionOrder[0], true)
}

// callbackIDs says which ids a button's data carries, by action. Every
// action that names a thought, nudge or review must be listed here: an
// unlisted action is refused, so a new button cannot be added without
// deciding what it is allowed to touch.
func callbackIDs(action string, parts []string) (thought, nudge, review int64, known bool) {
	num := func(i int) int64 {
		if i >= len(parts) {
			return 0
		}
		n, _ := strconv.ParseInt(parts[i], 10, 64)
		return n
	}
	switch action {
	case "ok", "wrong", "keep", "nr", "cn", "cx", "ct", "lt":
		return num(1), 0, 0, true
	case "do_it", "next_steps", "later", "not_interested":
		return num(1), num(2), 0, true
	case "rv":
		return num(2), 0, num(1), true
	case "ob", "rt", "fg":
		return 0, 0, 0, true // routine and account buttons carry no ids
	}
	return 0, 0, 0, false
}

// authorize is the single place a button press is checked against its
// sender. Button data is whatever the sender's client chose to send, so an
// id in it proves nothing; each one is looked up against the person
// pressing. It returns "" when the action may proceed.
func (b *Bot) authorize(ctx context.Context, user store.User, action string, parts []string) string {
	thought, nudge, review, known := callbackIDs(action, parts)
	if !known {
		return "I don't know that button."
	}
	const no = "That isn't yours to change."
	if action != "ob" && action != "rt" && action != "fg" && thought == 0 {
		return no // every remaining action acts on a thought and must name one
	}
	if thought != 0 {
		if ok, err := b.db.OwnsThought(ctx, user.ID, thought); err != nil || !ok {
			return no
		}
	}
	if nudge != 0 {
		if ok, err := b.db.OwnsNudge(ctx, user.ID, nudge, thought); err != nil || !ok {
			return no
		}
	}
	if review != 0 {
		if ok, err := b.db.OwnsReview(ctx, user.ID, review); err != nil || !ok {
			return no
		}
	}
	return ""
}

// overBudget reports whether no more paid work may be done for this person
// right now, and says why in words fit for the chat.
func (b *Bot) overBudget(ctx context.Context, user store.User) (bool, string) {
	prefs, err := b.db.GetPrefs(ctx, user.ID)
	if err != nil {
		return false, ""
	}
	if cost, _, err := b.db.UserMonthlyCostUSD(ctx, user.ID); err == nil && cost >= prefs.MonthlyBudgetUSD {
		return true, "this month's AI budget for you is used up"
	}
	if cost, _, err := b.db.MonthlyCostUSD(ctx); err == nil && cost >= b.globalBudget {
		return true, "this month's AI budget for the whole service is used up"
	}
	return false, ""
}

const nudgesOnRequestPerDay = 5

// --- commands about access and one's own data ---

func (b *Bot) invite(ctx context.Context, user store.User) {
	chat := user.TelegramChatID
	if !user.IsOwner() {
		b.reply(ctx, chat, "Only the owner can invite people.")
		return
	}
	code, err := b.db.CreateInvite(ctx, user.ID)
	if err != nil {
		b.log.Error("create invite", "err", err)
		b.reply(ctx, chat, "Couldn't create an invite just now.")
		return
	}
	link := code
	if b.username != "" {
		link = fmt.Sprintf("https://t.me/%s?start=%s", b.username, code)
	}
	b.reply(ctx, chat, "🎟  <b>One invite, for one person</b>\n\n<code>"+Esc(link)+"</code>\n\n<i>Good for seven days and a single use. They will be told that you can read what they send, and that /forget deletes it.</i>")
}

func (b *Bot) users(ctx context.Context, user store.User) {
	chat := user.TelegramChatID
	if !user.IsOwner() {
		b.reply(ctx, chat, "🤷  I don't know that one. Just send me a thought, or /list.")
		return
	}
	list, err := b.db.UserSummaries(ctx)
	if err != nil {
		b.reply(ctx, chat, "Couldn't load the list.")
		return
	}
	total, _, _ := b.db.MonthlyCostUSD(ctx)
	var sb strings.Builder
	fmt.Fprintf(&sb, "👥  <b>People</b>  <i>(%d)</i>\n", len(list))
	for _, u := range list {
		seen := "never"
		if u.LastSeen != nil {
			seen = u.LastSeen.In(userNow(user).Location()).Format("2 Jan")
		}
		role := ""
		if u.IsOwner() {
			role = "  <i>owner</i>"
		}
		fmt.Fprintf(&sb, "\n<b>%s</b>%s\n      %d thoughts · $%.2f this month · last thought %s\n",
			Esc(fallback(u.DisplayName, "Unnamed")), role, u.Thoughts, u.MonthCost, seen)
	}
	fmt.Fprintf(&sb, "\n<i>All together this month: $%.2f of $%.2f. /invite adds someone.</i>", total, b.globalBudget)
	b.reply(ctx, chat, sb.String())
}

func (b *Bot) cost(ctx context.Context, user store.User) {
	chat := user.TelegramChatID
	prefs, err := b.db.GetPrefs(ctx, user.ID)
	cost, n, err2 := b.db.UserMonthlyCostUSD(ctx, user.ID)
	if err != nil || err2 != nil {
		b.reply(ctx, chat, "Couldn't read the cost log.")
		return
	}
	text := fmt.Sprintf("💸  <b>This month</b>\n%d AI calls for you · about $%.2f of your $%.2f", n, cost, prefs.MonthlyBudgetUSD)
	if user.IsOwner() {
		if total, calls, err := b.db.MonthlyCostUSD(ctx); err == nil {
			text += fmt.Sprintf("\n<i>Everyone together: %d calls · $%.2f of $%.2f</i>", calls, total, b.globalBudget)
		}
	}
	b.reply(ctx, chat, text)
}

func (b *Bot) forgetAsk(ctx context.Context, user store.User) {
	kb := &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{{
		btn("🗑 Yes, delete everything", "fg:yes"), btn("Keep it", "fg:no"),
	}}}
	text := "⚠️  <b>Delete everything I hold about you?</b>\n\nEvery thought, every reading of it, your routine and your nudge history. This cannot be undone.\n\n<i>/export first if you want a copy.</i>"
	if _, err := b.client.SendMessage(ctx, user.TelegramChatID, text, kb); err != nil {
		b.log.Error("send forget", "err", err)
	}
}

func (b *Bot) forgetAnswer(ctx context.Context, user store.User, answer string, msgID int64) string {
	chat := user.TelegramChatID
	if answer != "yes" {
		_ = b.client.EditText(ctx, chat, msgID, "👍  Nothing was deleted.", nil)
		return "Kept."
	}
	if err := b.db.ForgetUser(ctx, user.ID); err != nil {
		b.log.Error("forget user", "err", err, "user", user.ID)
		return "Couldn't delete just now. Try again."
	}
	b.mu.Lock()
	if t, ok := b.followOn[user.ID]; ok {
		t.Stop()
		delete(b.followOn, user.ID)
	}
	b.mu.Unlock()
	b.log.Info("user forgotten", "user", user.ID)
	_ = b.client.EditText(ctx, chat, msgID, "🫥  <b>Everything is deleted.</b>\n\nI no longer know who you are. If you were invited, you would need a new invite to come back.", nil)
	return "Deleted."
}

func (b *Bot) export(ctx context.Context, user store.User) {
	chat := user.TelegramChatID
	data, err := b.db.ExportUser(ctx, user.ID)
	if err != nil {
		b.log.Error("export", "err", err)
		b.reply(ctx, chat, "Couldn't put the export together just now.")
		return
	}
	name := fmt.Sprintf("companion-export-%s.json", time.Now().Format("2006-01-02"))
	if err := b.client.SendDocument(ctx, chat, name, data, "Everything I hold about you."); err != nil {
		b.log.Error("send export", "err", err)
		b.reply(ctx, chat, "Couldn't send the file just now.")
	}
}
