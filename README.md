# companion

A personal AI companion that remembers what you want to do and gently helps you act on it.

- `docs/` — product discovery, phased action plan, Phase 0 runbook, decision log
- `services/api/` — Go API + Telegram bot + (Phase 2) evaluator, one binary, deployed to Fly.io
- `apps/mobile/` — Expo iOS app, parked (see `docs/decisions.md`)

## Run locally

```bash
cd services/api && cp .env.example .env   # fill in the three secrets
make run
```
