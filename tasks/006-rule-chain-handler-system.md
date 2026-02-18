# Task: Rule Chain & Handler System Architecture

**Status:** IN_PROGRESS
**Priority:** HIGH
**Assignee:** claude
**Created:** 2026-02-17
**Phase:** 4 (Architecture Evolution)

## Objective

Replace the monolithic `config.yaml` policy system with a flexible, extensible rule chain architecture inspired by Linux netfilter/iptables. Enable fine-grained command processing with pluggable handlers that can inspect, modify, and control command execution.

## Context

**Current Limitations:**
- Single `policy.yaml` with flat rule structure
- Limited action types (allow/deny/ask)
- No command modification capabilities
- Hard to extend without code changes
- Future plugin system needs better foundation

**Inspiration:**
- **Linux iptables**: Rule chains with match criteria and targets
- **Linux IP routing**: Priority-based rule evaluation
- **Nginx request processing**: Handler chain with phases
- **Envoy filters**: Pluggable filter chains

## Architecture Vision

### Rule Chain Model

```
┌─────────────────────────────────────────────────────────────┐
│                    Command Interception                      │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                   PRE-PROCESSING CHAIN                       │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐             │
│  │ Logger   │ -> │ Sanitizer│ -> │ Enricher │             │
│  │ Handler  │    │ Handler  │    │ Handler  │             │
│  └──────────┘    └──────────┘    └──────────┘             │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                      POLICY CHAIN                            │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐             │
│  │ Path     │ -> │ Pattern  │ -> │ Risk     │             │
│  │ Validator│    │ Matcher  │    │ Assessor │             │
│  └────┬─────┘    └────┬─────┘    └────┬─────┘             │
│       │ DENY          │ MODIFY         │ ASK               │
└───────┼───────────────┼────────────────┼────────────────────┘
        │               │                │
        │               ▼                │
        │    ┌─────────────────────┐    │
        │    │  MODIFICATION CHAIN  │    │
        │    │  ┌────────────────┐ │    │
        │    │  │ Arg Rewriter   │ │    │
        │    │  │ Env Injector   │ │    │
        │    │  │ Timeout Setter │ │    │
        │    │  └────────────────┘ │    │
        │    └──────────┬──────────┘    │
        │               │                │
        ▼               ▼                ▼
┌─────────────────────────────────────────────────────────────┐
│                   POST-PROCESSING CHAIN                      │
│  ┌──────────┐    ┌──────────┐    ┌──────────┐             │
│  │ Audit    │ -> │ Metrics  │ -> │ Notifier │             │
│  │ Logger   │    │ Collector│    │ Handler  │             │
│  └──────────┘    └──────────┘    └──────────┘             │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                      EXECUTION                               │
│              (Mirror / Local / Ghost)                        │
└─────────────────────────────────────────────────────────────┘
```

### Core Concepts

#### 1. **Rule**
A declarative match condition + action

