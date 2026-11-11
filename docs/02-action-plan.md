# Personal AI Companion — Review and Implementation Action Plan

**Based on:** `personal_ai_companion_product_discovery.md` v0.2 (10 Nov 2026)
**Audience:** founder / solo developer, personal use first
**Date:** 10 November 2026


> **Update 10 Nov 2026:** the first client is a private Telegram bot, not the iOS app. See `decisions.md`. Read "push notification" as "Telegram message with inline buttons" and `device_tokens` as `users.telegram_chat_id` throughout. Phases and exit criteria are unchanged.

---

## 1. Verdict

The discovery document is unusually clear for a v0.2. The product thesis is sharp (unfinished
intentions, not tasks), the "what it is not" list protects scope, and the hybrid
decision engine (deterministic policy + LLM judgement) is the correct architecture.

It is ready to build after fixing five things:

1. **Missing prerequisites.** Remote push on iOS requires the paid Apple Developer Program
   (USD 99/yr) and an EAS development build (Expo Go dropped remote push in SDK 53). The Go
   worker needs a host that runs 24/7. None of this is in the doc, and all of it blocks
   Iteration 2.
2. **No conversation surface.** Notification actions like "Let's do it" and promises like
   "I can help with the resale price" imply a chat with the agent, but Section 7 defines no
   screen for it. Without it, "progress over reminders" is only a slogan.
3. **Scoring and learning are under-specified.** The formula names five variables but gives
   no data source for three of them (I, O, F). "Simple preference scoring" has no formula.
   The plan below defines all of them concretely so they can be built and tuned.
4. **Nothing is observable.** The doc says weights will be "tuned through personal use" but
   there is no decision log or debug view to tune against. A `decision_runs` table and a
   debug screen are the cheapest, highest-value additions in this plan.
5. **Timeline assumes full-time work.** 3–5 weeks is fair for ~25 focused days. Part-time,
   expect 10–13 weekends. Re-ordered phases below put the risky integrations first.

Everything else in the doc should be kept as written.

---

## 2. What is strong — keep as-is

- Product principles 1–7 and the "what the product is not" list.
- Raw thought always preserved; AI assumptions distinguishable from user facts.
- "Doing nothing must be a valid output" of the LLM evaluation.
- Backend owns limits, quiet hours, intervals, dedupe, delivery. LLM owns content.
- Ignored notification is ambiguous, not negative.
- Four-type classification (actionable / experience / idea / development).
- Section 10.4 "do not overengineer": one API, one worker, one Postgres, one LLM.
- Success metric: intentions converted to action, not opens or notifications.

---

## 3. Gaps and improvements, ranked by impact

### 3.1 Add a per-thought conversation (product gap)
Every nudge action except "Later" and "Not interested" needs somewhere to land. Add a
**thought thread**: a chat scoped to one thought, where the agent has the interpretation,
history and the user's notes as context. This is also where "agent execution" (draft the
listing, research prices) lives later. Keep it out of the first two phases, but design the
data model for it now (`thought_messages` table).

### 3.2 Make every decision explainable (engineering gap)
Add `decision_runs` (one row per evaluator tick, with scores for every candidate and the LLM's
verdict) and `llm_calls` (one row per API call with prompt version, tokens, cost, latency).
Add a Debug tab in the app that lists the last 20 runs. The nudge feed's "why it was brought
up" (Section 7.3) reads straight from these.

### 3.3 Version interpretations separately from thoughts (data model gap)
The doc puts `interpreted_title` etc. on the thought row and separately says "preserve the
history of AI interpretations and user corrections". Those conflict. Use a
`thought_interpretations` table with `version` and `source` (`ai` | `user`). The thought row
caches the current version id.

### 3.4 Specify the scoring inputs (design gap)
See Section 6.4 below. Drop "opportunity" (O) to day-of-week only for MVP; it has no other
data source until integrations exist.

### 3.5 Specify preference learning (design gap)
Per-style helpfulness ratio with Laplace smoothing and 20% exploration. See Section 6.5.
It is ten lines of code and enough for one user.

### 3.6 Voice capture: use the keyboard (scope cut)
The iOS keyboard dictation button gives speech-to-text in any `TextInput` for zero work.
Mark custom voice capture as "later". This removes a whole workstream from the MVP.

