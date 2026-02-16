#!/usr/bin/env bash
set -euo pipefail

set -m

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

TOKFENCE_BINARY="${TOKFENCE_BINARY:-$REPO_ROOT/bin/tokfence}"
TOKFENCE_PROVIDER="${TOKFENCE_PROVIDER:-openai}"
TOKFENCE_HOST="${TOKFENCE_HOST:-127.0.0.1}"
TOKFENCE_PORT="${TOKFENCE_PORT:-9471}"
TOKFENCE_KEEP_DAEMON="${TOKFENCE_SMOKE_KEEP_DAEMON:-0}"
TOKFENCE_REMOVE_KEY="${TOKFENCE_SMOKE_REMOVE_KEY:-0}"
TOKFENCE_BUILD="${TOKFENCE_SMOKE_BUILD:-0}"
REQ_TIMEOUT="${REQ_TIMEOUT:-45}"

if ! command -v curl >/dev/null 2>&1; then
    echo "curl is required"
    exit 1
fi
if ! command -v python3 >/dev/null 2>&1; then
    echo "python3 is required for JSON validation"
    exit 1
fi

BASE_URL="http://$TOKFENCE_HOST:$TOKFENCE_PORT"
STARTED_DAEMON=0
REQUEST_ID=""
TMP_DIR="$(mktemp -d)"
LOG_FOLLOW_PID=""

trap cleanup EXIT

cleanup() {
    if [[ "$REQUEST_ID" != "" ]]; then
        :  # placeholder for optional future trace
    fi

    if [[ -n "${LOG_FOLLOW_PID:-}" ]]; then
        kill "$LOG_FOLLOW_PID" >/dev/null 2>&1 || true
        wait "$LOG_FOLLOW_PID" >/dev/null 2>&1 || true
    fi

    if [[ "$STARTED_DAEMON" -eq 1 && "$TOKFENCE_KEEP_DAEMON" != "1" ]]; then
        "$TOKFENCE_BINARY" stop --json >/dev/null 2>&1 || true
    fi

    if [[ "$TOKFENCE_REMOVE_KEY" == "1" ]]; then
        "$TOKFENCE_BINARY" vault remove "$TOKFENCE_PROVIDER" --json >/dev/null 2>&1 || true
    fi
    rm -rf "$TMP_DIR"
}

run_json() {
    "$TOKFENCE_BINARY" --json "$@"
}

json_dict_value() {
    local file="$1"
    local key="$2"
    python3 - "$file" "$key" <<'PY'
import json
import sys

path = sys.argv[1]
key = sys.argv[2]
with open(path, "r", encoding="utf-8") as handle:
    payload = json.load(handle)
if isinstance(payload, dict) and key in payload:
    value = payload[key]
    if isinstance(value, bool):
        print("true" if value else "false")
    else:
        print(value if value is not None else "")
else:
    raise SystemExit(1)
PY
}

require_provider_vars() {
    local provider="$1"
    case "$provider" in
        openai|anthropic)
            ;;
        *)
            echo "TOKFENCE_PROVIDER must be openai or anthropic"
            exit 1
            ;;
    esac
}

provider_env_key() {
    case "$1" in
        openai)
            echo "TOKFENCE_OPENAI_KEY"
            ;;
        anthropic)
            echo "TOKFENCE_ANTHROPIC_KEY"
            ;;
        *)
            echo ""
            ;;
    esac
}

read_key() {
    local env_name="$1"
    local raw="${!env_name-}"
    if [[ -z "$raw" ]]; then
        echo ""
        return
    fi
    printf '%s\n' "$raw"
}

daemon_running() {
    local status_file="$TMP_DIR/status.json"
    if ! run_json status > "$status_file" 2>/dev/null; then
        return 1
    fi
    local running
    if ! running="$(json_dict_value "$status_file" "running")"; then
        return 1
    fi
    [[ "$running" == "true" ]]
}

ensure_daemon() {
    if daemon_running; then
        return
    fi
    echo "starting tokfence daemon..."
    run_json start -d >/dev/null
    STARTED_DAEMON=1
    for _ in $(seq 1 60); do
        if daemon_running; then
            return
        fi
        sleep 0.25
    done
    echo "daemon did not become ready"
    exit 1
}

build_if_needed() {
    if [[ ! -x "$TOKFENCE_BINARY" || "$TOKFENCE_BUILD" == "1" ]]; then
        (cd "$REPO_ROOT" && go build -o "$TOKFENCE_BINARY" ./cmd/tokfence)
    fi
}

