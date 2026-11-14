package telegram

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"companion/api/internal/engine"
	"companion/api/internal/signals"
	"companion/api/internal/store"
)

// Onboarding asks five questions once: sleep, work hours, nudges during
// work, days off, city. Every one is skippable and has a default; the same
// questions are reachable one at a time from /routine.
//
// Callback data is "ob:<mode>:<question>:<value>", where mode "c" chains to
// the next question and "s" (single) returns to the routine summary.
var questionOrder = []string{"sl", "wk", "dw", "of", "ct"}

func btn(text, data string) InlineKeyboardButton {
	return InlineKeyboardButton{Text: text, CallbackData: data}
}

func (b *Bot) start(ctx context.Context, user store.User) {
	chat := user.TelegramChatID
	b.reply(ctx, chat, "👋  <b>Hi. I'm your companion.</b>\n\nSend me anything on your mind: something you want to do, try, learn or look into. I'll remember it and bring it up when the moment is right: a dry weekend for that ride, a sale on the thing you wanted.\n\nNo need to organise anything. Just type it.")
	prefs, err := b.db.GetPrefs(ctx, user.ID)
	if err == nil && prefs.OnboardedAt != nil {
		b.reply(ctx, chat, "<i>/list shows what I'm holding · /routine changes when I may nudge you</i>")
		return
	}
	b.reply(ctx, chat, "First, five quick questions so I pick good moments. Skip any of them.")
	b.ask(ctx, user, questionOrder[0], true)
}

// ask sends one question. chain decides what happens after the answer.
func (b *Bot) ask(ctx context.Context, user store.User, q string, chain bool) {
	mode := "s"
	if chain {
		mode = "c"
	}
	d := func(value string) string { return fmt.Sprintf("ob:%s:%s:%s", mode, q, value) }
	var text string
	var rows [][]InlineKeyboardButton
	switch q {
	case "sl":
		text = "🌙  <b>When do you usually sleep?</b>\nI will never message you in this window."
		rows = [][]InlineKeyboardButton{
			{btn("22:00 – 06:00", d("22:00-06:00")), btn("23:00 – 07:00", d("23:00-07:00"))},
			{btn("00:00 – 08:00", d("00:00-08:00")), btn("01:00 – 09:00", d("01:00-09:00"))},
			{btn("Other…", d("other")), btn("Skip", d("skip"))},
		}
	case "wk":
		text = "💼  <b>Your usual working hours?</b>"
		rows = [][]InlineKeyboardButton{
			{btn("09:00 – 18:00", d("09:00-18:00")), btn("10:00 – 19:00", d("10:00-19:00"))},
			{btn("11:00 – 20:00", d("11:00-20:00")), btn("No fixed hours", d("none"))},
			{btn("Other…", d("other")), btn("Skip", d("skip"))},
		}
	case "dw":
		text = "🔔  <b>Is it okay to nudge you during work?</b>"
		rows = [][]InlineKeyboardButton{
			{btn("Yes", d("yes")), btn("Only at lunch", d("lunch"))},
			{btn("No, wait until after", d("no"))},
		}
	case "of":
		text = "🏖  <b>Which days are you usually off?</b>"
		rows = [][]InlineKeyboardButton{
			{btn("Sat & Sun", d("6,7")), btn("Only Sunday", d("7"))},
			{btn("Fri & Sat", d("5,6")), btn("Sun & Mon", d("7,1"))},
			{btn("Skip", d("skip"))},
		}
	case "rv":
		text = "🗓  <b>When should the weekly review arrive?</b>\nOne message with everything I'm holding that has gone quiet, so you can keep or drop each."
		rows = [][]InlineKeyboardButton{
			{btn("Sunday evening", d("7@19:00")), btn("Saturday morning", d("6@10:30"))},
			{btn("Friday evening", d("5@19:30")), btn("Turn it off", d("0@"))},
		}
	case "ct":
		text = "📍  <b>Which city do you live in?</b>\nType its name. I use it for the weather on your days off and for local deals."
		rows = [][]InlineKeyboardButton{{btn("Skip", d("skip"))}}
		_ = b.db.SetPending(ctx, user.ID, &store.Pending{Kind: "city", Chain: chain, ExpiresAt: time.Now().Add(pendingTTL)})
	default:
		return
	}
	if _, err := b.client.SendMessage(ctx, user.TelegramChatID, text, &InlineKeyboardMarkup{InlineKeyboard: rows}); err != nil {
		b.log.Error("ask", "err", err, "question", q)
	}
}

