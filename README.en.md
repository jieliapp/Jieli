# Jieli

[简体中文](README.md) · English

Jieli provides **cloud sync, backup, search and sharing for AI coding conversations**: sync local Claude Code and Codex sessions to your own server in real time, browse them across devices, search their full text and share them by link.

Sync historical sessions and export conversations while keeping control of your data. Support for other agents is planned.

## Deploy with Docker

Install Docker and Docker Compose, then run these commands for your first deployment:

```sh
git clone https://github.com/jieliapp/Jieli.git
cd Jieli
docker compose up -d --build
```

Open [http://localhost:8080](http://localhost:8080) and follow the prompts to create an administrator and get started; data persists in the Docker volume mounted at `/app/data`, so back it up before upgrading.

## Deploy without Docker

For Linux/macOS. The script checks prerequisites and reports anything missing.

```sh
git clone https://github.com/jieliapp/Jieli.git
cd Jieli
./scripts/run.sh
```

The script builds and starts Jieli at [http://localhost:8080](http://localhost:8080). The database, attachments and automatically generated `.env` are stored in `data/`; back up this directory before upgrading. The server runs in the foreground; press `Ctrl+C` to stop it. Run the same script again after updating the code.

## Sync conversations

Install the [Jieli plugin](https://github.com/jieliapp/plugins) for your agent to sync conversations to your instance in real time.

To sync historical Claude Code and Codex sessions (requires Node.js 20+):

```sh
curl -fsSLo jieli-sync.mjs https://jieli.app/jieli-sync.mjs && node jieli-sync.mjs
```

The script uses your plugin configuration and prompts for missing settings. For self-hosting, set `JIELI_BASE_URL` or `base_url` in `~/.config/jieli/settings.json` to your instance URL.

## License

This project is licensed under the [Apache License 2.0](LICENSE).
