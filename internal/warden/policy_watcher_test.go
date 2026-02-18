package warden

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPolicyWatcherCreate(t *testing.T) {
	tempDir := t.TempDir()
	policyPath := filepath.Join(tempDir, "policy.yaml")

	// Create a test policy file
	policyContent := `
default_action: deny
rules:
  - command: ls
    action: allow
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatalf("failed to create policy file: %v", err)
	}

	policy, err := LoadPolicy(policyPath)
	if err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}

	// Create policy watcher
	watcher, err := NewPolicyWatcher(policyPath, policy, log.New(os.Stdout, "[test] ", 0))
	if err != nil {
		t.Fatalf("NewPolicyWatcher failed: %v", err)
	}

	if watcher.policyPath != policyPath {
		t.Errorf("policyPath = %s, want %s", watcher.policyPath, policyPath)
	}

	if watcher.policy == nil {
		t.Error("policy should not be nil")
	}
}

func TestPolicyWatcherStartStop(t *testing.T) {
	tempDir := t.TempDir()
	policyPath := filepath.Join(tempDir, "policy.yaml")

	// Create a test policy file
	policyContent := `
default_action: deny
rules:
  - command: ls
    action: allow
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		t.Fatalf("failed to create policy file: %v", err)
	}

	policy, err := LoadPolicy(policyPath)
	if err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}

	watcher, err := NewPolicyWatcher(policyPath, policy, log.New(os.Stdout, "[test] ", 0))
	if err != nil {
		t.Fatalf("NewPolicyWatcher failed: %v", err)
	}

	ctx := context.Background()
	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	if err := watcher.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
}

func TestPolicyWatcherReload(t *testing.T) {
	tempDir := t.TempDir()
	policyPath := filepath.Join(tempDir, "policy.yaml")

	// Create initial policy file
	initialPolicy := `
default_action: deny
rules:
  - command: ls
    action: allow
`
	if err := os.WriteFile(policyPath, []byte(initialPolicy), 0644); err != nil {
		t.Fatalf("failed to create policy file: %v", err)
	}

	policy, err := LoadPolicy(policyPath)
	if err != nil {
		t.Fatalf("failed to load policy: %v", err)
	}

	// Track reload callbacks
	reloadCalled := false
	var reloadedPolicy *PolicyEngine

	watcher, err := NewPolicyWatcher(policyPath, policy, log.New(os.Stdout, "[test] ", 0))
	if err != nil {
		t.Fatalf("NewPolicyWatcher failed: %v", err)
	}

	watcher.OnReload(func(p *PolicyEngine) {
		reloadCalled = true
		reloadedPolicy = p
	})

	ctx := context.Background()
	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	defer watcher.Stop()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Update policy file
	updatedPolicy := `
default_action: deny
rules:
  - command: ls
    action: allow
  - command: cat
    action: allow
`
	if err := os.WriteFile(policyPath, []byte(updatedPolicy), 0644); err != nil {
		t.Fatalf("failed to update policy file: %v", err)
	}

	// Wait for reload (debounce + processing time)
	time.Sleep(1 * time.Second)

	if !reloadCalled {
		t.Error("OnReload callback was not called")
	}

	if reloadedPolicy == nil {
		t.Error("reloadedPolicy is nil")
	}

	// Verify new policy has both rules
	currentPolicy := watcher.GetPolicy()
	if !currentPolicy.HasRule("ls") {
		t.Error("new policy should have 'ls' rule")
	}
	if !currentPolicy.HasRule("cat") {
		t.Error("new policy should have 'cat' rule")
	}
}