### 3.7 Drop the clickable prototype (process cut)
Section 22 step 2 proposes a Figma-style prototype before building. For a single user who is
also the builder, the real capture screen is faster to make and more informative to test.

### 3.8 Add a Phase 0 spike (risk)
Before any feature work, prove the three riskiest integrations end to end: a push arriving
on your physical iPhone, triggered by a Go handler on the host, after a structured-output
LLM call. One to three days. If any fail, the whole plan changes, so find out first.

### 3.9 Timezone, backups, auth (small but forgotten)
- Quiet hours need `users.timezone`. Store it, send it from the app on launch.
- Thoughts are private. Supabase free tier has no point-in-time recovery; schedule a weekly
  `pg_dump` to a private location.
- One static bearer token (Expo SecureStore on device, Fly secret on server) is sufficient
  auth for a single user and takes an hour. Keep `user_id` on every table so productizing
  later is a migration, not a rewrite.

### 3.10 Keep the single process (simplification)
Run the evaluator as a goroutine ticker inside the API binary with a Postgres advisory lock.
No separate worker deployment, no queue. Add a `POST /v1/admin/evaluate` endpoint so you
can trigger a run on demand while testing; without it, tuning means waiting hours.

---

## 4. Decisions to lock before writing code

| Decision | Recommendation | Why |
|---|---|---|
| Backend language | **Go** (as planned) | You already have Go 1.26 installed and have shipped Go code. Single binary deploys trivially. If you later prefer one language end-to-end, Expo + Supabase Edge Functions + pg_cron is the TypeScript-only alternative. |
| Hosting | **Fly.io** for the Go binary, **Supabase** for Postgres | Fly runs one small machine for a few dollars a month with HTTPS and secrets built in. Supabase free Postgres is fine; the 2-hourly evaluator keeps it from auto-pausing. |
| Push delivery | **Expo Push Service** (not raw APNs) | One HTTP call from Go, Expo handles APNs tokens and certificates. Switch to direct APNs only if you need features Expo does not expose. |
| App build | **EAS development build** on a physical iPhone, never Expo Go | Remote push does not work in Expo Go since SDK 53. Simulators do not receive remote push. |
| LLM | **Claude Opus 5.5** (`claude-opus-5-5`), structured outputs via `output_config.format`, effort `low` for interpretation and `medium` for decisions | Judgement quality *is* the product. At your volume (≈10 thoughts/day, 4 decision runs/day) the bill is roughly USD 5–8/month, so a cheaper model saves nothing meaningful. |
| Voice | **iOS keyboard dictation** for MVP | Zero code. Revisit with on-device speech recognition only if hands-free capture matters. |
| Conversation | **Per-thought thread**, Phase 4 | Needed for "Let's do it" to mean anything. Data model reserved from Phase 1. |
| Notification actions | **All actions open the app** for MVP | Background action handling in Expo is the least reliable part of the pipeline. Validate it later as an optimisation. |
| Daily cap | Start at **1** proactive nudge/day, raise to 2 after two weeks | The doc says 2; starting lower makes the first weeks' feedback cleaner and is easier to loosen than tighten. |
| Evaluator cadence | Every **2 hours**, 08:00–21:00 local | Enough granularity for "weekend morning" style timing without burning LLM calls. |

---

## 5. Target architecture

### 5.1 Repository layout (monorepo)

```
companion/
  apps/mobile/            Expo + TypeScript (expo-router, expo-notifications, expo-dev-client)
  services/api/           Go module: API + evaluator in one binary
    cmd/api/main.go
    internal/http/        handlers, auth middleware
    internal/store/       sqlc-generated queries, pgx pool
    internal/ai/          Anthropic client, prompts, schemas, validation
    internal/engine/      candidate selection, scoring, policy, decision run
    internal/push/        Expo push client
    migrations/           goose SQL migrations
    prompts/              interpret_v1.md, decide_v1.md (go:embed), fixtures/thoughts.jsonl
  docs/                   discovery doc, this plan, decision log
  fly.toml
```

Go libraries: stdlib `net/http` (1.22+ pattern routing), `pgx/v5`, `sqlc`, `goose`,
`log/slog`, `github.com/anthropics/anthropic-sdk-go/v2`.
Mobile libraries: `expo-router`, `expo-notifications`, `expo-secure-store`,
`@tanstack/react-query`. No state library until needed.

### 5.2 Data model v1

