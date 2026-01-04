#!/bin/bash
#
# Ralph Wiggum User Prompt Submit Hook
# Injects /continue command when in an active ralph-loop
# This maintains the autonomous loop between iterations
#

set -euo pipefail

# Configuration
RALPH_STATE_DIR="${HOME}/.claude/ralph-state"
RALPH_STATE_FILE="${RALPH_STATE_DIR}/current-loop.json"

# Read hook input from stdin
input=$(cat)

# Parse input
session_id=$(echo "$input" | jq -r '.session_id // ""')
prompt=$(echo "$input" | jq -r '.tool_input.prompt // ""')

# Check for ralph-loop command initiation
if [[ "$prompt" =~ ^/ralph-loop ]]; then
    # Let the command handler process this
    exit 0
fi

# Check for cancel-ralph command
if [[ "$prompt" =~ ^/cancel-ralph ]]; then
    # Let the command handler process this
    exit 0
fi

# Check if ralph loop is active
if [[ ! -f "$RALPH_STATE_FILE" ]]; then
    exit 0
fi

# Read loop state
loop_state=$(cat "$RALPH_STATE_FILE")
loop_active=$(echo "$loop_state" | jq -r '.active // false')
original_prompt=$(echo "$loop_state" | jq -r '.prompt // ""')
current_iteration=$(echo "$loop_state" | jq -r '.current_iteration // 0')

# If loop is active, inject context
if [[ "$loop_active" == "true" ]]; then
    cat << EOF
{
  "hookSpecificOutput": {
    "hookEventName": "UserPromptSubmit",
    "additionalContext": "Ralph loop iteration $current_iteration active. Original task: $original_prompt. Continue making progress on this task."
  }
}
EOF
fi

exit 0
