package warden

import (
	"clawrden/pkg/protocol"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Action represents the policy decision for a command.
type Action string

const (
	ActionAllow Action = "allow"
	ActionDeny  Action = "deny"
	ActionAsk   Action = "ask"
)

func (a Action) String() string {
	return string(a)
}

// Rule defines a single policy rule.
type Rule struct {
	Command string        `yaml:"command"`
	Action  Action        `yaml:"action"`
	Args    []string      `yaml:"args,omitempty"`    // Optional: specific arg patterns
	Reason  string        `yaml:"reason,omitempty"`  // Optional: human-readable reason
	Timeout time.Duration `yaml:"timeout,omitempty"` // Optional: per-command timeout (e.g., "300s", "5m")
}

// JailConfig defines a jail's intercepted commands and hardening mode.
type JailConfig struct {
	Commands []string `yaml:"commands"`
	Hardened bool     `yaml:"hardened"`
}

// PolicyConfig is the top-level policy configuration.
type PolicyConfig struct {
	DefaultAction  Action                `yaml:"default_action"`
	DefaultTimeout time.Duration         `yaml:"default_timeout,omitempty"` // Default timeout for all commands
	AllowedPaths   []string              `yaml:"allowed_paths,omitempty"`
	Jails          map[string]JailConfig `yaml:"jails,omitempty"`
	Rules          []Rule                `yaml:"rules"`
}

// PolicyEngine evaluates commands against a set of rules.
type PolicyEngine struct {
	config PolicyConfig
}

// LoadPolicy loads a policy from a YAML file.
func LoadPolicy(path string) (*PolicyEngine, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read policy file: %w", err)
	}

	var config PolicyConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse policy file: %w", err)
	}

	// Default to deny if not specified
	if config.DefaultAction == "" {
		config.DefaultAction = ActionDeny
	}

	// Default timeout if not specified (2 minutes)
	if config.DefaultTimeout == 0 {
		config.DefaultTimeout = 2 * time.Minute
	}

	// Default allowed paths if not specified
	if len(config.AllowedPaths) == 0 {
		config.AllowedPaths = []string{"/app/*", "/tmp/*"}
	}

	return &PolicyEngine{config: config}, nil
}

// DefaultPolicy returns a restrictive default policy.
func DefaultPolicy() *PolicyEngine {
	return &PolicyEngine{
		config: PolicyConfig{
			DefaultAction:  ActionDeny,
			DefaultTimeout: 2 * time.Minute,
			AllowedPaths:   []string{"/app/*", "/tmp/*"},
			Rules: []Rule{
				{Command: "ls", Action: ActionAllow},
				{Command: "cat", Action: ActionAllow},
				{Command: "head", Action: ActionAllow},
				{Command: "tail", Action: ActionAllow},
				{Command: "grep", Action: ActionAllow},
				{Command: "echo", Action: ActionAllow},
				{Command: "pwd", Action: ActionAllow},
				{Command: "wc", Action: ActionAllow},
				{Command: "find", Action: ActionAllow},
				{Command: "which", Action: ActionAllow},
			},
		},
	}
}

// EvaluationResult contains both the action and timeout for a request.
type EvaluationResult struct {
	Action  Action
	Timeout time.Duration
}

// Evaluate checks a request against the policy rules and returns the appropriate action and timeout.
func (pe *PolicyEngine) Evaluate(req *protocol.Request) EvaluationResult {
	command := filepath.Base(req.Command)

	for _, rule := range pe.config.Rules {
		if !matchCommand(rule.Command, command) {
			continue
		}

		// If no specific args patterns are defined, match on command alone
		if len(rule.Args) == 0 {
			timeout := rule.Timeout
			if timeout == 0 {
				timeout = pe.config.DefaultTimeout
			}
			return EvaluationResult{
				Action:  rule.Action,
				Timeout: timeout,
			}
		}

		// Check if the request args match the rule's arg patterns
		if matchArgs(rule.Args, req.Args) {
			timeout := rule.Timeout
			if timeout == 0 {
				timeout = pe.config.DefaultTimeout
			}
			return EvaluationResult{
				Action:  rule.Action,
				Timeout: timeout,
			}
		}
	}

	// No matching rule found — use default action and timeout
	return EvaluationResult{
		Action:  pe.config.DefaultAction,
		Timeout: pe.config.DefaultTimeout,
	}
}

// matchCommand checks if a command matches a rule pattern.
// Supports exact match and simple glob patterns.
func matchCommand(pattern, command string) bool {
	if pattern == "*" {
		return true
	}

	// Try filepath.Match for glob support
	matched, err := filepath.Match(pattern, command)
	if err != nil {
		// Invalid pattern — fall back to exact match
		return strings.EqualFold(pattern, command)
	}
	return matched
}

// matchArgs checks if any of the rule's arg patterns appear in the actual args.
func matchArgs(patterns, args []string) bool {
	argsStr := strings.Join(args, " ")
	for _, pattern := range patterns {
		if strings.Contains(argsStr, pattern) {
			return true
		}
	}
	return false
}

// ValidatePath checks if the given path matches any of the allowed path patterns.
// Patterns support glob syntax:
//   - "/app/*" matches anything under /app
//   - "/home/*/workspace/*" matches user workspace directories
//   - "/tmp/*" matches anything under /tmp
//
// Returns nil if path is allowed, error otherwise.
func (pe *PolicyEngine) ValidatePath(path string) error {
	if len(pe.config.AllowedPaths) == 0 {
		// No restrictions
		return nil
	}

	// Normalize path (remove trailing slashes, resolve ..)
	path = filepath.Clean(path)

	for _, pattern := range pe.config.AllowedPaths {
		// Try glob match first
		matched, err := filepath.Match(pattern, path)
		if err == nil && matched {
			return nil
		}

		// Also try prefix matching for patterns ending with /*
		if strings.HasSuffix(pattern, "/*") {
			prefix := strings.TrimSuffix(pattern, "/*")
			if strings.HasPrefix(path, prefix+"/") || path == prefix {
				return nil
			}
		}

		// Exact match
		if path == pattern {
			return nil
		}
	}

	return fmt.Errorf("path %q not allowed by policy (allowed patterns: %v)", path, pe.config.AllowedPaths)
}

// HasRule checks if the policy has any rule defined for a command.
func (pe *PolicyEngine) HasRule(command string) bool {
	for _, rule := range pe.config.Rules {
		if matchCommand(rule.Command, command) {
			return true
		}
	}
	return false
}

// GetJails returns the jail configurations from the policy.
func (pe *PolicyEngine) GetJails() map[string]JailConfig {
	return pe.config.Jails
}
