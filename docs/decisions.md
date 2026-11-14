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

## 2026-11-14 — First two live nudges reviewed

Both were acted on ("Let's do it", then done): the wardrobe light on Fri 19:00 and the beard
oil on Sat 09:00. Reading the decision log behind them showed two defects, fixed the same day:

- **Free time no longer overrides the two-day rest.** "The workday is over" fires every
  evening, so it surfaced a one-day-old idea at the minute work ended. Only weather, web and
  date hooks may interrupt a rest now.
- **Details from a lookup that found nothing timely are no longer given to the nudge
  writer.** The first nudge quoted a ₹375 listing from a lookup the engine itself had rated
  "waiting, confidence 0.5". The writer now gets facts only for fired or mixed signals, and
  `decide_v2` also tells it to stay with what the person said (building, not buying).

Open after the review, for the user to decide: whether a date-critical nudge may exceed the
daily cap of one; whether "ran out of" style thoughts should skip the two-day rest; a buffer
after work ends.

## 2026-11-14 — Lanes, one weekly budget, send windows and a weekly review

**Context.** "One nudge a day" was a single budget for everything: it let a no-reason reminder
block a deadline, and it still allowed a nudge every day whether or not anything had changed.
The user liked a lanes proposal but asked for the research first. The literature review
(`reports/Nudge timing research evidence.md`, notes in `research_notes/`) supported the kinds of
nudge and contradicted two details of the proposal: separate caps per lane added up to more
messages than before, and a "before work" window sat in the slot the evidence rates worst.

**Decision.** As described in the action plan, section 6.9: obligations (deadlines, times the
user picked) outside the budget; opportunities and quick actions inside one weekly budget of
five; one send window a day, learned from this user's own responses, with no early morning;
everything else in a weekly review. The user's own test for a good nudge, "it came when I could
act on it", is the success measure: acted versus ignored, per window.

**Consequences.** Expect fewer nudges than under the daily cap, and silent days. Ideas and
someday plans are no longer pushed on a timer; they wait for a trigger or the review. The
budget of five, the quick share of three, the back-off steps and the prior are judgement calls
to revisit with data. Nothing here has been validated on this user yet beyond unit tests, the
fixture test and dry runs.

## 2026-11-14 — A date trigger's lead time is the interpreter's call

"The luggage arrives on the 16th" would have produced a deadline reminder the evening before
and another on the day. A lead of zero now means only on the day; a lead of one or more earns
the day-before heads-up.

## 2026-11-14 — Design review, and hardening before inviting anyone

A two-pass review (`docs/05-system-review.md`) found the engine sound and the edges weak. The
user wants three or four invited testers, not scale, so the plan is: stage 0 hardening, stage 1
"friends", stage 2 accounts and an API when a second client starts, stage 3 (scale) on standby.

Stage 0 choices worth remembering:

- **Backups are a Go exporter, not pg_dump,** so they run on the laptop and in CI with nothing
  installed, and every backup is restored into a scratch database before it counts.
- **Development uses a schema, not a second project.** `DB_SCHEMA=dev` plus `DRY_SEND=true`
  isolates a local run inside the same free database.
- **At-most-once for nudges.** When it is unclear whether a message went out, it is counted as
  sent. A missed nudge is better than a repeated one.
- **Fading is gone.** A thought never leaves the user's view without their decision.

## 2026-11-14 — Invited testers: a guard at the door, not a rewrite

**Context.** The user wants three or four friends testing, not a product launch.

**Decision.** Ownership is enforced at one chokepoint, the callback handler, by a table that
names which ids each button carries; commands look thoughts up by the sender's own numbering.
The fuller design (a store handle scoped to one user, row-level security) waits for stage 2,
when a second client would otherwise need the same checks written again.

**Also decided.** Invites are single-use links from the owner. Each person has their own budget
and a required city (for the timezone). People are told at the door that the owner can read
their thoughts, and can export or delete everything themselves. The owner's view of other people
is counts and cost, never content.

**Costs.** Worst case is about $4 per friend per month plus the owner's $8, with a service-wide
stop at $18, set under the $20 cap in the Anthropic console. Raising the number of testers past
three means raising that cap or lowering the per-person budget.
