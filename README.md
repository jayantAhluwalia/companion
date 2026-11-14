# companion

A personal AI companion that remembers what you want to do and gently helps you act on it.

- `docs/` — product discovery, phased action plan, Phase 0 runbook, decision log
- `services/api/` — Go API + Telegram bot + nudge engine, one binary, deployed to Fly.io
- `apps/mobile/` — Expo iOS app, parked (see `docs/decisions.md`)

## Run locally

```bash
cd services/api && cp .env.example .env   # fill in the three secrets
make run
```

## Working on it

```bash
make dev                       # run locally on the "dev" schema; messages are logged, not sent
make test                      # unit tests, free
make dbtest                    # the store's SQL against a throwaway schema
make backup                    # export every table to ~/companion-backups and prove it restores
make live                      # 39-thought fixture test + one web lookup against the real API (~$0.85)
make sql Q="select id, status from thoughts"    # read-only query, no psql needed
make deploy
```

`make run` uses the real database but does not poll Telegram or nudge on a timer; the deployed
process owns both. Prefer `make dev` for anything experimental. CI (`.github/workflows`) runs
vet, unit and database tests on every push, and a nightly job takes an encrypted backup. To see what the engine would do right now without sending:

```bash
curl -X POST -H "Authorization: Bearer $API_TOKEN" "https://thought-companion-api.fly.dev/v1/admin/evaluate?dry=1"
```

In the bot: `/nudge` asks for the best thing to bring up now, `/why` explains the last
decision and shows the week's budget and today's send window, `/review` runs the weekly review
now, `/routine` edits sleep, work, days off, city and the review day.

Inviting someone: `/invite` (owner only) gives a single-use link. They are told how their data
is handled, answer five questions, and from then on have their own thoughts, numbers, budget and
nudges. `/users` shows who is in, with counts and cost only. Anyone can `/export` or `/forget`.

How the engine decides when to speak is in `docs/02-action-plan.md` (sections 5.5 and 6.9); the
evidence behind it is in `reports/Nudge timing research evidence.md`.