```sql
users                  (id, timezone, created_at)
user_preferences       (user_id PK, timezone text, quiet_start time, quiet_end time,   -- quiet = sleep window
                        work_start time, work_end time, work_days int[],           -- routine, stated once
                        nudge_during_work text,                                     -- 'yes'|'lunch'|'no'
                        daily_limit int, min_interval_minutes int, notifications_enabled bool,
                        style_scores jsonb)            -- {"companion":{"helpful":3,"not":1}, ...}
device_tokens          (id, user_id, expo_push_token, platform, created_at, last_seen_at)

thoughts               (id, user_id, raw_content, source text,              -- 'text'|'voice'
                        status text,                                        -- active|snoozed|done|archived|faded
                        current_interpretation_id, snoozed_until,
                        created_at, updated_at, last_touched_at,
                        last_nudged_at, nudge_count int, ignored_streak int)
thought_interpretations(id, thought_id, version int, source text,           -- 'ai'|'user'
                        type text, title, description, desired_outcome,
                        next_steps jsonb, estimated_effort_minutes,
                        confidence numeric, clarification_needed bool, clarification_question,
                        model, prompt_version, created_at)
thought_events         (id, thought_id, nudge_id null, type text, payload jsonb, created_at)
                        -- append-only: created, interpreted, viewed, edited, progress_note,
                        --              snoozed, dismissed, completed, not_relevant, nudge_opened...

decision_runs          (id, started_at, finished_at, trigger text,          -- 'ticker'|'manual'
                        candidate_count int, scores jsonb, llm_decision jsonb,
                        outcome text, reason text)                          -- outcome: nudged|none|blocked
nudges                 (id, thought_id, decision_run_id, nudge_type, style, message,
                        suggested_action, status text,                      -- scheduled|sent|opened|acted|expired|suppressed
                        suppressed_reason, scheduled_for, sent_at, opened_at,
                        feedback text, feedback_reason text, created_at)
llm_calls              (id, purpose, model, prompt_version, input_tokens, output_tokens,
                        cost_usd, latency_ms, request jsonb, response jsonb, created_at)

thought_messages       (id, thought_id, role text, content text, created_at)   -- Phase 4
```

`last_touched_at` is updated by any user-initiated event and is the input to the R score.
`ignored_streak` resets to 0 on any user response to a nudge.

### 5.3 API surface

```
POST   /v1/thoughts                      create (returns immediately; interpretation async)
GET    /v1/thoughts?status=active        list
GET    /v1/thoughts/{id}                 detail incl. current interpretation, events, nudges
PATCH  /v1/thoughts/{id}                 status, snoozed_until, user correction -> new interpretation (source=user)
POST   /v1/thoughts/{id}/events          progress_note, viewed, etc.
GET    /v1/nudges                        feed (newest first, with thought title + reason)
POST   /v1/nudges/{id}/respond           {action: next_steps|do_it|later|not_interested}
POST   /v1/nudges/{id}/feedback          {helpful: bool, reason?: tone|timing|relevance|other}
GET/PUT /v1/preferences
POST   /v1/devices                       register expo push token
GET    /v1/debug/decision-runs           last N runs
POST   /v1/admin/evaluate                trigger one evaluator run now
POST   /v1/thoughts/{id}/messages        Phase 4: chat turn
```

All routes behind `Authorization: Bearer <token>`.

### 5.4 LLM calls (two prompts, both versioned)

**Interpret** (on thought creation, async):
- Input: raw thought, the titles of the user's last 20 active thoughts (for related-thought
  hints), today's date.
- Output schema: `type`, `title`, `description`, `desired_outcome`, `next_steps[]` (each with
  `label`, `estimated_minutes`), `confidence`, `clarification_needed`, `clarification_question`,
  `related_thought_ids[]`, `assumptions[]`.
- Backend validates: type in enum, confidence in [0,1], ≤5 next steps, related ids exist.
  Store as interpretation v1. If `clarification_needed`, show the question in-app only;
  never via push.

**Decide** (per evaluator run, only if there are candidates):
- Input: shortlist (≤5) with interpretation, days since touched, nudge history summary,
  last 3 nudges' feedback; preferences and style scores; now (local), weekday; policy
  summary (what is already enforced so the model does not re-enforce it).
