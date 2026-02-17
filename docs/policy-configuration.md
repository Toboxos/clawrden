# Policy Configuration Guide

The `policy.yaml` file controls what commands agents can execute and from which directories they can operate.

## Structure

```yaml
default_action: deny

allowed_paths:
  - "/app/*"
  - "/tmp/*"

rules:
  - command: echo
    action: allow

  - command: npm
    action: ask

  - command: rm
    action: deny
    args:
      - "-rf"
```

## Actions

### `allow`
Command executes immediately without human approval.

**Use for:** Safe, read-only commands.

```yaml
- command: ls
  action: allow
- command: cat
  action: allow
- command: grep
  action: allow
```

### `deny`
Command is blocked immediately.

**Use for:** Dangerous or forbidden commands.

```yaml
- command: sudo
  action: deny
- command: rm
  action: deny
  args:
    - "-rf"
```

### `ask`
Command waits for human approval (HITL - Human-In-The-Loop).

**Use for:** Potentially risky commands that need oversight.

```yaml
- command: npm
  action: ask
- command: git
  action: ask
  args:
    - "push"
```

## Path Restrictions

The `allowed_paths` field restricts which directories commands can run from.

### Glob Patterns

Clawrden supports glob patterns for flexible path matching:

```yaml
allowed_paths:
  # Allow anything under /app
  - "/app/*"

  # Allow /tmp for testing
  - "/tmp/*"

  # Allow user workspace directories
  - "/home/*/workspace/*"

  # Allow specific app data
  - "/var/lib/myapp/*"

  # Multiple patterns can be specified
  - "/opt/data/*"
  - "/usr/local/share/myapp/*"
```

### Pattern Syntax

| Pattern | Matches | Example |
|---------|---------|---------|
| `/app/*` | Anything under /app | `/app/test`, `/app/sub/dir` |
| `/home/*/workspace/*` | User workspaces | `/home/alice/workspace/project` |
| `/var/lib/*/data/*` | App data directories | `/var/lib/myapp/data/file.txt` |
| `/app` | Exact path only | `/app` (not `/app/sub`) |

**Important:** Patterns ending with `/*` match all subdirectories recursively.

### Security Features

**Path Traversal Protection:**
```yaml
allowed_paths:
  - "/app/*"

# This request would be blocked:
# cwd: "/app/../etc/passwd"
# Resolved to: "/etc/passwd" (outside /app)
```

**Normalization:**
- Trailing slashes removed: `/app/` → `/app`
- Relative paths resolved: `/app/./sub` → `/app/sub`
- Parent references resolved: `/app/sub/../file` → `/app/file`

### Default Behavior

If `allowed_paths` is not specified, defaults to:
```yaml
allowed_paths:
  - "/app/*"
  - "/tmp/*"
```

If `allowed_paths` is an empty array, **all paths are allowed** (not recommended for production).

## Command Rules

### Basic Rule

```yaml
- command: echo
  action: allow
```

### With Argument Matching

```yaml
- command: git
  action: allow

- command: git
  action: ask
  args:
    - "push"

- command: git
  action: deny
  args:
    - "push --force"
    - "push -f"
```

**How it works:**
- If args match a pattern, that rule applies
- If no args patterns match, command-only rules apply
- First matching rule wins

### Wildcard Commands

```yaml
# Allow all commands starting with "test-"
- command: "test-*"
  action: allow
```

## Complete Example

```yaml
default_action: deny

# Restrict operations to specific directories
allowed_paths:
  - "/app/*"                    # Main application directory
  - "/tmp/*"                    # Temporary files
  - "/home/agent/workspace/*"   # Agent workspace
  - "/var/cache/myapp/*"        # Cache directory

rules:
  # Safe read-only commands - auto allow
  - command: ls
    action: allow
  - command: cat
    action: allow
  - command: head
    action: allow
  - command: tail
    action: allow
  - command: grep
    action: allow
  - command: find
    action: allow
  - command: echo
    action: allow
  - command: pwd
    action: allow

  # Version control - allow reads, ask for writes
  - command: git
    action: allow
    args:
      - "status"
      - "diff"
      - "log"
      - "show"

  - command: git
    action: ask
    args:
      - "commit"
      - "push"
      - "pull"

  - command: git
    action: deny
    args:
      - "push --force"
      - "push -f"
      - "reset --hard"

  # Package managers - require approval
  - command: npm
    action: ask
  - command: pip
    action: ask
  - command: yarn
    action: ask

  # Dangerous commands - always deny
  - command: sudo
    action: deny
  - command: su
    action: deny
  - command: rm
    action: deny
    args:
      - "-rf"
      - "-fr"
  - command: chmod
    action: deny
    args:
      - "777"
  - command: chown
    action: deny

  # Infrastructure tools - require approval
  - command: docker
    action: ask
  - command: kubectl
    action: ask
  - command: terraform
    action: ask
```

