#!/bin/sh
set -eu
umask 077

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$repo_dir"

for dependency in git go node npm cc openssl; do
    if ! command -v "$dependency" >/dev/null 2>&1; then
        echo "Missing dependency: $dependency. Install Git, Go 1.25+, Node.js 20.19 / 22.12 / 24 or newer, npm, a C compiler and OpenSSL." >&2
        exit 1
    fi
done
node -e '
const [major, minor] = process.versions.node.split(".").map(Number);
if (!((major === 20 && minor >= 19) || (major === 22 && minor >= 12) || major >= 24)) {
    console.error("Node.js 20.19+, 22.12+, or 24+ is required.");
    process.exit(1);
}'
go_version=$(GOTOOLCHAIN=local go env GOVERSION)
node -e '
const match = /^go(\d+)\.(\d+)(?:\.\d+)?$/.exec(process.argv[1]);
if (!match || Number(match[1]) < 1 || (Number(match[1]) === 1 && Number(match[2]) < 25)) {
    console.error("Go 1.25 or newer is required. Installed: " + process.argv[1]);
    process.exit(1);
}' "$go_version"

mkdir -p data .cache
env_file="$repo_dir/data/.env"
if [ ! -e "$env_file" ]; then
    env_tmp=$(mktemp "$repo_dir/data/.env.XXXXXX")
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

SESSION_SECRET=
set -a
. "$env_file"
set +a
if [ -z "${SESSION_SECRET:-}" ]; then
    echo "SESSION_SECRET is missing or empty in $env_file" >&2
    exit 1
fi

echo "Building Jieli..."
npm --prefix frontend ci
npm --prefix frontend run build
CGO_ENABLED=1 go build -trimpath -o .cache/jieli ./cmd/server

echo "Starting Jieli on port ${PORT:-8080}. Press Ctrl+C to stop."
exec "$repo_dir/.cache/jieli" "$@"
