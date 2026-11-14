# System design review

14 November 2026. Scope: `services/api` (Go API, Telegram bot, nudge engine) as deployed, one
user. Two passes: the author's, verified against the code, and an independent read-only review
focused on failure modes and multi-user isolation. Findings carry a file reference so they can
be checked. Three defects were fixed during the review (section 2); everything else is open.

## 1. Verdict

The decision core is sound for one user: gates and lanes are plain code with tests, every
decision is logged with its inputs, prompts are versioned, model spend is recorded, and the
engine does not know which channel delivers. The weaknesses are around that core:

- **Durability of a capture.** A thought can be saved and then never interpreted, with no retry.
- **Bookkeeping around sends.** A message can be delivered and not counted, or counted and not delivered.
- **Operations.** No backups, no alerting, no CI, no staging; local runs use the production database.
- **Multi-user.** Not safe today. Several actions do not check who owns the thing they change,
  limits and costs are global, and the engine and the bot each process everyone in one queue.

## 2. Fixed during the review

| Defect | Risk | Fix |
|---|---|---|
| Transport errors included the Telegram request URL, which contains the bot token | A network failure would have written the token to logs, the decision log, `/why` and the `/nudge` reply | `telegram/client.go` drops the URL from such errors; test added. No leak had occurred (database and logs checked). |
| No `.dockerignore` | `.env` was sent to the image builder with the build context | Added. |
| Answering a thought in the weekly review matched its number anywhere in the button data | Thought #3 in review #3 would have cleared every row | Matches the full `rv:<review>:<thought>:` prefix. |

## 3. Open findings that matter with one user

Ranked. "Where" is relative to `services/api/internal`.

| # | Severity | Finding | Where | Fix |
|---|---|---|---|---|
| 1 | High | **A capture can be lost in all but name.** Interpretation runs in a goroutine with no recovery. A deploy, crash, API error or timeout leaves the thought saved but uninterpreted, and nothing retries: it never becomes a candidate and never reaches the review. On timeout the user hears nothing after "Got it". | `telegram/bot.go` (interpret) | A sweep on each tick that re-interprets thoughts with no interpretation, with an attempt cap; send the failure message on a fresh context. |
| 2 | High | **A restart can duplicate a capture.** The Telegram offset is only in memory and is confirmed by the next poll, so updates handled just before a restart are delivered again: a second thought, a second paid interpretation. Button presses replay too. | `telegram/bot.go` (Run) | Store the Telegram message id on the thought with a unique constraint; ignore repeats. |
| 3 | High | **No backups.** The Supabase free tier keeps none. Losing the project loses every thought. | — | Nightly `pg_dump` to storage outside Supabase, and a tested restore. |
| 4 | Medium | **Send and record are not atomic.** If the database write after a successful send fails (a blip, or the context cancelled by shutdown), the nudge stays "scheduled" with no send time: it is not counted in the daily cap, the budget, the cooldown or the deadline limit, so another can follow. The mirror case marks a delivered message as failed. Reviews have the same gap. | `engine/engine.go`, `engine/review.go` | Do post-send bookkeeping on a context that cannot be cancelled, with a short retry; treat recent "scheduled" rows as sent; reconcile at startup. |
| 5 | Medium | **"Ignored" is counted too broadly, and fading is silent and one-way.** A deadline notice or a `/nudge` result that was read but not tapped raises the ignore streak. After two, the thought is review-only; after four it becomes "faded": gone from `/list`, the engine and the review, with no message and no way back. Tapping a button after expiry does not undo it. | `store/nudges.go` (ExpireNudges) | Count ignores only for automatic budgeted nudges; run expiry in one transaction; un-fade on any response; show faded thoughts in `/list` or drop the state in favour of the review. |
| 6 | Medium | **`/review` on demand uses the weekly budget and can make the scheduled review monthly.** Two on-demand reviews read without tapping consume two of five nudges, skip Sunday's review, and trigger the "two untouched reviews" pause. | `engine/review.go` | Record how a review was triggered; leave on-demand ones out of the budget and the pause rule. |
| 7 | Medium | **Nothing tells anyone when it breaks.** The health check returns OK without touching the database, Telegram or the engine. If the token is revoked or the database password rotated, the bot goes quiet and stays quiet. | `http/server.go`, `fly.toml` | A health check that reports the last successful tick and poll; a message to the owner on repeated errors. |
| 8 | Medium | **No staging, and local runs use production.** `.env` on the laptop holds the production database URL and bot token. Local runs have applied migrations to production; a local admin call can send real messages alongside the deployed ticker. | `.env`, `Makefile` | A second database for development, selected by default for `make run`. |
| 9 | Medium | **No CI, and the SQL is untested.** Tests cover the rules well; `store`, `http` and the bot flows have none, and nothing runs on push. Every query was verified only by running it live. | repo | A workflow running `go vet` and `go test`; store tests against a throwaway Postgres. |
| 10 | Medium-low | **The engine sends from a snapshot that can be minutes old.** Thoughts are loaded, then a web lookup (up to 3 min) and the model call (up to 90 s) run, then the nudge is sent without re-reading. `/done 12` typed in that gap is followed by a nudge about #12. | `engine/engine.go` | Re-check the thought's status and the permit just before recording the nudge. |
| 11 | Medium-low | **Messages over Telegram's 4,096 characters fail silently.** An uninterpreted thought is listed with its full text, so one long one breaks every `/list`. `/why` can exceed the limit with several triggers. `/list` silently shows only the newest 50. | `telegram/format.go` | Clip in every formatter; split long messages. |
| 12 | Low-medium | **Web lookups can be wasted or starved.** A lookup that fails is not cached, and thoughts are visited oldest first, so one persistently failing lookup takes every run's single lookup. Thoughts that can no longer be pushed are still looked up. | `engine/hooks.go` | Cache failures briefly; skip lookups for thoughts that would be held anyway. |
| 13 | Low | **The spend guard trusts a hand-kept price table.** A model id not in the table costs $0 in the log, which silently disables the monthly budget. | `ai/pricing.go` | Fail closed on an unknown model. |
| 14 | Low | **The decision log grows without bound.** Every run past the gates stores all thoughts' scores and trigger facts (about 1.5 KB each today, more with more thoughts); only blocked runs are pruned. Missing indexes: `decision_runs(user_id)`, `nudges(thought_id)`, `llm_calls(created_at)`. | `store/nudges.go` | Keep full detail for 30 days, then a summary; add the indexes. |
| 15 | Low | Smaller items: "Keep it as a thought" restores a thought typed `unclear`, which is then never nudged or reviewed. A stale "Later" button on a finished thought reactivates it. Photos, voice notes and edited messages are dropped without a reply. Snooze dates in `/list` are shown in UTC. Today's window can change mid-day when yesterday's nudge expires. The container runs as root. | various | Individually small. |

