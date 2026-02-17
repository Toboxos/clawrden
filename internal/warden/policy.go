package warden

import (
	"clawrden/pkg/protocol"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	Command string   `yaml:"command"`
	Action  Action   `yaml:"action"`
	Args    []string `yaml:"args,omitempty"`    // Optional: specific arg patterns
	Reason  string   `yaml:"reason,omitempty"` // Optional: human-readable reason
}

// PolicyConfig is the top-level policy configuration.
type PolicyConfig struct {
	DefaultAction Action `yaml:"default_action"`
	Rules         []Rule `yaml:"rules"`
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

	return &PolicyEngine{config: config}, nil
}

// DefaultPolicy returns a restrictive default policy.
func DefaultPolicy() *PolicyEngine {
	return &PolicyEngine{
		config: PolicyConfig{
			DefaultAction: ActionDeny,
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

// Evaluate checks a request against the policy rules and returns the appropriate action.
func (pe *PolicyEngine) Evaluate(req *protocol.Request) Action {
	command := filepath.Base(req.Command)

	for _, rule := range pe.config.Rules {
		if !matchCommand(rule.Command, command) {
			continue
		}

		// If no specific args patterns are defined, match on command alone
		if len(rule.Args) == 0 {
			return rule.Action
		}

		// Check if the request args match the rule's arg patterns
		if matchArgs(rule.Args, req.Args) {
			return rule.Action
		}
	}

	// No matching rule found — use default action
	return pe.config.DefaultAction
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
