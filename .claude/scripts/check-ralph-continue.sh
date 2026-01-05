#!/bin/bash
#
# Check Ralph Loop Status and Provide Continuation Context
# Returns JSON with loop state and next actions
#

set -euo pipefail

RALPH_STATE_DIR="${HOME}/.claude/ralph-state"
RALPH_STATE_FILE="${RALPH_STATE_DIR}/current-loop.json"

# Check if loop is active
if [[ ! -f "$RALPH_STATE_FILE" ]]; then
    echo '{"active": false, "message": "No active ralph loop"}'
    exit 0
fi

# Read state
state=$(cat "$RALPH_STATE_FILE")
active=$(echo "$state" | jq -r '.active // false')

if [[ "$active" != "true" ]]; then
    end_reason=$(echo "$state" | jq -r '.end_reason // "unknown"')
    echo "{\"active\": false, \"message\": \"Loop ended: $end_reason\"}"
    exit 0
fi

# Get loop info
prompt=$(echo "$state" | jq -r '.prompt // ""')
iteration=$(echo "$state" | jq -r '.current_iteration // 0')
max=$(echo "$state" | jq -r '.max_iterations // 0')
promise=$(echo "$state" | jq -r '.completion_promise // ""')

# Increment iteration
new_iteration=$((iteration + 1))
echo "$state" | jq ".current_iteration = $new_iteration" > "$RALPH_STATE_FILE"

# Check max iterations
if [[ "$max" -gt 0 ]] && [[ "$new_iteration" -gt "$max" ]]; then
    echo "$state" | jq '.active = false | .end_reason = "max_iterations"' > "$RALPH_STATE_FILE"
    echo "{\"active\": false, \"message\": \"Max iterations ($max) reached\", \"completed\": true}"
    exit 0
fi

# Return continuation context
cat << EOF
{
  "active": true,
  "iteration": $new_iteration,
  "max_iterations": $max,
  "prompt": $(echo "$prompt" | jq -Rs .),
  "completion_promise": "$promise",
  "message": "Continue working on the task. Iteration $new_iteration of ${max:-unlimited}."
}
EOF
