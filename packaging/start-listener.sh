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

fail() {
    echo "$*" >&2
    exit 2
}

# Match the listener's signed integer flags, rejecting octal/ambiguous inputs.
validate_count() {
    case "$2" in
        ''|*[!0-9]*|0[0-9]*) fail "$1 must be a non-negative decimal integer" ;;
    esac
    if ! [ "$2" -ge 0 ] 2>/dev/null; then
        fail "$1 is outside the supported integer range"
    fi
}

validate_count MIN_RUNNERS "$MIN_RUNNERS"
validate_count MAX_RUNNERS "$MAX_RUNNERS"
validate_count GITHUB_APP_INSTALLATION_ID "$GITHUB_APP_INSTALLATION_ID"
[ "$GITHUB_APP_INSTALLATION_ID" -gt 0 ] || fail "GITHUB_APP_INSTALLATION_ID must be positive"
# The listener explicitly treats max=0 as unlimited.
if [ "$MAX_RUNNERS" -ne 0 ] && [ "$MIN_RUNNERS" -gt "$MAX_RUNNERS" ]; then
    fail "MIN_RUNNERS must not exceed MAX_RUNNERS (unless MAX_RUNNERS is 0)"
fi
if [ ! -f "$PRIVATE_KEY_FILE" ] || [ ! -r "$PRIVATE_KEY_FILE" ]; then
    fail "GitHub App private key must be a readable regular file: $PRIVATE_KEY_FILE"
fi
# A substitution inside `set --` hides cat's exit status, even with set -e.
if ! PRIVATE_KEY=$(cat "$PRIVATE_KEY_FILE"); then
    fail "Failed to read GitHub App private key: $PRIVATE_KEY_FILE"
fi
[ -n "$PRIVATE_KEY" ] || fail "GitHub App private key is empty: $PRIVATE_KEY_FILE"

set -- aktion-listener \
    --url "$GITHUB_CONFIG_URL" \
    --name "$SCALE_SET_NAME" \
    --min-runners "$MIN_RUNNERS" \
    --max-runners "$MAX_RUNNERS" \
    --app-client-id "$GITHUB_APP_CLIENT_ID" \
    --app-installation-id "$GITHUB_APP_INSTALLATION_ID" \
    --app-private-key "$PRIVATE_KEY" \
    --log-level "$LOG_LEVEL"

if [ -n "${LABELS:-}" ]; then
    set -- "$@" --labels "$LABELS"
fi

if [ -n "${RUNNER_IMAGE:-}" ]; then
    set -- "$@" --runner-image "$RUNNER_IMAGE"
fi

exec "$@"
