#!/bin/bash
#
# Ralph Wiggum Stop Hook
# Prevents Claude from stopping during an active ralph-loop session
# Returns exit code 0 with decision=block to continue the loop
#

set -euo pipefail

# Configuration
RALPH_STATE_DIR="${HOME}/.claude/ralph-state"
RALPH_STATE_FILE="${RALPH_STATE_DIR}/current-loop.json"

# Read hook input from stdin
input=$(cat)

# Parse input
session_id=$(echo "$input" | jq -r '.session_id // ""')
cwd=$(echo "$input" | jq -r '.cwd // ""')
transcript_path=$(echo "$input" | jq -r '.transcript_path // ""')

# Check if ralph loop is active
if [[ ! -f "$RALPH_STATE_FILE" ]]; then
    # No active loop, allow normal stop
    exit 0
fi

# Read loop state
loop_state=$(cat "$RALPH_STATE_FILE")
loop_active=$(echo "$loop_state" | jq -r '.active // false')
loop_session=$(echo "$loop_state" | jq -r '.session_id // ""')
original_prompt=$(echo "$loop_state" | jq -r '.prompt // ""')
max_iterations=$(echo "$loop_state" | jq -r '.max_iterations // 0')
current_iteration=$(echo "$loop_state" | jq -r '.current_iteration // 0')
completion_promise=$(echo "$loop_state" | jq -r '.completion_promise // ""')

# Check if loop is active for this session
if [[ "$loop_active" != "true" ]]; then
    exit 0
fi

# Increment iteration counter
new_iteration=$((current_iteration + 1))

# Check if max iterations reached
if [[ "$max_iterations" -gt 0 ]] && [[ "$new_iteration" -gt "$max_iterations" ]]; then
    # Deactivate loop
    echo "$loop_state" | jq '.active = false | .end_reason = "max_iterations_reached"' > "$RALPH_STATE_FILE"

    cat << EOF
{
  "decision": "approve",
  "reason": "Ralph loop completed: maximum iterations ($max_iterations) reached.",
  "stopReason": "Ralph loop: max iterations reached"
}
EOF
    exit 0
fi

# Check for completion promise in transcript
if [[ -n "$completion_promise" ]] && [[ -f "$transcript_path" ]]; then
    if grep -q "$completion_promise" "$transcript_path" 2>/dev/null; then
        # Completion promise found, deactivate loop
        echo "$loop_state" | jq '.active = false | .end_reason = "completion_promise_found"' > "$RALPH_STATE_FILE"

        cat << EOF
{
  "decision": "approve",
  "reason": "Ralph loop completed: completion promise '$completion_promise' found.",
  "stopReason": "Ralph loop: task completed"
}
EOF
        exit 0
    fi
fi

# Update iteration counter
echo "$loop_state" | jq ".current_iteration = $new_iteration" > "$RALPH_STATE_FILE"

# Block stop and continue loop
cat << EOF
{
  "decision": "block",
  "reason": "Ralph loop active (iteration $new_iteration of ${max_iterations:-unlimited}). Continue working on: $original_prompt",
  "stopReason": "Ralph loop: continuing iteration $new_iteration",
  "systemMessage": "You are in an autonomous ralph-loop. Continue working on the task. Original prompt: $original_prompt. If you believe the task is complete, output the completion phrase or explicitly state completion."
}
EOF

exit 0