- Output schema: `action` (`nudge` | `none`), `thought_id`, `nudge_type`
  (check_in|action_prompt|opportunity|follow_up|revival|relevance_check), `style`
  (companion|coach), `message` (≤200 chars), `suggested_action`, `reason` (one sentence,
  shown in the debug screen and the feed).
- Backend validates: `thought_id` is in the shortlist, style in enum, message length, no
  dates/deadlines invented (reject if message contains "deadline", "overdue", "must").

Implementation notes for Go + Anthropic SDK:
- Use `output_config.format` with a JSON schema for both prompts; validate again in Go.
- Check `stop_reason` before reading content; handle `refusal` by recording the run as
  `blocked` rather than crashing the ticker.
- Put the static system prompt first and the per-run data last so prompt caching applies.
- Log every call to `llm_calls` with `usage` tokens.

### 5.5 Evaluator (one run)

```
run(trigger):
  acquire pg advisory lock (skip if held)
  prefs, now_local = load
  if in_quiet_hours(now_local):              record(outcome=blocked, reason=quiet_hours); return
  if nudges_sent_today >= prefs.daily_limit: record(blocked, daily_limit); return
  if minutes_since_last_sent < prefs.min_interval_minutes: record(blocked, min_interval); return
  candidates = SELECT thoughts WHERE status='active'
                 AND (snoozed_until IS NULL OR snoozed_until < now)
                 AND (last_nudged_at IS NULL OR last_nudged_at < now - cooldown(thought))
  if empty:                                   record(none, no_candidates); return
  scored = score(each candidate)              -- Section 6.4, components stored in run.scores
  shortlist = top 5 by score
  decision = llm.decide(shortlist, context)   -- Section 5.4
  if decision.action == none:                 record(none, decision.reason); return
  validate(decision) else                     record(blocked, invalid_llm_output); return
  nudge = insert nudges(status=scheduled)
  send via Expo push; on success status=sent, thought.last_nudged_at=now, nudge_count+1
  record(nudged, decision.reason)
```

`cooldown(thought) = 3 days × 2^ignored_streak`, capped at 30 days.

### 5.6 Notification pipeline (mobile)

1. On launch: request permission, get Expo push token, `POST /v1/devices`.
2. Register one notification category `nudge` with four actions (all `opensAppToForeground`).
3. Payload: `{nudge_id, thought_id}`. On tap or action: `POST /v1/nudges/{id}/respond`, then
   navigate to the thought detail with the nudge highlighted.
4. Foreground receipt: show an in-app banner rather than a system alert.

---

## 6. Behaviour specifications the doc left open

### 6.1 Classification confidence
- confidence ≥ 0.75: accept silently.
- 0.5–0.75: accept, show an "AI guessed this is a …, tap to change" chip on the detail view.
- < 0.5 or `clarification_needed`: store as type `unclear`, show the clarification question
  in-app, exclude from nudging until answered or edited.

### 6.2 Status lifecycle
`active` → `snoozed` (until date) → `active`
`active` → `done` (user) | `archived` (user) | `faded` (system, see 6.6)
Any status can return to `active` via the detail view.

### 6.3 Nudge types and when they apply
| Type | Trigger | Cap |
|---|---|---|
| action_prompt | actionable intention with a ≤30 min next step | normal |
| check_in | personal development, ≥7 days since touched | normal |
| opportunity | experience/interest and it is Thu–Sat | 1/week per thought |
| follow_up | user said "do_it" or added progress ≥3 days ago | normal |
| revival | ≥21 days untouched, never nudged or last nudge >14 days ago | 1/week total |
| relevance_check | ignored_streak ≥ 3 | once per thought |

### 6.4 Scoring inputs (all in [0,1])
- **R** recency: `0` if touched < 2 days ago, else `min(days_since_touch / 14, 1)`.
- **I** importance: start 0.5; +0.25 if user edited or added context; +0.25 if opened
  detail ≥2 times; −0.25 per dismissal (min 0).
- **A** actionability: 1.0 if a next step ≤30 min exists; 0.5 if any steps; 0 if `unclear`.
- **O** opportunity: base by type: experiences 1.0 on days off and Thu–Sat else 0.3;
  development 0.6; actionable 0.5; ideas 0.4. Then routine from preferences: inside working
  hours multiply by 0.3 if `nudge_during_work = no`, by 1.0 in the lunch hour if `lunch`,
  by 0.6 if `yes`. Sleep window is a hard gate, never a score. (Weather and calendar later.)
