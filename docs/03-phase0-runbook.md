# Phase 0 runbook — Telegram capture + Claude interpretation + proactive nudge

Goal: send the bot a thought, get an interpretation back with buttons, and receive a
server-initiated nudge. That proves every link the real product needs.

Already done: API deployed at `https://thought-companion-api.fly.dev`, Anthropic key and API
token set as Fly secrets, live interpretation verified.

## A. Create the bot (3 min)

1. In Telegram, open **@BotFather** and send `/newbot`.
2. Name: anything (e.g. `Companion`). Username: must end in `bot` and be unique
   (e.g. `jayant_companion_bot`).
3. BotFather replies with a token like `123456789:AAH...`. Copy it.
4. Optional but recommended: `/setprivacy` is irrelevant for private chats; skip it.

## B. Configure and deploy (5 min)

Put the token in `services/api/.env` as `TELEGRAM_BOT_TOKEN=...`, leave `TELEGRAM_CHAT_ID`
empty, then:

```bash
cd services/api
fly secrets set TELEGRAM_BOT_TOKEN=<token> --stage
fly deploy
```

## C. Lock the bot to your chat (2 min)

1. Open the bot in Telegram and send `/start`.
2. It replies "This bot is private. Chat id: NNNN". Copy the number.
3. ```bash
   fly secrets set TELEGRAM_CHAT_ID=NNNN       # triggers a restart
   ```
4. Send `/start` again; you should get the welcome message. Any other Telegram user who finds
   the bot only ever sees the "private" line.

## D. The spike (3 min)

1. Send a thought, e.g. `I should probably sell my old LS2 helmet, it's just sitting around.`
   Expect "Got it." within a second, then the interpretation with two buttons a few seconds later.
2. Tap **Looks right**. The buttons disappear and a toast confirms.
3. Send `/nudge`. A nudge about that thought arrives with four action buttons. Tap one.
4. From your Mac, trigger the same thing without touching Telegram:
   ```bash
   TOKEN=$(grep '^API_TOKEN=' services/api/.env | cut -d= -f2)
   curl -s -X POST https://thought-companion-api.fly.dev/v1/admin/nudge -H "Authorization: Bearer $TOKEN"
   ```
   This is the exact path the Phase 2 evaluator will use.

## Exit criteria

- [ ] D1: interpretation arrives in chat
- [ ] D3: `/nudge` arrives with working buttons
- [ ] D4: curl-triggered nudge arrives

## Known rough edges (Phase 1 fixes them)

- Thoughts live in memory; a redeploy forgets them. Phase 1 adds Postgres.
- "Not quite" records nothing yet. Phase 1 adds corrections as a new interpretation version.
- `apps/mobile` is parked. See `docs/decisions.md`.
