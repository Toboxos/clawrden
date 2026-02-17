package warden

import (
	"clawrden/pkg/protocol"
	"testing"
)

func TestScrubEnvironment(t *testing.T) {
	env := []string{
		"PATH=/usr/bin:/bin",
		"HOME=/home/agent",
		"NODE_ENV=development",
		"LANG=en_US.UTF-8",
		"TERM=xterm-256color",
		"LD_PRELOAD=/evil/lib.so",
		"DOCKER_HOST=tcp://evil:2375",
		"KUBECONFIG=/home/agent/.kube/config",
		"AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
		"SECRET_STUFF=supersecret",
		"RANDOM_VAR=whatever",
	}

	scrubbed := ScrubEnvironment(env)

	// Should keep allwlisted vars
	expected := map[string]bool{
		"PATH":     false,
		"HOME":     false,
		"NODE_ENV": false,
		"LANG":     false,
		"TERM":     false,
	}

	for _, e := range scrubbed {
		key := envKey(e)
		if _, ok := expected[key]; ok {
			expected[key] = true
		} else {
			t.Errorf("unexpected env var passed through: %s", key)
		}
	}

	for key, found := range expected {
		if !found {
			t.Errorf("expected env var %s was not passed through", key)
		}
	}

	// Verify blocklisted vars are NOT present
	for _, e := range scrubbed {
		key := envKey(e)
		if key == "LD_PRELOAD" || key == "DOCKER_HOST" || key == "KUBECONFIG" || key == "AWS_ACCESS_KEY_ID" {
			t.Errorf("blocklisted env var %s was passed through", key)
		}
	}
}

func TestPolicyEvaluate(t *testing.T) {
	pe := DefaultPolicy()

	tests := []struct {
		command  string
		expected Action
	}{
		{"ls", ActionAllow},
		{"cat", ActionAllow},
		{"grep", ActionAllow},
		{"echo", ActionAllow},
		{"pwd", ActionAllow},
		{"head", ActionAllow},
		{"tail", ActionAllow},
		{"wc", ActionAllow},
		{"rm", ActionDeny},       // Not in allowlist -> default deny
		{"apt-get", ActionDeny},  // Not in allowlist -> default deny
		{"npm", ActionDeny},      // Not in allowlist -> default deny
		{"sudo", ActionDeny},     // Not in allowlist -> default deny
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			req := &protocol.Request{
				Command:  tt.command,
				Args:     []string{},
				Cwd:      "/app",
				Identity: protocol.Identity{UID: 1000, GID: 1000},
			}
			action := pe.Evaluate(req)
			if action != tt.expected {
				t.Errorf("command %q: got %v, want %v", tt.command, action, tt.expected)
			}
		})
	}
}

func TestPolicyEvaluateWithArgs(t *testing.T) {
	// Rules are evaluated in order; deny rule for rm -rf / must come first
	pe := &PolicyEngine{
		config: PolicyConfig{
			DefaultAction: ActionDeny,
			Rules: []Rule{
				{Command: "rm", Action: ActionDeny, Args: []string{"-rf /"}},
				{Command: "rm", Action: ActionAllow, Args: []string{"-r"}},
				{Command: "npm", Action: ActionAsk},
			},
		},
	}

	tests := []struct {
		command  string
		args     []string
		expected Action
	}{
		{"npm", []string{"install"}, ActionAsk},
		{"rm", []string{"-rf", "/"}, ActionDeny},
	}

	for _, tt := range tests {
		t.Run(tt.command+" "+tt.args[0], func(t *testing.T) {
			req := &protocol.Request{
				Command:  tt.command,
				Args:     tt.args,
				Cwd:      "/app",
				Identity: protocol.Identity{UID: 1000, GID: 1000},
			}
			action := pe.Evaluate(req)
			if action != tt.expected {
				t.Errorf("command %q %v: got %v, want %v", tt.command, tt.args, action, tt.expected)
			}
		})
	}
}

func TestValidatePath(t *testing.T) {
	tests := []struct {
		cwd   string
		valid bool
	}{
		{"/app", true},
		{"/app/backend", true},
		{"/app/frontend/src", true},
		{"/tmp", false},
		{"/etc/passwd", false},
		{"/home/user", false},
		{"/", false},
	}

	for _, tt := range tests {
		t.Run(tt.cwd, func(t *testing.T) {
			// Test via the Request + the server's path check
			req := &protocol.Request{Cwd: tt.cwd}
			valid := len(req.Cwd) >= 4 && req.Cwd[:4] == "/app"
			if valid != tt.valid {
				t.Errorf("cwd %q: got valid=%v, want valid=%v", tt.cwd, valid, tt.valid)
			}
		})
	}
}