- **F** feedback: style score (6.5) of the best style; 0.5 until there are ≥3 data points.

Weights stay `0.30R + 0.20I + 0.20A + 0.15O + 0.15F` until two weeks of debug-screen
reading suggests otherwise. Store all components per candidate in `decision_runs.scores`.

### 6.5 Preference learning
```
score(style) = (helpful[style] + 1) / (helpful[style] + not_helpful[style] + 2)
pick: with probability 0.2 choose a random style (exploration), else argmax score
```
"Action taken" counts as helpful. "Dismissed with reason=tone" counts as not helpful for that
style. Ignored counts as nothing. Later split counters by thought type.

### 6.6 Stale handling
After a `relevance_check` nudge is ignored for 7 days, set status `faded` and stop nudging.
Faded thoughts stay visible in the overview under "Quietly set aside" and can be reactivated.
Never delete.

### 6.7 Resolved open questions (from Section 22)
| Question | Default |
|---|---|
| Revive interests proactively? | Yes, via `opportunity` and `revival` types, both weekly-capped. |
| Passing thought vs intention? | Do not decide at capture. The user's first response to a nudge decides. |
| Ask clarifying questions? | Yes, in-app only, never by push. |
| Multiple related thoughts? | Interpret prompt returns `related_thought_ids`; show as links. Merging is manual. |
| What interrupts the user? | Only what passes policy plus the LLM's "nudge" verdict. Day-of-week is the only context signal in MVP. |
| Daily digest? | Not in MVP. Reconsider after Phase 3 if the feed shows many `none` outcomes with high scores. |
| Stale detection? | Section 6.6. |

---

## 7. Phased action plan

Effort is in focused days (roughly a full day of uninterrupted work). Part-time, treat one
focused day as one weekend day.

### Phase 0 — Prerequisites and de-risking spike (2–3 days)
Goal: a push notification arrives on your iPhone, sent by Go code on Fly.io, carrying text
produced by a structured-output Claude call.

1. Enrol in the Apple Developer Program (can take 24–48 h; start today).
2. Install tooling: `npm i -g eas-cli`, `brew install flyctl libpq`, `go install` sqlc and goose.
3. Create the Supabase project; note the connection string.
4. Create the monorepo; `go mod init`; hello-world API with bearer auth; deploy to Fly.
5. `npx create-expo-app` with expo-router; add `expo-dev-client` and `expo-notifications`;
   `eas build --profile development --platform ios`; install on the phone.
6. Register the push token with the API; `curl` an endpoint that sends an Expo push.
7. Add the Anthropic Go SDK; one structured-output call that classifies a hard-coded thought.
8. Create an Anthropic API key, store as a Fly secret.

Exit: step 6 and 7 work end to end. Write down anything that surprised you in `docs/`.

### Phase 1 — Capture and memory (4–6 days)
Goal: dump thoughts for a week and trust the system to remember and understand them.

1. Migrations for `users`, `user_preferences`, `device_tokens`, `thoughts`,
   `thought_interpretations`, `thought_events`, `llm_calls`. Seed one user.
2. sqlc queries; thought CRUD handlers; event append.
3. Interpretation job: goroutine after insert, prompt `interpret_v1`, schema validation,
   store v1, log the call. Retry once on transient error; mark `unclear` on persistent error.
4. User corrections create interpretation `source=user` and bump `current_interpretation_id`.
5. Mobile: Home (capture box + recent 10), Thought detail (raw, interpretation, next steps,
   status actions, edit type/title), simple overview list grouped by status.
6. `prompts/fixtures/thoughts.jsonl` with 30 varied thoughts and expected types; a Go test
   behind a build tag that runs them and reports accuracy. Run it before every prompt change.
7. Routine onboarding (added 11 Nov 2026): `/start` asks four button questions, all skippable
   with defaults: sleep window (quiet hours), weekday working hours, whether nudges are okay
   during work (yes / lunch only / no), and days off. Stored in `user_preferences`; `/routine`
   edits them later. Stated answers are priors; observed response times override them in Phase 3.

Exit: 20+ real thoughts captured over a week; ≥80% interpretations acceptable without edit;
capture-to-saved under 2 seconds on the phone.

### Phase 2 — Decision engine and nudges (5–7 days)
Goal: the agent brings relevant thoughts back, and you can explain every decision it made.

