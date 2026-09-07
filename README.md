# Jef Bot

A Go Discord bot with slash commands and SQLite persistence.

## Start locally

1. Install [mise](https://mise.jdx.dev/) and provision the pinned Go toolchain:

   ```sh
   mise install
   ```

2. Create a Discord application and bot in the [Discord Developer Portal](https://discord.com/developers/applications).
3. Enable the `applications.commands` scope when inviting it to your server.
4. Fill in `DISCORD_TOKEN` and `DISCORD_APPLICATION_ID` in `.env`.
5. For development, set `DISCORD_GUILD_ID` to your test server ID so command changes appear immediately.
6. Start the bot:

   ```sh
   mise run run
   ```

The initial command is `/ping`, which responds with `Pong!`. The SQLite file is
created at `data/bot.db` by default. See `.env.example` for all settings.

## Cumpleaños

Members can manage only their own recurring birthday (day and month):

```text
/cumpleanos registrar fecha:DD/MM
/cumpleanos editar fecha:DD/MM
/cumpleanos eliminar
/cumpleanos ver
```

The server owner configures the staff role with `/cumpleanos configurar-rol`.
That role, and the owner as a recovery path, can set the notification channel
and IANA timezone, remove an absent user by Discord ID, or bulk-clean records
of departed members. Only members holding the configured staff role can use
`/cumpleanos registrar-usuario` to register someone else's birthday.
They can also use `/cumpleanos editar-usuario` to correct another member's
registered birthday. Staff can set announcement text with
`/cumpleanos configurar-mensaje`. A non-blank template must contain exactly
one case-sensitive `{usuarios}` placeholder, for example
`🎂 ¡Feliz cumpleaños, {usuarios}!`; it is replaced inline with the birthday
member mentions. Set it to blank to send only the mentions. Multiple members
are mentioned in record order, separated by `, `. Use `/ayuda` for command
syntax and permission details.

## Development

```sh
mise tasks
mise run fmt
mise run check
```

Database schema changes belong in `internal/database/migrations`; see
`AGENTS.md` for the project conventions.