```yaml
# rules.d/npm-security.yaml
rules:
  - name: "Block npm scripts with shell exec"
    priority: 100  # Lower number = higher priority
    match:
      command: "npm"
      args_pattern: "run.*&&|;|`|\\$\\("
    action:
      target: DENY
      reason: "Shell injection risk in npm script"

  - name: "Sanitize npm install"
    priority: 200
    match:
      command: "npm"
      args_pattern: "install|ci"
    action:
      target: MODIFY
      handler: npm-sanitizer
      then: ASK  # After modification, ask for approval

  - name: "Allow npm list"
    priority: 300
    match:
      command: "npm"
      args_pattern: "list|ls|view"
    action:
      target: ALLOW
```

#### 2. **Handler**
Code that processes commands (built-in or plugin)

```go
// Handler interface
type Handler interface {
    Name() string
    Priority() int

    // Match returns true if handler should process this command
    Match(ctx context.Context, cmd *Command) bool

    // Handle processes the command
    // Returns decision (ALLOW/DENY/ASK/MODIFY/CONTINUE) and modified command
    Handle(ctx context.Context, cmd *Command) (*Decision, *Command, error)

    // Config returns handler configuration schema
    Config() HandlerConfig
}

// Example: Path Validator Handler
type PathValidatorHandler struct {
    allowedPaths []string
}

func (h *PathValidatorHandler) Handle(ctx context.Context, cmd *Command) (*Decision, *Command, error) {
    for _, arg := range cmd.Args {
        if looksLikePath(arg) && !h.isPathAllowed(arg) {
            return &Decision{
                Target: DENY,
                Reason: fmt.Sprintf("Path %s not in allowed paths", arg),
            }, cmd, nil
        }
    }
    return &Decision{Target: CONTINUE}, cmd, nil
}
```

#### 3. **Chain**
Ordered sequence of handlers for a processing phase

```yaml
# clawrden.yaml
chains:
  pre-processing:
    - handler: logger
      priority: 10
    - handler: arg-sanitizer
      priority: 20
    - handler: context-enricher
      priority: 30

  policy:
    - handler: path-validator
      priority: 100
    - handler: pattern-matcher
      priority: 200
      config: rules.d/  # Load rules from directory
    - handler: risk-assessor
      priority: 300

  modification:
    - handler: npm-sanitizer
      priority: 100
      enabled_if: "command == 'npm'"
    - handler: docker-network-restrictor
      priority: 200
      enabled_if: "command == 'docker'"

  post-processing:
    - handler: audit-logger
      priority: 10
    - handler: metrics-collector
      priority: 20
    - handler: slack-notifier
      priority: 30
      enabled_if: "decision == 'ASK' || decision == 'DENY'"
```

#### 4. **Targets (Actions)**

- **ALLOW** - Execute immediately, skip remaining handlers
- **DENY** - Block immediately, skip remaining handlers
- **ASK** - Queue for human approval, skip remaining handlers
- **MODIFY** - Transform command, continue to next handler
- **CONTINUE** - No decision, pass to next handler
- **JUMP** - Jump to another chain (advanced)

### Plugin System Integration

#### Plugin Structure

```
/etc/clawrden/
├── clawrden.yaml           # Core config
├── rules.d/                # Declarative rules
│   ├── 00-defaults.yaml
│   ├── 10-npm.yaml
│   ├── 20-docker.yaml
│   └── 99-custom.yaml
├── handlers.d/             # Handler configs
│   ├── npm-sanitizer.yaml
│   ├── docker-restrictor.yaml
│   └── git-auditor.yaml
└── plugins/                # Plugin binaries/scripts
    ├── npm-plugin.so       # Go plugin
    ├── security-scanner/   # External executable
    │   └── scanner.py
    └── manifest.d/         # Plugin manifests
        ├── npm-plugin.yaml
        └── security-scanner.yaml
```

#### Plugin Manifest

```yaml
# plugins/manifest.d/npm-plugin.yaml
apiVersion: clawrden.io/v1
kind: PluginManifest
metadata:
  name: npm-security-plugin
  version: 1.0.0
  author: "Security Team"

spec:
  type: native  # native | exec | wasm
  binary: ../npm-plugin.so

  handlers:
    - name: npm-sanitizer
      description: "Sanitizes npm install commands"
      chains: [modification]
      default_priority: 200

    - name: npm-audit-checker
      description: "Runs npm audit on package.json changes"
      chains: [post-processing]
      default_priority: 150

  configuration:
    schema:
      type: object
      properties:
        allowed_registries:
          type: array
          items: {type: string}
        block_preinstall_scripts:
          type: boolean
          default: true

  dependencies:
    - npm >= 8.0.0
    - node >= 18.0.0
```

#### Plugin Configuration

```yaml
# handlers.d/npm-sanitizer.yaml
handler: npm-sanitizer
enabled: true
config:
  allowed_registries:
    - "https://registry.npmjs.org"
    - "https://npm.company.internal"
  block_preinstall_scripts: true
  allow_git_dependencies: false
  max_package_size_mb: 100
  audit_level: moderate  # low | moderate | high | critical
```

### Configuration Management Strategy

#### 1. **Layered Configuration**

```
┌─────────────────────────────────────┐
│      clawrden.yaml (Core)           │  Global settings, chain definitions
├─────────────────────────────────────┤
│      rules.d/*.yaml                 │  Declarative rules (priority-ordered)
├─────────────────────────────────────┤
│      handlers.d/*.yaml              │  Handler-specific configs
├─────────────────────────────────────┤
│      plugins/manifest.d/*.yaml      │  Plugin declarations
└─────────────────────────────────────┘
```

#### 2. **Configuration Precedence**

1. **Command-line flags** (highest)
2. **Environment variables** (CLI_* prefix)
3. **rules.d/** (evaluated by priority number)
4. **handlers.d/** (per-handler overrides)
5. **clawrden.yaml** (defaults)
6. **Built-in defaults** (lowest)

#### 3. **Hot Reload Strategy**

```go
type ConfigManager struct {
    watcher *fsnotify.Watcher

    // Separate config stores
    coreConfig    *CoreConfig
    rules         *RuleRegistry
    handlers      *HandlerRegistry
    plugins       *PluginRegistry
}

func (cm *ConfigManager) Watch() {
    for {
        select {
        case event := <-cm.watcher.Events:
            switch {
            case strings.HasPrefix(event.Name, "rules.d/"):
                cm.reloadRules()
            case strings.HasPrefix(event.Name, "handlers.d/"):
                cm.reloadHandlers()
            case event.Name == "clawrden.yaml":
                cm.reloadCore()
            }
        }
    }
}
```

#### 4. **Configuration Validation**

```bash
# CLI command to validate config
clawrden-cli config validate

# Output:
✓ Core config: clawrden.yaml
✓ Rules loaded: 15 (rules.d/)
✓ Handlers loaded: 8 (handlers.d/)
✓ Plugins loaded: 3 (plugins/)
✗ Warning: Rule 'npm-security' priority conflicts with 'docker-security'
✗ Error: Handler 'custom-scanner' references missing plugin 'scanner-plugin'
```

## Detailed Design

### 1. Handler Interface (Go)

```go
// pkg/handler/handler.go
package handler

import "context"

// Target represents the action to take
type Target string

const (
    TargetAllow    Target = "ALLOW"
    TargetDeny     Target = "DENY"
    TargetAsk      Target = "ASK"
    TargetModify   Target = "MODIFY"
    TargetContinue Target = "CONTINUE"
    TargetJump     Target = "JUMP"
)

// Decision returned by a handler
type Decision struct {
    Target      Target
    Reason      string
    JumpChain   string            // For JUMP target
    Metadata    map[string]any    // Additional context
}

// Command represents an intercepted command
type Command struct {
    Binary      string
    Args        []string
    Env         map[string]string
    WorkingDir  string
    User        string
    Group       string
    Metadata    map[string]any    // Enriched by handlers
}

// Handler processes commands
type Handler interface {
    // Name returns unique handler identifier
    Name() string

    // Priority for ordering (lower = earlier)
    Priority() int

    // Match checks if handler should process this command
    Match(ctx context.Context, cmd *Command) bool

    // Handle processes the command
    Handle(ctx context.Context, cmd *Command) (*Decision, *Command, error)

    // Initialize handler with config
    Init(config map[string]any) error

    // Schema returns configuration JSON schema
    Schema() []byte
}

// Registry manages handlers
type Registry struct {
    handlers map[string]Handler
    chains   map[string]*Chain
}

func (r *Registry) Register(h Handler) error
func (r *Registry) Get(name string) (Handler, error)
func (r *Registry) ProcessChain(ctx context.Context, chain string, cmd *Command) (*Decision, *Command, error)
```

### 2. Rule Definition (YAML)

```yaml
# rules.d/20-npm.yaml
apiVersion: clawrden.io/v1
kind: RuleSet
metadata:
  name: npm-security-rules
  priority: 20  # Lower = higher priority

rules:
  - id: npm-deny-shell-injection
    priority: 100
    description: "Block npm commands with shell metacharacters"
    match:
      command: npm
      args:
        pattern: ".*(&&|\\||;|`|\\$\\(|>|<).*"
    action:
      target: DENY
      reason: "Shell injection risk in npm command"
      notify: [slack, email]

  - id: npm-audit-install
    priority: 200
    description: "Require approval for npm install"
    match:
      command: npm
      args:
        prefix: [install, ci, add]
    action:
      target: MODIFY
      handlers:
        - npm-sanitizer      # Clean arguments
        - npm-audit-checker  # Run security audit
      then: ASK  # After handlers, require approval
      timeout: 5m
      notify: [slack]

  - id: npm-allow-readonly
    priority: 300
    description: "Auto-allow read-only npm commands"
    match:
      command: npm
      args:
        prefix: [list, ls, view, show, search, info]
    action:
      target: ALLOW

  - id: npm-restrict-registry
    priority: 150
    description: "Enforce internal registry for private packages"
    match:
      command: npm
      args:
        pattern: "@company/.*"
    action:
      target: MODIFY
      handlers:
        - registry-enforcer
      config:
        registry: "https://npm.company.internal"
```

### 3. Handler Configuration (YAML)

```yaml
# handlers.d/npm-sanitizer.yaml
handler: npm-sanitizer
enabled: true
chains: [modification]
priority: 200

config:
  # Registry restrictions
  allowed_registries:
    - "https://registry.npmjs.org"
    - "https://npm.company.internal"

  # Script execution
  block_scripts:
    - preinstall
    - postinstall
    - prepare
  allow_scripts_from_trusted: true
  trusted_packages:
    - "@company/*"

  # Dependency restrictions
  allow_git_dependencies: false
  allow_local_paths: false
  max_package_size_mb: 100

  # Security
  require_lockfile: true
  audit_level: moderate  # low | moderate | high | critical
  auto_fix_vulnerabilities: false

  # Modifications
  force_flags:
    - "--ignore-scripts"  # Add to all npm install
  strip_flags:
    - "--force"           # Remove dangerous flags
```

### 4. Plugin Development

#### Go Plugin Example

```go
// plugins/npm-plugin/main.go
package main

import (
    "context"
    "github.com/yourusername/clawrden/pkg/handler"
)

// NpmSanitizerHandler implements handler.Handler
type NpmSanitizerHandler struct {
    config *NpmConfig
}

type NpmConfig struct {
    AllowedRegistries []string          `json:"allowed_registries"`
    BlockScripts      []string          `json:"block_scripts"`
    RequireLockfile   bool              `json:"require_lockfile"`
    AuditLevel        string            `json:"audit_level"`
}

func (h *NpmSanitizerHandler) Name() string { return "npm-sanitizer" }
func (h *NpmSanitizerHandler) Priority() int { return 200 }

func (h *NpmSanitizerHandler) Match(ctx context.Context, cmd *handler.Command) bool {
    return cmd.Binary == "npm"
}

func (h *NpmSanitizerHandler) Handle(ctx context.Context, cmd *handler.Command) (*handler.Decision, *handler.Command, error) {
    modified := cmd.Clone()

    // Add --ignore-scripts if installing
    if containsAny(cmd.Args, []string{"install", "ci", "add"}) {
        if h.config.BlockScripts != nil {
            modified.Args = append(modified.Args, "--ignore-scripts")
        }
    }

    // Enforce registry
    if !hasRegistry(cmd.Args) {
        modified.Args = append(modified.Args, "--registry", h.config.AllowedRegistries[0])
    }

    // Check for lockfile
    if h.config.RequireLockfile && !lockfileExists(cmd.WorkingDir) {
        return &handler.Decision{
            Target: handler.TargetDeny,
            Reason: "package-lock.json required but not found",
        }, modified, nil
    }

    return &handler.Decision{
        Target: handler.TargetContinue,
    }, modified, nil
}

func (h *NpmSanitizerHandler) Init(config map[string]any) error {
    // Parse config into struct
    h.config = parseConfig(config)
    return nil
}

func (h *NpmSanitizerHandler) Schema() []byte {
    return []byte(`{
        "type": "object",
        "properties": {
            "allowed_registries": {
                "type": "array",
                "items": {"type": "string"}
            },
            "block_scripts": {
                "type": "array",
                "items": {"type": "string"}
            }
        }
    }`)
}

// Plugin exports (required by Go plugin system)
var Handler handler.Handler = &NpmSanitizerHandler{}
```

#### External Executable Plugin (Python)

```python
#!/usr/bin/env python3
# plugins/security-scanner/scanner.py

import json
import sys

def main():
    # Read command from stdin (JSON)
    cmd = json.load(sys.stdin)

    # Perform security scan
    vulnerabilities = scan_command(cmd)

    # Return decision (JSON to stdout)
    if vulnerabilities:
        decision = {
            "target": "DENY",
            "reason": f"Found {len(vulnerabilities)} security issues",
            "metadata": {
                "vulnerabilities": vulnerabilities
            }
        }
    else:
        decision = {
            "target": "CONTINUE"
        }

    json.dump(decision, sys.stdout)

def scan_command(cmd):
    # Implement security scanning logic
    vulns = []

    # Example: Check for known bad patterns
    if any(bad in ' '.join(cmd['args']) for bad in ['rm -rf /', 'dd if=']):
        vulns.append({
            "severity": "critical",
            "description": "Destructive command detected"
        })

    return vulns

if __name__ == "__main__":
    main()
```

```yaml
# plugins/manifest.d/security-scanner.yaml
apiVersion: clawrden.io/v1
kind: PluginManifest
metadata:
  name: security-scanner
  version: 1.0.0

spec:
  type: exec
  command: python3
  args: [plugins/security-scanner/scanner.py]

  handlers:
    - name: security-scanner
      chains: [policy]
      default_priority: 50

  configuration:
    schema:
      type: object
      properties:
        scan_level:
          type: string
          enum: [basic, standard, strict]
          default: standard
```

## Implementation Plan

### Phase 1: Foundation (1-2 weeks)

1. **Core Handler System**
   - [ ] Define `Handler` interface in `pkg/handler/`
   - [ ] Implement `Registry` for handler management
   - [ ] Implement `Chain` processing logic
   - [ ] Add basic built-in handlers (logger, path-validator)

2. **Rule Engine**
   - [ ] Design rule YAML schema
   - [ ] Implement rule parser and loader
   - [ ] Implement rule matching logic (patterns, priorities)
   - [ ] Add rule-to-handler bridge

3. **Configuration Management**
   - [ ] Implement layered config loading (core + rules.d + handlers.d)
   - [ ] Add config validation
   - [ ] Add hot-reload support (fsnotify)
   - [ ] CLI commands: `config validate`, `config reload`

### Phase 2: Core Handlers (1 week)

4. **Built-in Handlers**
   - [ ] Path validator (existing logic)
   - [ ] Pattern matcher (regex/glob matching)
   - [ ] Arg sanitizer (shell escape, injection prevention)
   - [ ] Audit logger (structured logging)
   - [ ] Metrics collector (Prometheus)

5. **Command Modification**
   - [ ] Arg rewriter handler
   - [ ] Env injector handler
   - [ ] Timeout setter handler
   - [ ] Working directory validator

### Phase 3: Plugin System (2 weeks)

6. **Plugin Infrastructure**
   - [ ] Plugin manifest schema
   - [ ] Plugin loader (Go plugins, exec, future WASM)
   - [ ] Plugin registry and lifecycle management
   - [ ] Plugin sandboxing/isolation

7. **Example Plugins**
   - [ ] NPM sanitizer plugin
   - [ ] Docker network restrictor plugin
   - [ ] Git auditor plugin
   - [ ] Security scanner plugin (Python example)

### Phase 4: Migration & Testing (1 week)

8. **Migrate Existing Policy**
   - [ ] Convert current `policy.yaml` to rule chains
   - [ ] Create backward compatibility layer
   - [ ] Migration guide documentation

9. **Testing**
   - [ ] Unit tests for handler system
   - [ ] Integration tests for chains
   - [ ] Plugin loading tests
   - [ ] Performance benchmarks (overhead < 10ms)

10. **Documentation**
    - [ ] Handler development guide
    - [ ] Plugin authoring guide
    - [ ] Rule configuration reference
    - [ ] Migration guide from old policy system

## Benefits

### For Users
- **Flexibility**: Fine-grained control over command processing
- **Extensibility**: Add new behaviors without code changes
- **Transparency**: Clear rule precedence and decision logic
- **Hot Reload**: Update rules without restarting warden

### For Developers
- **Plugin System**: Extend Clawrden without forking
- **Testability**: Test handlers in isolation
- **Modularity**: Clear separation of concerns
- **Reusability**: Share handlers across deployments

### For Security
- **Defense in Depth**: Multiple layers of validation
- **Audit Trail**: Every handler logs its decisions
- **Fail-Safe**: DENY by default if no rule matches
- **Isolation**: Plugins run in separate processes

## Trade-offs & Risks

### Complexity
- **Risk**: More moving parts, steeper learning curve
- **Mitigation**: Good defaults, clear documentation, migration guide

### Performance
- **Risk**: Handler chains add latency
- **Mitigation**: Optimize hot paths, benchmark, lazy loading

### Configuration Sprawl
- **Risk**: Too many config files hard to manage
- **Mitigation**: Clear directory structure, validation tools, IDE plugins

### Plugin Security
- **Risk**: Malicious plugins could bypass protections
- **Mitigation**: Plugin signing, capability restrictions, sandboxing

## Success Metrics

- [ ] Handler overhead < 5ms per command
- [ ] Plugin loading < 100ms on startup
- [ ] Hot reload < 50ms
- [ ] Backward compatible with existing `policy.yaml`
- [ ] At least 3 example plugins demonstrating different patterns
- [ ] Documentation covers 90% of use cases
- [ ] Migration guide successfully tested with real deployments

## Related Files

- `internal/warden/policy.go` - Current policy engine (to be replaced)
- `internal/executor/` - Execution layer (unchanged)
- `pkg/protocol/` - Socket protocol (unchanged)

## Future Enhancements

- **WASM Plugins**: Run plugins in WebAssembly sandbox
- **Remote Handlers**: Call handlers over gRPC
- **Policy Compiler**: Compile rules to eBPF for kernel-level enforcement
- **Visual Rule Builder**: Web UI for rule creation
- **Policy Templates**: Marketplace for common rule sets
- **AI-Assisted Rules**: LLM suggests rules based on audit logs

## Questions to Resolve

1. **Rule Priority**: Numeric (iptables style) or named chains (nginx style)?
   - Leaning toward numeric for simplicity

2. **Plugin Language Support**: Go-only or multi-language?
   - Start with Go plugins + exec protocol, add WASM later

3. **Configuration Format**: YAML only or support JSON/TOML?
   - YAML primary, JSON for API compatibility

4. **Hot Reload Behavior**: Graceful (finish in-flight) or immediate?
   - Graceful: finish current commands, apply to new ones

5. **Plugin Distribution**: Local files, registry, or both?
   - Phase 1: local files only
   - Future: plugin registry (like Helm charts)

## Notes

- This is a major architectural evolution, not just a refactor
- Should be treated as v2.0.0 (breaking change)
- Consider phased rollout with feature flag
- Get user feedback early with prototype

---

**Next Steps:**
1. Review this design with stakeholders
2. Prototype handler interface + 2-3 example handlers
3. Test performance overhead with benchmark suite
4. Create detailed API documentation
5. Begin Phase 1 implementation