1. Migrations for `decision_runs`, `nudges`.
2. Candidate selection query; scoring with stored components; policy checks.
3. `decide_v1` prompt with structured output; validation; `none` handling.
4. Expo push sender; nudge status transitions; `last_nudged_at` / `nudge_count` updates.
5. Ticker goroutine (2 h, local 08:00–21:00) with advisory lock; manual trigger endpoint.
6. Mobile: nudge feed; notification categories and response handling; deep link into
   thought detail; preferences screen (quiet hours, daily limit, enable/disable).
7. Debug screen listing decision runs with outcome, reason and candidate scores.

Exit: one week of live nudges; zero policy violations in `decision_runs`; every sent nudge
has a reason you agree with or a logged note on why not.

### Phase 3 — Feedback and learning (4–5 days)
Goal: the agent starts adapting and backing off.

1. Feedback endpoint and UI (helpful / not helpful, reason chips: tone, timing, relevance).
2. Snooze (1 day / 3 days / 1 week / pick date), dismiss, not relevant, done, progress note.
3. `ignored_streak` and exponential cooldown; `relevance_check` type; `faded` status (6.6).
4. Style scores with exploration (6.5); F component wired into scoring and into the decide
   prompt as a hint.
5. `revival` and `opportunity` types with weekly caps.
6. SQL views for the Section 19 metrics; a `make metrics` target that prints them.

Exit: two to three weeks of use. Review metrics weekly. Adjust weights only with evidence
from the debug screen.

### Phase 4 — Per-thought conversation (4–5 days)
Goal: "Let's do it" leads somewhere useful.

1. `thought_messages` table; chat endpoint that sends the thought's interpretation, events
   and last 20 messages as context; streaming optional.
2. Agent can propose an updated next-step list; user taps "apply" to create a new
   interpretation `source=user`.
3. Mobile: thread screen reachable from detail and from nudge actions `next_steps` / `do_it`.

Exit: for three real thoughts, a conversation moved the thought to a concrete next step.

### Phase 5 — Nice-to-haves (pick by felt need, not by list)
Share-sheet capture; home-screen widget; weather as an O input (Open-Meteo, free);
pgvector related-thought search; raising the daily cap to 2; background notification
actions; app icon and polish; TestFlight for a second user.

**Total: roughly 20–26 focused days.** Full-time that is 4–5 weeks, matching the discovery
estimate. Weekends only, plan on three months.

---

## 8. Prerequisite checklist

- [ ] Apple Developer Program membership (paid) — start first, it has a waiting period
- [ ] Expo account (EAS free tier is enough for development builds)
- [ ] Fly.io account; `flyctl` installed
- [ ] Supabase project; `psql` installed (`brew install libpq`)
- [ ] Anthropic API key with a monthly spend limit set in the console
- [ ] `eas-cli`, `sqlc`, `goose` installed
- [ ] Physical iPhone on the same Apple ID, developer mode enabled
- [ ] A private git remote for the monorepo and for weekly `pg_dump` output

---

## 9. Running cost estimate (personal use)

| Item | Approximate monthly cost |
|---|---|
| Apple Developer Program | USD 99 / year |
| Fly.io, one shared-cpu machine | USD 2–5 |
| Supabase Postgres free tier | 0 |
| Expo push service, EAS free tier | 0 |
| Claude Opus 5.5 at ~10 interpretations + ~4 decision runs per day | USD 5–8 |

Set a spend cap on the Anthropic console before Phase 0 step 7.

---

## 10. Risks introduced or sharpened by this plan

- **Evaluator double-send** if two Fly machines run. Mitigated by the advisory lock and by
  running a single machine; keep it that way until there is a reason not to.
- **Supabase free project pausing** after a week without traffic. The 2-hourly ticker
  prevents it; if you ever pause the app, expect a manual resume.
- **Expo development build expiry.** Builds signed with a paid account last a year; rebuild
  whenever native dependencies change.
- **LLM refusal or malformed output** stalls a run. Every run records `blocked` with the
  reason; the ticker must never crash on a bad response.
- **Learning from too little data.** One user generates maybe 30 nudges a month. Keep
  exploration on and do not trust style scores until there are at least 10 feedback points
  per style.

---

## 11. First three things to do this week

1. Enrol in the Apple Developer Program.
2. Run Phase 0 steps 2–5 and get a development build on your phone.
3. Make one Go handler send one Expo push to that phone.
