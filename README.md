# Jieli

Jieli stores, searches, shares and exports AI coding conversations for humans and
agents. Supported plugins sync conversations directly with Jieli.
Previously stored Amp conversations remain readable; the Amp runtime, model relay
and model configuration are no longer offered.

## Run your own instance

Build the frontend and Go server:

```sh
npm --prefix frontend ci
npm --prefix frontend run build
go build -o jieli ./cmd/server
```

Configure foundational environment variables using `.env.example`, including a
private `SESSION_SECRET`, `SQLITE_PATH` and a random `JIELI_SETUP_TOKEN`. Set
the public site URL, SMTP and registration in Site Administration → Site
Configuration.
The server reads the process environment; it does not automatically load `.env`.
Start `./jieli`, then initialize an administrator at `/web/auth/signin`.

Self-hosted users have unlimited sync messages. Email registration is closed by
default; an administrator may create accounts directly, or configure and test
SMTP before explicitly opening registration. OAuth providers are optional.

## Back up and upgrade

Before upgrading, back up the SQLite database with its online backup command
(`sqlite3 data/jieli.db '.backup /safe/path/jieli.db'`) and copy the attachment
storage directory. Keep the old binary and its environment configuration.
Do not copy only the live database file while SQLite is in WAL mode.

Build the new version, stop the old process and start the new binary with the
same database, attachment paths and session secret. Supported schema upgrades
preserve accounts, conversations and membership history. Very old schemas with
required `threads.data_json` are rejected; back up and migrate them explicitly.
Configure the site URL and SMTP in Site Administration after upgrading from an
older environment-based mail configuration.

To restore, stop Jieli, retain the failed database and its WAL files in a separate
location, restore the backup database and matching attachments, then start the
matching old binary. Verify login, historical conversations and image downloads.

## Editions and plugins

The public edition builds without commercial source or dependencies. The hosted build and its billing implementation are maintained separately
in the private branch.

Install conversation upload plugins from [jieliapp/plugins](https://github.com/jieliapp/plugins).
The public API supports Claude Code, Codex, OpenCode and Pi uploads. Previously
stored Amp conversations remain readable; new Amp uploads are rejected.

## Verify

```sh
go test ./...
npm --prefix frontend run check
npm --prefix frontend test
npm --prefix frontend run build
```