## 4. Structural improvements

1. **A service layer between clients and the store.** Capture, respond, correct, snooze, review
   and routine are written inside the Telegram package and call the database directly. Moving
   them behind one layer is what makes a second client, an API and ownership checks possible, and
   it is the natural place to enforce "this belongs to this user" once instead of per call site.
2. **Schedule per user instead of ticking everyone.** The engine wakes every 30 minutes and
   evaluates every user in sequence under one lock. Most ticks are blocked by the gates. Give each
   user a `next_check_at` computed from their gates and windows, and have workers claim due users
   (`select … for update skip locked`). That removes the pointless ticks, spreads load, and makes
   the tick time independent of the number of users.
3. **One lock per user, and concurrent handling per chat.** A single mutex serialises every
   evaluation, and one loop handles every Telegram update, so one user's slow lookup delays all.
4. **A lookup cache shared across thoughts.** Web findings are cached per thought. The questions
   are written to be generic ("are any sales running on …"), so caching by question and country
   would cut the largest per-user cost as soon as two people want similar things.
5. **Webhook instead of long polling,** before running more than one machine. Two pollers on one
   token conflict.

## 5. Multi-user

### 5.1 What blocks it today

| # | Severity | Problem |
|---|---|---|
| A | Critical | **Button actions do not check ownership.** Thought and nudge ids are global sequential numbers shown in the chat, and Telegram button data must be treated as attacker-controlled. `not_interested`, `later`, `ok`, `wrong`, `do_it` and the review tap counter act on whatever id they are given. One user could archive, snooze or keep-fresh another's thoughts and corrupt their nudge record. Typed commands (`/done 12`) and some buttons do check. |
| B | High | **Limits, budgets and `/cost` are global.** One heavy user uses up the monthly budget and the daily web lookups for everyone, and `/cost` shows everyone's combined spend. |
| C | High | **Spend per user is unbounded.** Every message starts a paid interpretation with no cap or rate limit. `/nudge`, follow-on passes and deadlines skip the spend guards. A user who blocks the bot still triggers a paid model call every few hours. |
| D | High | **A new user is on Indian time until they set a city, and the city question is skippable.** A friend abroad would be nudged in the middle of the night, and "today" would be wrong for the interpreter. |
| E | Medium | **Day arithmetic breaks on daylight-saving changes.** Dates are counted in 24-hour blocks, so across a spring-forward every date is a day short and a deadline can be reminded on the wrong days. India has no clock changes, so this is invisible today. |
| F | Medium | **One queue for everyone** (see 4.2 and 4.3). Later users in the loop can miss a one-hour lunch window. |
| G | Medium | **One admin token and no notion of roles.** The admin endpoints act on the one configured chat. There is no allow-list beyond a single chat id, and nothing checks that a chat is private, so an allowed group would become one shared user. |
| H | Low-medium | **Web content reaches users.** A page found by a lookup can influence the wording of a nudge and the label on its link. The link itself must come from the search results, but its title is model-written. Each path is per-thought, so nothing crosses users. |

