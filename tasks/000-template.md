# Task: [Title]

**Status:** TODO | IN_PROGRESS | BLOCKED | DONE
**Priority:** HIGH | MEDIUM | LOW
**Assignee:** [name or "claude"]
**Created:** YYYY-MM-DD
**Completed:** YYYY-MM-DD (if done)
**Related:** [Link to ROADMAP.md section, issue #, or other task]

## Objective

[Clear, concise description of what needs to be accomplished]

## Context

[Why this is needed, background information, links to related work]

## Approach

[High-level implementation strategy or technical approach]

## Progress Log

### YYYY-MM-DD - Initial Planning
- [ ] Subtask 1
- [ ] Subtask 2
- [ ] Subtask 3

**Notes:**
- Decision made: [reasoning]
- Blocker identified: [description]

### YYYY-MM-DD - Implementation
- [x] Completed subtask 1
- [x] Completed subtask 2
- [ ] Still working on subtask 3

**Notes:**
- Files modified: `path/to/file.go:123`
- Discovered: [insight or issue]

### YYYY-MM-DD - Completion
- [x] All subtasks complete
- [x] Tests passing
- [x] Documentation updated

**Notes:**
- Final commit: [commit hash]
- Lessons learned: [reflection]

## Implementation Details

### Files Modified
- `internal/path/to/file.go` - Added timeout handling
- `tests/integration/timeout_test.go` - New test cases
- `docs/configuration.md` - Updated timeout documentation

### Key Changes
- Added `Timeout` field to `Rule` struct
- Implemented `context.WithTimeout` in executor
- Updated audit log to track timeout violations

### Code Patterns Used
```go
// Example of pattern used
ctx, cancel := context.WithTimeout(ctx, rule.Timeout)
defer cancel()
```

## Testing

- [ ] Unit tests added
- [ ] Integration tests added/updated
- [ ] Manual testing completed
- [ ] All tests passing (22/22 or updated count)

### Test Cases
1. Command completes within timeout → success
2. Command exceeds timeout → proper error
3. No timeout configured → uses default

## Definition of Done

- [ ] Code implemented and reviewed
- [ ] Tests written and passing
- [ ] Documentation updated
- [ ] No regressions (all existing tests pass)
- [ ] Commit message references this task
- [ ] Task marked as DONE

## Notes / Open Questions

- [Any questions that came up during implementation]
- [Technical decisions that need validation]
- [Follow-up work identified]

## Related Resources

- ROADMAP.md: Phase X, Step Y
- Documentation: `docs/some-doc.md`
- Similar work: `tasks/other-task.md`
