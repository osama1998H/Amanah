#!/bin/bash
#
# Ralph Wiggum State Management
# Provides utilities for managing ralph-loop state
#

set -euo pipefail

# Configuration
RALPH_STATE_DIR="${HOME}/.claude/ralph-state"
RALPH_STATE_FILE="${RALPH_STATE_DIR}/current-loop.json"
RALPH_LOG_FILE="${RALPH_STATE_DIR}/loop-history.log"

# Ensure state directory exists
mkdir -p "$RALPH_STATE_DIR"

# Functions
ralph_init_loop() {
    local prompt="$1"
    local max_iterations="${2:-0}"
    local completion_promise="${3:-}"
    local session_id="${4:-$(date +%s)}"

    cat > "$RALPH_STATE_FILE" << EOF
{
  "active": true,
  "session_id": "$session_id",
  "prompt": $(echo "$prompt" | jq -Rs .),
  "max_iterations": $max_iterations,
  "current_iteration": 0,
  "completion_promise": "$completion_promise",
  "start_time": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "end_time": null,
  "end_reason": null
}
EOF

    echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] STARTED: $prompt (max: $max_iterations)" >> "$RALPH_LOG_FILE"
}

ralph_cancel_loop() {
    local reason="${1:-user_cancelled}"

    if [[ -f "$RALPH_STATE_FILE" ]]; then
        local state=$(cat "$RALPH_STATE_FILE")
        local prompt=$(echo "$state" | jq -r '.prompt // "unknown"')
        local iterations=$(echo "$state" | jq -r '.current_iteration // 0')

        echo "$state" | jq ".active = false | .end_time = \"$(date -u +"%Y-%m-%dT%H:%M:%SZ")\" | .end_reason = \"$reason\"" > "$RALPH_STATE_FILE"

        echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] CANCELLED ($reason): $prompt after $iterations iterations" >> "$RALPH_LOG_FILE"

        echo "Ralph loop cancelled after $iterations iterations. Reason: $reason"
    else
        echo "No active ralph loop to cancel."
    fi
}

ralph_status() {
    if [[ ! -f "$RALPH_STATE_FILE" ]]; then
        echo "No ralph loop state found."
        return 1
    fi

    local state=$(cat "$RALPH_STATE_FILE")
    local active=$(echo "$state" | jq -r '.active // false')

    if [[ "$active" == "true" ]]; then
        local prompt=$(echo "$state" | jq -r '.prompt // "unknown"')
        local iteration=$(echo "$state" | jq -r '.current_iteration // 0')
        local max=$(echo "$state" | jq -r '.max_iterations // 0')
        local start=$(echo "$state" | jq -r '.start_time // "unknown"')

        echo "Ralph Loop Status: ACTIVE"
        echo "  Prompt: $prompt"
        echo "  Iteration: $iteration of ${max:-unlimited}"
        echo "  Started: $start"
    else
        local end_reason=$(echo "$state" | jq -r '.end_reason // "unknown"')
        echo "Ralph Loop Status: INACTIVE (ended: $end_reason)"
    fi
}

ralph_get_prompt() {
    if [[ -f "$RALPH_STATE_FILE" ]]; then
        jq -r '.prompt // ""' "$RALPH_STATE_FILE"
    fi
}

ralph_is_active() {
    if [[ -f "$RALPH_STATE_FILE" ]]; then
        local active=$(jq -r '.active // false' "$RALPH_STATE_FILE")
        [[ "$active" == "true" ]]
    else
        return 1
    fi
}

# Command dispatch
case "${1:-status}" in
    init)
        ralph_init_loop "${2:-}" "${3:-0}" "${4:-}" "${5:-}"
        ;;
    cancel)
        ralph_cancel_loop "${2:-user_cancelled}"
        ;;
    status)
        ralph_status
        ;;
    prompt)
        ralph_get_prompt
        ;;
    is-active)
        ralph_is_active && echo "true" || echo "false"
        ;;
    *)
        echo "Usage: $0 {init|cancel|status|prompt|is-active}"
        exit 1
        ;;
esac