send_stream_request() {
    local provider="$1"
    local route="$2"
    local body_file="$TMP_DIR/request-body.json"
    local resp_file="$TMP_DIR/response.sse"
    local err_file="$TMP_DIR/request.err"
    local pid

    if [[ "$provider" == "anthropic" ]]; then
        cat <<'JSON' > "$body_file"
{"model":"claude-3-5-sonnet-20241022","max_tokens":64,"messages":[{"role":"user","content":"Say hello briefly."}],"stream":true}
JSON
    else
        cat <<'JSON' > "$body_file"
{"model":"gpt-4o-mini","messages":[{"role":"user","content":"Say hello briefly."}],"max_tokens":32,"stream":true}
JSON
    fi

    local start_ms
    start_ms="$(python3 - <<'PY'
import time
print(int(time.time() * 1000))
PY
)"
    local first_ms=""
    (
        curl -N -sS \
            --max-time "$REQ_TIMEOUT" \
            --connect-timeout 10 \
            -H "Content-Type: application/json" \
            -d "@$body_file" \
            -X POST \
            "$BASE_URL$route" \
            > "$resp_file" \
            2> "$err_file"
    ) &
    pid=$!

    local waited=0
    while (( waited < 120 )); do
        if [[ -s "$resp_file" ]]; then
            first_ms="$(python3 - <<'PY'
import time
print(int(time.time() * 1000))
PY
)"
            break
        fi
        sleep 0.05
        waited=$((waited + 1))
    done

    wait "$pid"
    local rc=$?
    if [[ $rc -ne 0 ]]; then
        echo "stream request failed (exit $rc)"
        if [[ -s "$err_file" ]]; then
            cat "$err_file"
        fi
        exit 1
    fi

    if [[ "$first_ms" == "" ]]; then
        echo "no streaming data was received within 6s"
        exit 1
    fi
    if ! grep -q 'data:' "$resp_file"; then
        echo "response does not contain SSE data events"
        head -n 20 "$resp_file"
        exit 1
    fi

    local ttfb
    ttfb=$((first_ms - start_ms))
    echo "SSE first chunk latency: ${ttfb}ms"
}

validate_log_record() {
    local provider="$1"
    local logs_file="$TMP_DIR/logs.json"
    run_json log --provider "$provider" --since 1h > "$logs_file"
    python3 - "$provider" "$logs_file" <<'PY'
import json
import sys

provider = sys.argv[1]
path = sys.argv[2]
with open(path, "r", encoding="utf-8") as handle:
    rows = json.load(handle)
if not isinstance(rows, list):
    raise SystemExit(1)
for row in rows:
    if not isinstance(row, dict):
        continue
    if row.get("provider") != provider:
        continue
    status = int(row.get("status_code", 0))
    if status < 200 or status >= 300:
        continue
    if not row.get("is_streaming", False):
        continue
    print(row.get("id", ""))
    print(row.get("estimated_cost_cents", 0))
    raise SystemExit(0)
raise SystemExit(1)
PY
}

validate_stats() {
    local provider="$1"
    local stats_file="$TMP_DIR/stats.json"
    run_json stats --period 1h --by provider > "$stats_file"
    python3 - "$provider" "$stats_file" <<'PY'
import json
import sys

provider = sys.argv[1]
path = sys.argv[2]
with open(path, "r", encoding="utf-8") as handle:
    rows = json.load(handle)
if not isinstance(rows, list):
    raise SystemExit(1)
for row in rows:
    if not isinstance(row, dict):
        continue
    if row.get("group") != provider:
        continue
    req = int(row.get("request_count", 0))
    cost = int(row.get("estimated_cost_cents", 0))
    if req <= 0:
        raise SystemExit(1)
    print(req)
    print(cost)
    raise SystemExit(0)
raise SystemExit(1)
PY
}

validate_snapshot() {
    local snapshot_file="$TMP_DIR/widget.json"
    run_json widget render > "$snapshot_file"
    python3 "$snapshot_file" <<'PY'
import json
import sys

with open(sys.argv[1], "r", encoding="utf-8") as handle:
    snapshot = json.load(handle)
if not isinstance(snapshot, dict):
    raise SystemExit(1)
if not bool(snapshot.get("running", False)):
    raise SystemExit(1)
if int(snapshot.get("today_requests", -1)) < 1:
    raise SystemExit(1)
print(int(snapshot.get("today_requests", 0)))
print(int(snapshot.get("today_cost_cents", 0)))
PY
}

