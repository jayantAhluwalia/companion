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
make test                      # unit tests, free
make live                      # 30-thought fixture test + one web lookup against the real API (~$0.65)
make sql Q="select id, status from thoughts"    # read-only query, no psql needed
make deploy
```

A local `make run` with `BOT_ENABLED=false` does not poll Telegram or nudge on a timer; the
deployed process owns both. To see what the engine would do right now without sending:

```bash
curl -X POST -H "Authorization: Bearer $API_TOKEN" "https://thought-companion-api.fly.dev/v1/admin/evaluate?dry=1"
```

In the bot: `/nudge` asks for the best thing to bring up now, `/why` explains the last
decision, `/routine` edits sleep, work, days off and city.
