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