## Multi-Environment Setup

### Development (Permissive)

```yaml
# dev-policy.yaml
default_action: allow

allowed_paths:
  - "/app/*"
  - "/tmp/*"
  - "/home/*"

rules:
  # Only block obviously dangerous commands
  - command: sudo
    action: deny
  - command: rm
    action: ask
    args:
      - "-rf"
```

### Staging (Moderate)

```yaml
# staging-policy.yaml
default_action: deny

allowed_paths:
  - "/app/*"
  - "/tmp/*"

rules:
  # Auto-allow safe commands
  - command: ls
    action: allow
  - command: cat
    action: allow

  # Ask for writes
  - command: npm
    action: ask
  - command: git
    action: ask
    args:
      - "push"
```

### Production (Strict)

```yaml
# prod-policy.yaml
default_action: deny

# Very restricted paths
allowed_paths:
  - "/app/logs/*"
  - "/app/data/*"

rules:
  # Only allow minimal safe commands
  - command: ls
    action: allow
  - command: cat
    action: allow

  # Everything else requires approval
  # (default_action: deny)
```

## Testing Your Policy

### Dry Run Mode

Start warden with audit-only mode (future feature):
```bash
./bin/clawrden-warden \
  --policy policy.yaml \
  --audit audit.log \
  --dry-run
```

### Validate Policy File

```bash
# Check syntax
cat policy.yaml | yq .

# Test path matching
./bin/clawrden-cli test-policy --path /app/test
./bin/clawrden-cli test-policy --command npm --args install
```

## Best Practices

### 1. Start Restrictive
Begin with `default_action: deny` and explicitly allow what's needed.

### 2. Use Specific Paths
```yaml
# Good
allowed_paths:
  - "/app/workspace/*"
  - "/app/logs/*"

# Too broad
allowed_paths:
  - "/*"
```

### 3. Layer Your Rules
```yaml
# Specific rules first
- command: git
  action: deny
  args:
    - "push --force"

# General rules after
- command: git
  action: ask
```

### 4. Document Your Rules
```yaml
# Safe read-only operations for log analysis
- command: grep
  action: allow

# Package installation requires human oversight
- command: npm
  action: ask
```

### 5. Test Path Patterns
```bash
# Verify your patterns work as expected
echo "/app/test" | grep -E "^/app/"
echo "/home/user/workspace/file" | grep -E "^/home/.*/workspace/"
```

## Troubleshooting

### Commands Getting Blocked

Check the audit log:
```bash
./bin/clawrden-cli history | grep denied
```

View in dashboard: `http://localhost:8080`

### Path Rejections

```bash
# Check which paths are allowed
grep allowed_paths policy.yaml

# Test a specific path
# Add to policy:
allowed_paths:
  - "/your/path/*"
```

### Rule Not Matching

Rules are evaluated in order. First match wins:
```yaml
# This rule will never match because "git" already matched
- command: git
  action: allow

- command: git  # ← Never reached!
  action: deny
  args:
    - "push --force"

# Fix: Put specific rules first
- command: git
  action: deny
  args:
    - "push --force"

- command: git
  action: allow
```

## Advanced Patterns

### Multi-User Workspaces
```yaml
allowed_paths:
  - "/home/*/agent-workspace/*"
```

### Dynamic App Directories
```yaml
allowed_paths:
  - "/var/lib/*/data/*"
  - "/opt/*/workspace/*"
```

### Combination Patterns
```yaml
allowed_paths:
  - "/app/*"                    # App directory
  - "/tmp/agent-*/*"            # Agent temp dirs
  - "/home/*/workspace/safe/*"  # User safe zones
```

---

**Remember:** Good policy configuration is a balance between security and usability. Start strict and gradually allow what's necessary based on actual usage patterns from the audit log.
