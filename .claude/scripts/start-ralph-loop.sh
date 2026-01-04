#!/bin/bash
#
# Start Ralph Loop - Direct invocation script
# Usage: ./start-ralph-loop.sh "<prompt>" [max_iterations] [completion_promise]
#

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RALPH_STATE_SCRIPT="${SCRIPT_DIR}/ralph-state.sh"

# Arguments
PROMPT="${1:-}"
MAX_ITERATIONS="${2:-10}"
COMPLETION_PROMISE="${3:-}"

if [[ -z "$PROMPT" ]]; then
    echo "Error: Prompt is required"
    echo "Usage: $0 \"<prompt>\" [max_iterations] [completion_promise]"
    exit 1
fi

# Initialize the loop
"$RALPH_STATE_SCRIPT" init "$PROMPT" "$MAX_ITERATIONS" "$COMPLETION_PROMISE"

echo "=========================================="
echo "Ralph Loop Activated"
echo "=========================================="
echo "Task: $PROMPT"
echo "Max Iterations: $MAX_ITERATIONS"
if [[ -n "$COMPLETION_PROMISE" ]]; then
    echo "Completion Promise: $COMPLETION_PROMISE"
fi
echo ""
echo "To cancel: Run .claude/scripts/ralph-state.sh cancel"
echo "To check status: Run .claude/scripts/ralph-state.sh status"
echo "=========================================="