// onboardingAnswer handles a tapped answer and returns the toast text.
func (b *Bot) onboardingAnswer(ctx context.Context, user store.User, chain bool, q, value string, msgID int64) string {
	chat := user.TelegramChatID
	record := "" // what the question message is rewritten to
	var err error
	switch {
	case value == "skip":
		if q == "ct" {
			_ = b.db.SetPending(ctx, user.ID, nil)
		}
		record = questionLabel(q) + ": <i>skipped</i>"
	case value == "other":
		kind := map[string]string{"sl": "sleep", "wk": "work"}[q]
		_ = b.client.ClearKeyboard(ctx, chat, msgID)
		_ = b.db.SetPending(ctx, user.ID, &store.Pending{Kind: kind, Chain: chain, ExpiresAt: time.Now().Add(pendingTTL)})
		b.reply(ctx, chat, "⌨️  Type it like <code>23:30-07:00</code>")
		return ""
	case q == "sl" || (q == "wk" && value != "none"):
		start, end, ok := parseRange(value)
		if !ok {
			return "I couldn't read that."
		}
		if q == "sl" {
			err = b.db.SetSleep(ctx, user.ID, start, end)
		} else {
			err = b.db.SetWork(ctx, user.ID, start, end)
		}
		record = fmt.Sprintf("%s: <b>%s – %s</b>", questionLabel(q), start, end)
	case q == "wk":
		err = b.db.SetWork(ctx, user.ID, store.Unset, store.Unset)
		record = questionLabel(q) + ": <b>no fixed hours</b>"
	case q == "rv":
		dayText, timeText, _ := strings.Cut(value, "@")
		day, _ := strconv.Atoi(dayText)
		at := store.Unset
		if h, m, ok := strings.Cut(timeText, ":"); ok {
			hh, _ := strconv.Atoi(h)
			mm, _ := strconv.Atoi(m)
			at = store.Clock(hh*60 + mm)
		}
		err = b.db.SetReview(ctx, user.ID, day, at)
		record = questionLabel(q) + ": <b>off</b>"
		if day != 0 {
			record = fmt.Sprintf("%s: <b>%s %s</b>", questionLabel(q), engine.DayList([]int{day}), at)
		}
	case q == "dw":
		err = b.db.SetNudgeDuringWork(ctx, user.ID, value)
		record = questionLabel(q) + ": <b>" + map[string]string{"yes": "yes", "lunch": "only at lunch", "no": "no, after work"}[value] + "</b>"
	case q == "of":
		var days []int
		for _, part := range strings.Split(value, ",") {
			if n, convErr := strconv.Atoi(part); convErr == nil && n >= 1 && n <= 7 {
				days = append(days, n)
			}
		}
		err = b.db.SetDaysOff(ctx, user.ID, days)
		record = questionLabel(q) + ": <b>" + engine.DayList(days) + "</b>"
	}
	if err != nil {
		b.log.Error("save routine", "err", err, "question", q)
		return "Couldn't save that."
	}
	_ = b.client.EditText(ctx, chat, msgID, record, nil)
	b.next(ctx, user, q, chain, value)
	return "Saved."
}

// next moves a chained onboarding forward, or shows the summary.
func (b *Bot) next(ctx context.Context, user store.User, answered string, chain bool, value string) {
	if !chain {
		// Changing a setting from /routine also counts as being set up, so
		// /start never re-asks questions that were answered one by one.
		_ = b.db.MarkOnboarded(ctx, user.ID)
		b.routine(ctx, user)
		return
	}
	for i, q := range questionOrder {
		if q != answered {
			continue
		}
		if i+1 == len(questionOrder) {
			break
		}
		following := questionOrder[i+1]
		if answered == "wk" && value == "none" {
			following = "of" // nothing to ask about nudging during work
		}
		b.ask(ctx, user, following, true)
		return
	}
	_ = b.db.MarkOnboarded(ctx, user.ID)
	b.reply(ctx, user.TelegramChatID, "✨  <b>All set.</b>")
	b.routine(ctx, user)
	b.reply(ctx, user.TelegramChatID, "Now just send me whatever is on your mind.")
}

