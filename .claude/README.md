# Ralph Wiggum - Autonomous Loops for Amanah

This directory contains the Ralph Wiggum hooks implementation for autonomous, long-running task execution loops in Claude Code.

## Overview

Ralph Wiggum turns Claude Code into a persistent loop. You give it a prompt, it works until done (or until you stop it). When Claude thinks it's done, the Stop hook intercepts the exit, re-feeds the original prompt, and Claude continues.

## Directory Structure

```
.claude/
├── commands/           # Slash commands
│   ├── ralph-loop.md   # /ralph-loop - Start autonomous loop
│   ├── cancel-ralph.md # /cancel-ralph - Stop active loop
│   └── ralph-status.md # /ralph-status - Check loop status
├── hooks/              # Hook scripts
│   ├── stop-hook.sh    # Prevents Claude from stopping during loop
│   └── user-prompt-submit-hook.sh  # Injects loop context
├── scripts/            # Utility scripts
│   └── ralph-state.sh  # State management utilities
├── settings.json       # Hook configuration
└── README.md           # This file
```

## Quick Start

### Start an Autonomous Loop

```
/ralph-loop "Your task description here" --max-iterations 20 --completion-promise "DONE"
```

**Arguments:**
- `"prompt"` - The task to work on (required)
- `--max-iterations <n>` - Safety limit on iterations (default: 10)
- `--completion-promise "<text>"` - Phrase that signals completion

### Check Status

```
/ralph-status
```

### Cancel a Loop

```
/cancel-ralph
```

## How It Works

1. `/ralph-loop` initializes state and starts working
2. When Claude attempts to stop, `stop-hook.sh` intercepts
3. The hook checks completion criteria:
   - Max iterations reached?
   - Completion promise found in output?
4. If not complete, hook blocks the stop and injects the original prompt
5. Claude continues working with context from previous iterations
6. Loop repeats until criteria met or cancelled

## Best Practices

### Writing Good Prompts

1. **Clear Completion Criteria**: Define exactly what "done" looks like
   ```
   /ralph-loop "Implement user CRUD API. Requirements:
   - POST /users - create user
   - GET /users/:id - get user
   - PUT /users/:id - update user
   - DELETE /users/:id - delete user
   - All endpoints have validation
   - Unit tests for each endpoint
   Output TASK_COMPLETE when all requirements are met." --completion-promise "TASK_COMPLETE" --max-iterations 30
   ```

2. **Incremental Goals**: Break complex tasks into phases
   ```
   /ralph-loop "Phase 1: Set up database models
   Phase 2: Implement API endpoints
   Phase 3: Add validation
   Phase 4: Write tests
   Complete each phase before moving to next.
   Output PHASE_COMPLETE after each phase." --max-iterations 50
   ```

3. **Self-Verification**: Include testing in the loop
   ```
   /ralph-loop "Implement feature X. After implementation:
   1. Run go test ./...
   2. Fix any failures
   3. Run go build ./...
   4. Fix any errors
   Repeat until all tests pass and build succeeds.
   Output BUILD_SUCCESS when complete." --completion-promise "BUILD_SUCCESS"
   ```

### Safety Mechanisms

- **Always set `--max-iterations`** - Prevents runaway loops
- **Use completion promises** - Clear exit conditions
- **Monitor progress** - Use `/ralph-status` periodically
- **Cancel when needed** - `/cancel-ralph` stops immediately

## State Storage

Loop state is stored in `~/.claude/ralph-state/`:
- `current-loop.json` - Active loop configuration
- `loop-history.log` - Historical log of all loops

## Troubleshooting

### Loop Not Starting
- Check that scripts are executable: `chmod +x .claude/hooks/*.sh .claude/scripts/*.sh`
- Verify settings.json is valid JSON

### Loop Not Stopping
- Ensure completion promise matches exactly (case-sensitive)
- Check max-iterations is set
- Use `/cancel-ralph` to force stop

### Hooks Not Running
- Verify absolute paths in settings.json
- Check script permissions
- Run with `claude --debug` to see hook execution

## Sources

- [Ralph Wiggum: Autonomous Loops for Claude Code](https://paddo.dev/blog/ralph-wiggum-autonomous-loops/)
- [Claude Code Hooks Reference](https://code.claude.com/docs/en/hooks)
- [GitHub: anthropics/claude-code ralph-wiggum plugin](https://github.com/anthropics/claude-code/tree/main/plugins/ralph-wiggum)