start_log_follow() {
    local provider="$1"
    local logs_file="$2"
    run_json log --json -f --provider "$provider" --since 0s > "$logs_file" 2>/dev/null &
    LOG_FOLLOW_PID=$!
}

stop_log_follow() {
    if [[ -n "${LOG_FOLLOW_PID:-}" ]]; then
        kill "$LOG_FOLLOW_PID" >/dev/null 2>&1 || true
        wait "$LOG_FOLLOW_PID" >/dev/null 2>&1 || true
        LOG_FOLLOW_PID=""
    fi
}

validate_log_follow() {
    local provider="$1"
    local logs_file="$2"
    python3 - "$provider" "$logs_file" <<'PY'
import json
import sys

provider = sys.argv[1]
path = sys.argv[2]
with open(path, "r", encoding="utf-8") as handle:
    for raw in handle:
        raw = raw.strip()
        if not raw:
            continue
        try:
            row = json.loads(raw)
        except Exception:
            continue
        if not isinstance(row, dict):
            continue
        if row.get("provider") != provider:
            continue
        status_code = int(row.get("status_code", 0))
        if status_code < 200 or status_code >= 300:
            continue
        print(row.get("id", ""))
        print(row.get("estimated_cost_cents", 0))
        raise SystemExit(0)
raise SystemExit(1)
PY
}

main() {
    require_provider_vars "$TOKFENCE_PROVIDER"
    build_if_needed

    local provider_key_env
    local key
    provider_key_env="$(provider_env_key "$TOKFENCE_PROVIDER")"
    key="$(read_key "$provider_key_env")"

    if [[ -z "$key" ]]; then
        echo "Missing key: set ${provider_key_env} for provider ${TOKFENCE_PROVIDER}"
        exit 1
    fi

    echo "Using tokfence binary: $TOKFENCE_BINARY"
    echo "Using provider: $TOKFENCE_PROVIDER"
    ensure_daemon

    echo "$key" | run_json vault add "$TOKFENCE_PROVIDER" -
    FOLLOW_LOGS_FILE="$TMP_DIR/follow.log.jsonl"
    start_log_follow "$TOKFENCE_PROVIDER" "$FOLLOW_LOGS_FILE"

    local route
    if [[ "$TOKFENCE_PROVIDER" == "anthropic" ]]; then
        route="/anthropic/v1/messages"
    else
        route="/openai/v1/chat/completions"
    fi
    echo "send request -> $BASE_URL$route"
    send_stream_request "$TOKFENCE_PROVIDER" "$route"
    stop_log_follow

    echo "validate follow logs..."
    local follow_report
    if ! follow_report="$(validate_log_follow "$TOKFENCE_PROVIDER" "$FOLLOW_LOGS_FILE")"; then
        echo "live follow log validation failed"
        exit 1
    fi
    local follow_id
    local follow_cost
    follow_id="$(printf '%s\n' "$follow_report" | sed -n 1p)"
    follow_cost="$(printf '%s\n' "$follow_report" | sed -n 2p)"
    echo "follow log id: $follow_id, estimated_cost_cents=$follow_cost"
    if [[ -z "$follow_id" ]]; then
        echo "follow log validation did not return request id"
        exit 1
    fi

    echo "validate logs..."
    local log_report
    if ! log_report="$(validate_log_record "$TOKFENCE_PROVIDER")"; then
        echo "no matching streaming success log found"
        exit 1
    fi
    REQUEST_ID="$(printf '%s\n' "$log_report" | sed -n 1p)"
    local log_cost
    log_cost="$(printf '%s\n' "$log_report" | sed -n 2p)"
    echo "log id: $REQUEST_ID, estimated_cost_cents=$log_cost"
    if [[ -z "$REQUEST_ID" ]]; then
        echo "log validation did not return request id"
        exit 1
    fi

    echo "validate stats..."
    if ! validate_stats "$TOKFENCE_PROVIDER" >/dev/null; then
        echo "stats validation failed"
        exit 1
    fi

    echo "validate widget snapshot..."
    if ! validate_snapshot >/dev/null; then
        echo "snapshot validation failed"
        exit 1
    fi

    echo "TOKFENCE LIVE E2E OK (request_id=$REQUEST_ID)"
}

main
