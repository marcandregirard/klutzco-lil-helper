# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Discord bot for the Idle Clans game community (KlutzCo clan), written in Go. Provides boss/key lookup commands, automated clan log reporting, daily/weekly boss quest scheduling, and market price analysis.

## Build & Run Commands

```bash
# Build
go build -o bot ./cmd/bot

# Run tests
go test ./...

# Run a single test
go test ./internal/bot/... -run TestBuildBossMessage

# Run locally (requires .env with DISCORD_BOT_TOKEN and DISCORD_APP_ID)
go run ./cmd/bot

# Docker
docker compose build
docker compose up
```

## Environment Variables

Required: `DISCORD_BOT_TOKEN`, `DISCORD_APP_ID`

Optional: `DB_PATH` (default: `/app/data/lilhelper.db`), `CLAN_LOG_URL`, `CLAN_LOG_INTERVAL`, `CLAN_MESSAGE_CHANNEL`, `CLAN_DONATION_CHANNEL`, `BOSS_CHANNEL`, `BOSS_SUMMARY_CHANNEL`, `BOSS_SUMMARY_TIME` (default: `9:30`, format: `HH:MM` in Eastern time)

Local dev uses `.env` (auto-loaded by godotenv). Docker uses `.klutz.env`.

## Architecture

**Entry point:** `cmd/bot/main.go` — loads env, opens SQLite (WAL mode, single connection), runs migrations, creates and starts the bot.

**`internal/bot/`** — Bot runtime:
- `handler.go` — Discord session setup and interaction routing
- `clanlogs.go` — Background goroutine fetching clan logs from the Idle Clans API on a timer, storing new messages in SQLite
- `messagesender.go` — Background goroutine posting unsent clan messages to Discord every 30s. Detects large gold donations (>=1M) and posts celebration embeds
- `messagescheduler.go` — Background goroutine posting daily/weekly boss quest polls at UTC midnight with emoji reactions

**`internal/commands/`** — Discord slash commands (`/boss`, `/keys`, `/market-food`). Each command defines its `*discordgo.ApplicationCommand`, handler, and optional autocomplete. Uses a global DB singleton set via `SetDB()`.

**`internal/model/`** — Data models and persistence:
- `boss.go` / `data.go` — Boss and key lookup data with static maps
- `clanmessage.go` — ClanMessage CRUD (insert, get unsent, mark sent)
- `scheduledmessage.go` — ScheduledMessage CRUD for tracking posted polls
- `migrate.go` — Schema creation and legacy upgrades

## Key Dependencies

- `github.com/bwmarrin/discordgo` — Discord API
- `modernc.org/sqlite` — Pure-Go SQLite driver (no CGO)
- `github.com/joho/godotenv` — .env file loading

## Database

SQLite with WAL journal mode. Single connection (`SetMaxOpenConns(1)`) to avoid writer contention. Two tables: `clan_messages` (with unique index for dedup) and `scheduled_messages`.

## External APIs

- Clan logs: `https://query.idleclans.com/api/Clan/logs/clan/KlutzCo?limit=10`
- Market prices: `https://query.idleclans.com/api/PlayerMarket/items/prices/latest`

Both use retry logic with exponential backoff (3 attempts).
