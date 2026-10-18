#!/bin/sh
set -eu

if [ -z "${SESSION_SECRET:-}" ]; then
    env_file=/app/data/.env
    if [ ! -e "$env_file" ]; then
        umask 077
        env_tmp=$(mktemp /app/data/.env.XXXXXX)
        trap 'rm -f "$env_tmp"' EXIT
        generated_secret=$(openssl rand -hex 32)
        printf 'SESSION_SECRET=%s\n' "$generated_secret" > "$env_tmp"
        # Publish a complete .env without overwriting another startup's file.
        if ! ln "$env_tmp" "$env_file"; then
            [ -s "$env_file" ] || exit 1
        fi
        rm -f "$env_tmp"
        trap - EXIT
    fi
    set -a
    . "$env_file"
    set +a
    if [ -z "${SESSION_SECRET:-}" ]; then
        echo "SESSION_SECRET is missing or empty in $env_file" >&2
        exit 1
    fi
fi

exec /usr/local/bin/jieli "$@"
