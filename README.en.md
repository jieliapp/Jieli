# Jieli

[简体中文](README.md) · English

Save, search, share and export AI coding conversations. Supports Claude Code and Codex, with support for other agents planned.

## Deploy with Docker

Install Docker and Docker Compose, then run these commands for your first deployment:

```sh
git clone https://github.com/jieliapp/Jieli.git
cd Jieli
docker compose up -d --build
```

Open [http://localhost:8080](http://localhost:8080), go to the sign-in page, and enter your email, password and password confirmation to create the administrator and sign in immediately. No verification email is needed.
Create the administrator before making the site available to others.

Administrators can create users directly, or configure and test SMTP in the admin panel before enabling email sign-up.
Self-hosted instances have no sync message limit. Configure the site URL and email settings in the admin panel.

The database, attachments and automatically generated `.env` live in the Docker volume at `/app/data`. Back up before upgrading. Do not run `docker compose down -v`: it deletes your data.

## Deploy without Docker

For Linux/macOS. The script checks prerequisites and reports anything missing.

```sh
git clone https://github.com/jieliapp/Jieli.git
cd Jieli
./scripts/run.sh
```

The script builds and starts Jieli at [http://localhost:8080](http://localhost:8080). The database, attachments and automatically generated `.env` are stored in `data/`; back up this directory before upgrading. The server runs in the foreground; press `Ctrl+C` to stop it. Run the same script again after updating the code.

## Sync conversations

Install the [Jieli plugin](https://github.com/jieliapp/plugins) for your agent to sync conversations to your instance.
