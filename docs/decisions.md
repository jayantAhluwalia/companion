# Decision log

## 2026-11-10 — Telegram bot is the first client, iOS app parked

**Context.** The Apple Developer Program costs about INR 8,700/year. For a personal MVP it buys
one thing the product needs, remote push, plus App Store distribution that only matters if
the product is later shared. Expo Go cannot receive remote push, so the free alternative was
weekly Xcode rebuilds, which is a bad loop for a side project.

**Decision.** Deliver capture and nudges through a private Telegram bot. The Go API, Claude
integration, Fly deployment, decision engine and data model are unchanged; only the delivery
layer differs. The Expo app in `apps/mobile` stays on disk but is not built.

**What changes in the plan.**
- `device_tokens` table is replaced by `users.telegram_chat_id`.
- Nudge delivery is `sendMessage` with an inline keyboard; actions arrive as callback queries.
- The per-thought conversation (Phase 4) becomes a reply thread in the chat, much cheaper.
- An "overview" is a `/list` command rather than a screen.
- Long polling instead of a webhook: works identically locally and on Fly, no public URL to secure.

**Target is unchanged.** The intended end state is still the iOS app with interactive
notifications (category actions, rich content). Telegram is the interim channel while
Phases 1–3 build and tune the engine, which is not blocked by the app.

**Design rule to keep the swap cheap.** Delivery is a `Channel` interface in the Go code
(`Send(nudge) -> delivery id`, `HandleAction(action, nudgeID)`), with Telegram as the first
implementation and APNs via Expo as the second. Action identifiers (`do_it`, `next_steps`,
`later`, `not_interested`) are identical across channels so feedback collected on Telegram
trains the same preference model the app will use. `nudges.channel` records which one sent it.

**Revisit when.** When the engine feels right after a few weeks of use, buy the Apple
membership and resume `apps/mobile` from its Phase 0 runbook; the app work is Phase 5.

## 2026-11-11 — Ask for routine once, as priors, not rules

**Context.** The engine needed quiet hours and a sense of good moments. iOS cannot reveal what
other apps are in use, so "nudge while scrolling" is a Phase 5 experiment via Screen Time, not
a Phase 2 input. The cheap, reliable context is the user's own routine.

**Decision.** `/start` asks four button questions (sleep window, working hours, nudges during
work, days off), each skippable with a default, editable via `/routine`. Sleep is a hard gate.
Work hours and days off adjust the opportunity score. From Phase 3, observed response times
override stated hours. Never ask hour-by-hour detail.

## 2026-11-12 — Postgres on Supabase Mumbai, Fly in Singapore, hand-written queries

**Decision.** Supabase free tier in ap-south-1 through the session pooler (the free tier's
direct connection is IPv6-only). Fly's Mumbai region is deprecated for new machines, so the
API runs in `sin`, about 60 ms from the database. Queries are hand-written against pgx
instead of generated with sqlc: fifteen queries did not justify a code-generation step.
Migrations are embedded goose SQL files applied at startup, so every deploy migrates.

## 2026-11-13 — Nudge on the right moment: hooks, signals, and timeliness first

**Context.** The engine as planned ranked thoughts by age and type. The user's bar is higher:
bring up the ride on Thursday because the weekend is dry; bring up the clothes because a sale
starts. That needs facts from outside the app.

**Decision.** The interpreter attaches up to three hooks to a thought (`weather`, `web`,
`date`, `free_time`). The engine checks them: Open-Meteo for weather (free, coordinates only),
Claude with server-side web search for sales, releases and events. Timeliness becomes the
heaviest score component, and a hook that has not fired scores below having no hook, so the
engine waits. The model writes the nudge only from facts the backend fetched, and web sources
are kept only if they appear in the actual search results.

**Consequences.** A web lookup costs about $0.10–0.17, so they are rationed (two a day, cached
until the suggested recheck date, stopped at 80% of a $15 monthly budget). Interpretation
roughly tripled in cost to about $0.016 because the prompt and output grew. Expect $6–10 a
month at 10–20 active thoughts.

## 2026-11-13 — Routine is a gate, not a score

**Decision.** "Don't nudge me at work" and "only at lunch" block the tick outright. Days off
still only adjust which thought is preferred. Reason: as a score input it moved the total by
about 0.05 and would not have prevented a nudge; and with one nudge a day, a badly timed one
wastes the day. This supersedes the "work hours adjust the opportunity score" line of 11 Nov.

## 2026-11-13 — A re-read never overwrites a reading the user accepted

**Context.** The first hook backfill re-interpreted four thoughts and stored the whole new
reading. One confirmed "experience" came back as an "idea". Caught the same hour, repaired by
migration 00003.

**Decision.** Backfills take only the new fields (hooks, timeframe) and keep type, title and
steps. The same applies to any future prompt upgrade.

## 2026-11-13 — Not every message is a thought

**Context.** Every non-command message was stored and interpreted, so "Okay thats fine"
became thought #5.

**Decision.** Two layers. A small word list catches plain acknowledgments, greetings and
emoji-only messages before anything is saved: free and instant. Everything else goes to the
interpreter (`interpret_v3`), which now also returns `not_a_thought` for talk aimed at the bot;
those are archived with a "Keep it as a thought" button. The list is kept small on purpose and
the prompt says "when in doubt it is a thought", because dropping a real thought is the worse
error. The fixture test fails on any thought-or-chatter mistake.