// onboardingTyped handles answers that are typed instead of tapped.
func (b *Bot) onboardingTyped(ctx context.Context, user store.User, p store.Pending, text string) {
	chat := user.TelegramChatID
	retry := func(msg string) {
		p.ExpiresAt = time.Now().Add(pendingTTL)
		_ = b.db.SetPending(ctx, user.ID, &p)
		b.reply(ctx, chat, msg+"\n<i>/cancel to skip it.</i>")
	}
	switch p.Kind {
	case "sleep", "work":
		start, end, ok := parseRange(text)
		if !ok {
			retry("🤔  I couldn't read that. Try <code>23:30-07:00</code>.")
			return
		}
		q, save := "sl", b.db.SetSleep
		if p.Kind == "work" {
			q, save = "wk", b.db.SetWork
		}
		if err := save(ctx, user.ID, start, end); err != nil {
			b.log.Error("save routine", "err", err)
			b.reply(ctx, chat, "Couldn't save that.")
			return
		}
		b.reply(ctx, chat, fmt.Sprintf("%s: <b>%s – %s</b>", questionLabel(q), start, end))
		b.next(ctx, user, q, p.Chain, text)

	case "city":
		gctx, cancel := context.WithTimeout(ctx, 15*time.Second)
		place, err := b.weather.Geocode(gctx, text)
		cancel()
		if errors.Is(err, signals.ErrNoSuchPlace) {
			retry("🤔  I couldn't place “" + Esc(clip(text, 40)) + "”. Try just the town or city name.")
			return
		}
		if err != nil {
			b.log.Warn("geocode", "err", err)
			retry("The place lookup is not answering. Try again in a moment.")
			return
		}
		label := place.Label()
		if err := b.db.SetHome(ctx, user.ID, label, place.CountryCode, place.Lat, place.Lon, place.Timezone); err != nil {
			b.log.Error("save home", "err", err)
			b.reply(ctx, chat, "Couldn't save that.")
			return
		}
		b.reply(ctx, chat, fmt.Sprintf("📍  City: <b>%s</b>", Esc(label)))
		b.next(ctx, user, "ct", p.Chain, text)
	}
}

func questionLabel(q string) string {
	return map[string]string{
		"sl": "🌙  Sleep", "wk": "💼  Work", "dw": "🔔  Nudges during work", "of": "🏖  Days off", "ct": "📍  City", "rv": "🗓  Weekly review",
	}[q]
}

// routine shows the current settings with a button to change each.
func (b *Bot) routine(ctx context.Context, user store.User) {
	p, err := b.db.GetPrefs(ctx, user.ID)
	if err != nil {
		b.log.Error("prefs", "err", err)
		b.reply(ctx, user.TelegramChatID, "Couldn't load your routine just now.")
		return
	}
	kb := &InlineKeyboardMarkup{InlineKeyboard: [][]InlineKeyboardButton{
		{btn("🌙 Sleep", "rt:sl"), btn("💼 Work", "rt:wk"), btn("🔔 At work", "rt:dw")},
		{btn("🏖 Days off", "rt:of"), btn("📍 City", "rt:ct"), btn("🗓 Review", "rt:rv")},
	}}
	if _, err := b.client.SendMessage(ctx, user.TelegramChatID, formatRoutine(p), kb); err != nil {
		b.log.Error("send routine", "err", err)
	}
}

var clockRe = regexp.MustCompile(`(?i)(\d{1,2})(?:[:.](\d{2}))?\s*(am|pm)?`)

// parseRange reads "23:30-07:00", "11pm to 7am" or "9 - 18".
func parseRange(s string) (start, end store.Clock, ok bool) {
	m := clockRe.FindAllStringSubmatch(s, -1)
	if len(m) != 2 {
		return 0, 0, false
	}
	clocks := make([]store.Clock, 2)
	for i, g := range m {
		h, _ := strconv.Atoi(g[1])
		min := 0
		if g[2] != "" {
			min, _ = strconv.Atoi(g[2])
		}
		switch strings.ToLower(g[3]) {
		case "pm":
			if h < 12 {
				h += 12
			}
		case "am":
			if h == 12 {
				h = 0
			}
		}
		if h > 24 || min > 59 {
			return 0, 0, false
		}
		clocks[i] = store.Clock((h%24)*60 + min)
	}
	if clocks[0] == clocks[1] {
		return 0, 0, false
	}
	return clocks[0], clocks[1], true
}
