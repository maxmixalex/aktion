#!/bin/sh
set -eu

: "${GITHUB_CONFIG_URL:?GITHUB_CONFIG_URL is required}"
: "${SCALE_SET_NAME:?SCALE_SET_NAME is required}"
: "${GITHUB_APP_CLIENT_ID:?GITHUB_APP_CLIENT_ID is required}"
: "${GITHUB_APP_INSTALLATION_ID:?GITHUB_APP_INSTALLATION_ID is required}"

PRIVATE_KEY_FILE="${GITHUB_APP_PRIVATE_KEY_FILE:-/run/secrets/github_app_private_key}"
MIN_RUNNERS="${MIN_RUNNERS:-0}"
MAX_RUNNERS="${MAX_RUNNERS:-1}"
LOG_LEVEL="${LOG_LEVEL:-info}"

if [ ! -r "$PRIVATE_KEY_FILE" ]; then
    echo "GitHub App private key is not readable: $PRIVATE_KEY_FILE" >&2
    exit 2
fi

set -- aktion-listener \
    --url "$GITHUB_CONFIG_URL" \
    --name "$SCALE_SET_NAME" \
    --min-runners "$MIN_RUNNERS" \
    --max-runners "$MAX_RUNNERS" \
    --app-client-id "$GITHUB_APP_CLIENT_ID" \
    --app-installation-id "$GITHUB_APP_INSTALLATION_ID" \
    --app-private-key "$(cat "$PRIVATE_KEY_FILE")" \
    --log-level "$LOG_LEVEL"

if [ -n "${LABELS:-}" ]; then
    set -- "$@" --labels "$LABELS"
fi

if [ -n "${RUNNER_IMAGE:-}" ]; then
    set -- "$@" --runner-image "$RUNNER_IMAGE"
fi

exec "$@"
