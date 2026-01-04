# Ralph Loop - Autonomous Task Execution

You are initiating a Ralph Wiggum autonomous loop. This will run Claude in a continuous loop until the task is complete or max iterations are reached.

## Arguments Received
$ARGUMENTS

## Your Task

1. **Parse the arguments** to extract:
   - The main task/prompt (required)
   - `--max-iterations <n>` - Maximum iterations before stopping (default: 10)
   - `--completion-promise "<text>"` - Exact phrase that signals task completion

2. **Initialize the ralph loop** by running this command:
   ```bash
   /home/user/Amanah/.claude/scripts/ralph-state.sh init "<task_prompt>" <max_iterations> "<completion_promise>"
   ```

3. **Confirm loop activation** to the user with:
   - The task being worked on
   - Maximum iterations configured
   - Completion criteria (if any)
   - How to cancel: `/cancel-ralph`

4. **Begin working on the task immediately** after confirmation.

## Example Usage

```
/ralph-loop "Implement a complete REST API for user management with CRUD operations, input validation, and tests. Output TASK_COMPLETE when done." --max-iterations 20 --completion-promise "TASK_COMPLETE"
```

## Important Notes

- The Stop hook will prevent Claude from finishing until the task is complete
- Always set `--max-iterations` as a safety mechanism
- Include clear completion criteria in your prompt
- The loop persists file changes between iterations
- Use `/cancel-ralph` to stop the loop at any time

## Begin Now

Parse the arguments above and initialize the ralph loop. Then start working on the task.
