# AGENTS.md

## Project overview

This repository contains a Discord bot written in Go. It uses Discord's gateway
and application commands through `discordgo`, with SQLite as its local durable
store.

## Layout

- `cmd/bot`: executable entry point and process lifecycle.
- `internal/config`: environment-based configuration.
- `internal/database`: SQLite connection and schema migrations.
- `internal/discord`: Discord gateway handlers and commands.
- `data/`: local runtime database files; never commit its contents.

Keep reusable application code in `internal/`; `cmd/bot/main.go` should stay
focused on wiring dependencies and shutdown.

## Local setup

1. Install [mise](https://mise.jdx.dev/) and run `mise install` to provision
   the project Go toolchain.
2. Copy `.env.example` to `.env` if it is absent.
3. Set `DISCORD_TOKEN` and `DISCORD_APPLICATION_ID` from the Discord developer
   portal. Use `DISCORD_GUILD_ID` while developing for near-instant command
   updates; leave it blank to register globally.
4. Run `mise run tidy` after changing imports, then `mise run run`.

The bot creates the directory containing `DATABASE_PATH` and applies migrations
at startup. Default local storage is `data/bot.db`.

## Configuration and secrets

- Never commit `.env`, Discord tokens, or SQLite database files.
- Add every new configuration key to `.env.example` and document its default.
- Configuration is read once at startup. Treat invalid or missing required
  values as startup errors rather than allowing a partially configured bot.

## Database changes

- Add an ordered SQL file under `internal/database/migrations`, using a zero
  padded numeric prefix (for example, `0002_add_profiles.sql`).
- Migrations run transactionally and are recorded in `schema_migrations`.
- Do not alter an already-shipped migration; add a new one instead.
- Prefer parameterized queries and pass `context.Context` through database APIs.

## Discord changes

- Prefer slash commands and interaction responses over prefix commands.
- Keep command definitions alongside their handlers in `internal/discord`.
- A command must respond within Discord's interaction deadline. Defer a reply
  before doing work that may take longer than a moment.
- Restrict gateway intents to the minimum the bot needs. Update the Discord
  developer portal before enabling privileged intents in code.

## Birthday feature

- Birthday data is scoped to a Discord server; store only day and month, never
  birth year or age.
- The configured staff role administers notifications and cleanup. The server
  owner always retains recovery access to change that role.
- Notification timezones are IANA names stored per server. The scheduler posts
  at 09:00 local time and must prevent duplicate delivery after restarts.
- A non-blank notification template must contain exactly one case-sensitive
  `{usuarios}` placeholder (for example, `🎂 ¡Feliz cumpleaños, {usuarios}!`),
  which is replaced inline with birthday member mentions. When it is blank, the
  announcement consists only of those mentions. If multiple members share a
  date, mention each in record order, separated by `, `.

## Quality checks

Run these before handing off a change:

```sh
mise run fmt
mise run check
```

Avoid adding dependencies without a clear need. Favor the standard library for
configuration, logging, and simple utilities.