### 5.2 Design

- **Identity.** A `users` row is an account. A separate `channels` table holds the ways to reach
  it (Telegram chat, iOS device, web), so Telegram stops being the definition of a user. Entry is
  by invite code; admin is a role on the account.
- **Isolation, enforced twice.** First in code: the store hands out a handle scoped to one user
  (`store.ForUser(id)`), and every method on it filters by that id, so a missing check becomes
  impossible to write rather than easy to forget. Second in the database: row-level security on
  every user-owned table, keyed on a per-transaction setting, as a backstop.
- **Numbering.** Each user's thoughts are numbered from 1. Internal ids never appear in chat or
  in button data that the handler trusts.
- **Quotas.** Per user: a monthly model budget, daily web lookups, a rate limit on captures and
  on `/nudge`. A global circuit breaker above them. Checked before every model call, including
  interpretation. Notifications turn off after Telegram reports the bot is blocked.
- **Time.** A timezone is required before the first proactive message, asked separately from the
  city. Day differences are computed on calendar dates.
- **Concurrency.** Per-user locks, a small worker pool, and the per-user schedule from 4.2.
- **Privacy.** Thoughts are personal and are stored as plain text that the operator can read, and
  they are sent to the model provider. Other people need to be told that before they start, and
  need `/export` and `/forget` (a hard delete). If this ever goes beyond friends, India's data
  protection law and its consent and deletion duties apply and need proper advice.
- **Cost.** At current settings a heavy user is roughly $6–10 a month, mostly web lookups. The
  shared lookup cache (4.4) is the main lever; per-user budgets are the guard.

### 5.3 Staged plan

| Stage | Outcome | Contents | Rough size |
|---|---|---|---|
| 0. Harden | Safe to rely on alone | Findings 1–9: capture sweep and de-duplication, backups, send bookkeeping, ignore/fade rules, review budget, health and alerts, a development database, CI | 3–4 days |
| 1. Friends | A handful of invited people on Telegram | Invite codes and allow-list; user-scoped store handle on every action; per-user thought numbers; per-user budgets and rate limits; per-user locks and per-chat handling; timezone required; calendar-date arithmetic; `/forget` and `/export`; owner-only `/cost` and admin | 4–5 days |
| 2. Accounts and API | Any client can plug in | The service layer and a user-facing API; `channels` table; the engine delivers through several channels; row-level security | About a week, shared with the iOS phase |
| 3. Scale | Hundreds of users | Per-user scheduling with a work queue; webhook; shared lookup cache; metrics; decision-log retention | As needed |

Stage 0 is worth doing regardless of multi-user. Stage 1 does not require stage 2, but stage 1's
user-scoped store handle is the first half of stage 2's service layer, so the work carries over.

## 6. Not confirmed from the code

- Whether the advisory lock is released by Supabase's session pooler if the process is killed
  mid-tick. If not, every later tick would skip and nudges would stop. Test by killing mid-tick.
  Moving to the transaction pooler would break the lock outright.
- That a custom Telegram client can send arbitrary button data. The ownership finding assumes it
  can, which is the safe assumption.
- Telegram rate limits (`retry_after` is ignored everywhere) with several users.
- Whether two first messages arriving at once can make user creation return no row.
- The model prices in `ai/pricing.go` against the provider's current list.
