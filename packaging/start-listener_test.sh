#!/bin/sh
# Run with a POSIX shell; no Docker daemon, network, or real credentials needed.
set -eu
script="$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)/start-listener.sh"
test_dir=$(mktemp -d)
trap 'rm -rf "$test_dir"' EXIT HUP INT TERM
mkdir "$test_dir/bin" "$test_dir/failing-cat"
cat > "$test_dir/bin/aktion-listener" <<'MOCK'
#!/bin/sh
set -eu
while [ "$#" -gt 0 ]; do
    case "$1" in
        --url) [ "$2" = "$GITHUB_CONFIG_URL" ] ;;
        --name) [ "$2" = "$SCALE_SET_NAME" ] ;;
        --min-runners) [ "$2" = "${MIN_RUNNERS:-0}" ] ;;
        --max-runners) [ "$2" = "${MAX_RUNNERS:-1}" ] ;;
        --app-client-id) [ "$2" = "$GITHUB_APP_CLIENT_ID" ] ;;
        --app-installation-id) [ "$2" = "$GITHUB_APP_INSTALLATION_ID" ] ;;
        --app-private-key) [ "$2" = "$EXPECTED_KEY" ] ;;
        --log-level) [ "$2" = "${LOG_LEVEL:-info}" ] ;;
        --labels) [ "$2" = "$LABELS" ] ;;
        --runner-image) [ "$2" = "$RUNNER_IMAGE" ] ;;
        *) exit 99 ;;
    esac
    shift 2
done
echo STARTED
MOCK
cat > "$test_dir/failing-cat/cat" <<'MOCK'
#!/bin/sh
printf 'partial key'
exit 1
MOCK
chmod +x "$test_dir/bin/aktion-listener" "$test_dir/failing-cat/cat"
export PATH="$test_dir/bin:$PATH"
export GITHUB_CONFIG_URL=https://github.com/example/repo SCALE_SET_NAME='test runner'
export GITHUB_APP_CLIENT_ID=test GITHUB_APP_INSTALLATION_ID=1
export MIN_RUNNERS=0 MAX_RUNNERS=1 LOG_LEVEL=info LABELS='' RUNNER_IMAGE=''
export GITHUB_APP_PRIVATE_KEY_FILE="$test_dir/key.pem"
export EXPECTED_KEY='fake key first line
fake key second line'
printf '%s\n' "$EXPECTED_KEY" > "$GITHUB_APP_PRIVATE_KEY_FILE"
: > "$test_dir/empty.pem"

expect_success() {
    "$@" > "$test_dir/output" 2>&1
    grep -q '^STARTED$' "$test_dir/output"
}
expect_failure() {
    expected=$1
    shift
    if "$@" > "$test_dir/output" 2>&1; then
        echo "Unexpected success: $expected" >&2
        exit 1
    fi
    if grep -q STARTED "$test_dir/output" || ! grep -q "$expected" "$test_dir/output"; then
        cat "$test_dir/output" >&2
        exit 1
    fi
}

expect_success sh "$script"
expect_success env MIN_RUNNERS=3 MAX_RUNNERS=0 LOG_LEVEL=DEBUG LABELS='linux,custom label' RUNNER_IMAGE='local/runner:test' sh "$script"
expect_failure GITHUB_CONFIG_URL env GITHUB_CONFIG_URL='' sh "$script"
expect_failure 'regular file' env GITHUB_APP_PRIVATE_KEY_FILE="$test_dir" sh "$script"
expect_failure 'regular file' env GITHUB_APP_PRIVATE_KEY_FILE="$test_dir/missing.pem" sh "$script"
expect_failure 'key is empty' env GITHUB_APP_PRIVATE_KEY_FILE="$test_dir/empty.pem" sh "$script"
expect_failure 'Failed to read' env PATH="$test_dir/failing-cat:$PATH" sh "$script"
for value in -1 invalid 08 99999999999999999999999999999; do
    expect_failure MIN_RUNNERS env MIN_RUNNERS="$value" sh "$script"
    expect_failure MAX_RUNNERS env MAX_RUNNERS="$value" sh "$script"
done
expect_failure 'must not exceed' env MIN_RUNNERS=2 MAX_RUNNERS=1 sh "$script"
expect_failure 'must be positive' env GITHUB_APP_INSTALLATION_ID=0 sh "$script"
echo 'PASS: listener startup regression tests'
