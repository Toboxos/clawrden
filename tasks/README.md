# Tasks Directory

**Purpose:** Track active and completed development tasks for Clawrden.

**Status:** This directory is gitignored - tasks are local work-in-progress notes.

## Workflow

1. **Create Task** - Copy `000-template.md` and rename to `<id>-<description>.md`
2. **Work & Document** - Update progress log as you implement
3. **Mark Done** - Change status to DONE when complete (don't delete!)
4. **Keep History** - Completed tasks provide context for future work

## Naming Convention

```
<id>-<kebab-case-description>.md

Examples:
001-timeout-enforcement.md
002-docker-integration-tests.md
010-jailhouse-implementation.md
```

## Task Statuses

- **TODO** - Planned but not started
- **IN_PROGRESS** - Actively being worked on
- **BLOCKED** - Waiting on external dependency or decision
- **DONE** - Completed (keep file for historical context)

## Quick Commands

```bash
# List all tasks
ls -la tasks/

# Find active tasks
grep -l "Status: IN_PROGRESS" tasks/*.md

# Find blocked tasks
grep -l "Status: BLOCKED" tasks/*.md

# Find completed tasks
grep -l "Status: DONE" tasks/*.md

# Search for specific topic
grep -r "timeout" tasks/
```

## Template Usage

```bash
# Create new task
cp tasks/000-template.md tasks/042-my-new-feature.md

# Edit and update status
vim tasks/042-my-new-feature.md
# Change Status: TODO → IN_PROGRESS
```

## Best Practices

1. **One task per file** - Keep focused and atomic
2. **Update frequently** - Document as you work, not after
3. **Be specific** - Include file paths, line numbers, commit hashes
4. **Track blockers** - Document why you're stuck
5. **Record decisions** - Explain trade-offs and rationale
6. **Don't delete done tasks** - They're valuable historical context

## Integration with ROADMAP.md

- Tasks should reference their ROADMAP.md section
- Use tasks for detailed implementation tracking
- ROADMAP is high-level strategy, tasks are execution details

## Example Task Structure

See `000-template.md` for the recommended structure. Key sections:

- **Header** - Metadata (status, priority, assignee, dates)
- **Objective** - What needs to be accomplished
- **Progress Log** - Chronological updates with checklist
- **Implementation Details** - Files, patterns, code snippets
- **Testing** - Test coverage and validation
- **Definition of Done** - Clear completion criteria

## Why This Approach?

✅ **Living Documentation** - Tasks evolve as work progresses
✅ **Searchable History** - Easy to find past decisions
✅ **Context Preservation** - Future work benefits from past notes
✅ **Progress Tracking** - Visual checklist of what's done
✅ **Local Only** - Work-in-progress notes don't clutter repo
