package jailhouse

import (
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	tempDir := t.TempDir()

	cfg := Config{
		ArmoryPath:    filepath.Join(tempDir, "armory"),
		JailhousePath: filepath.Join(tempDir, "jailhouse"),
		StatePath:     filepath.Join(tempDir, "state.json"),
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}

	if mgr.armoryPath != cfg.ArmoryPath {
		t.Errorf("armoryPath = %q, want %q", mgr.armoryPath, cfg.ArmoryPath)
	}
}

func TestEnsureArmory(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(string) error
		wantError bool
	}{
		{
			name: "shim exists and is executable",
			setup: func(armoryPath string) error {
				shimPath := filepath.Join(armoryPath, "clawrden-shim")
				if err := os.WriteFile(shimPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
					return err
				}
				return nil
			},
			wantError: false,
		},
		{
			name: "shim does not exist",
			setup: func(armoryPath string) error {
				// Don't create shim
				return nil
			},
			wantError: true,
		},
		{
			name: "shim exists but not executable",
			setup: func(armoryPath string) error {
				shimPath := filepath.Join(armoryPath, "clawrden-shim")
				if err := os.WriteFile(shimPath, []byte("test"), 0644); err != nil {
					return err
				}
				return nil
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			armoryPath := filepath.Join(tempDir, "armory")
			if err := os.MkdirAll(armoryPath, 0755); err != nil {
				t.Fatalf("create armory dir: %v", err)
			}

			if err := tt.setup(armoryPath); err != nil {
				t.Fatalf("setup failed: %v", err)
			}

			mgr, _ := NewManager(Config{
				ArmoryPath:    armoryPath,
				JailhousePath: filepath.Join(tempDir, "jailhouse"),
				StatePath:     filepath.Join(tempDir, "state.json"),
				Logger:        log.New(os.Stdout, "", 0),
			})

			err := mgr.EnsureArmory()
			if (err != nil) != tt.wantError {
				t.Errorf("EnsureArmory() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestCreateJail(t *testing.T) {
	tempDir := t.TempDir()
	armoryPath := filepath.Join(tempDir, "armory")
	jailhousePath := filepath.Join(tempDir, "jailhouse")

	// Create armory with shim
	if err := os.MkdirAll(armoryPath, 0755); err != nil {
		t.Fatalf("create armory: %v", err)
	}
	shimPath := filepath.Join(armoryPath, "clawrden-shim")
	if err := os.WriteFile(shimPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("create shim: %v", err)
	}

	mgr, err := NewManager(Config{
		ArmoryPath:    armoryPath,
		JailhousePath: jailhousePath,
		StatePath:     filepath.Join(tempDir, "state.json"),
	})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}

	if err := mgr.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Test creating a jail
	jailID := "test-jail-123"
	commands := []string{"ls", "npm", "docker"}

	err = mgr.CreateJail(jailID, commands, false)
	if err != nil {
		t.Fatalf("CreateJail failed: %v", err)
	}

	// Verify jail directory exists
	jailPath := filepath.Join(jailhousePath, jailID, "bin")
	if _, err := os.Stat(jailPath); os.IsNotExist(err) {
		t.Errorf("jail directory not created: %s", jailPath)
	}

	// Verify symlinks exist
	for _, cmd := range commands {
		linkPath := filepath.Join(jailPath, cmd)
		if _, err := os.Lstat(linkPath); err != nil {
			t.Errorf("symlink not created for %s: %v", cmd, err)
		}

		// Verify symlink points to shim
		target, err := os.Readlink(linkPath)
		if err != nil {
			t.Errorf("failed to read symlink %s: %v", linkPath, err)
		}
		if target != shimPath {
			t.Errorf("symlink %s points to %s, want %s", linkPath, target, shimPath)
		}
	}

	// Verify state was updated
	state, err := mgr.GetJail(jailID)
	if err != nil {
		t.Errorf("GetJail failed: %v", err)
	}
	if state.JailID != jailID {
		t.Errorf("state JailID = %s, want %s", state.JailID, jailID)
	}
	if len(state.Commands) != len(commands) {
		t.Errorf("state has %d commands, want %d", len(state.Commands), len(commands))
	}
}

func TestCreateJail_InvalidCommands(t *testing.T) {
	tempDir := t.TempDir()
	armoryPath := filepath.Join(tempDir, "armory")
	jailhousePath := filepath.Join(tempDir, "jailhouse")

	// Create armory with shim
	if err := os.MkdirAll(armoryPath, 0755); err != nil {
		t.Fatalf("create armory: %v", err)
	}
	shimPath := filepath.Join(armoryPath, "clawrden-shim")
	if err := os.WriteFile(shimPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("create shim: %v", err)
	}

	mgr, _ := NewManager(Config{
		ArmoryPath:    armoryPath,
		JailhousePath: jailhousePath,
		StatePath:     filepath.Join(tempDir, "state.json"),
	})
	if err := mgr.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	tests := []struct {
		name     string
		commands []string
	}{
		{"path traversal with slash", []string{"../bin/ls"}},
		{"path traversal with dotdot", []string{".."}},
		{"absolute path", []string{"/bin/ls"}},
		{"empty command", []string{""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.CreateJail("test-jail", tt.commands, false)
			if err == nil {
				t.Errorf("CreateJail should have failed for commands %v", tt.commands)
			}
		})
	}
}

func TestDestroyJail(t *testing.T) {
	tempDir := t.TempDir()
	armoryPath := filepath.Join(tempDir, "armory")
	jailhousePath := filepath.Join(tempDir, "jailhouse")

	// Setup
	if err := os.MkdirAll(armoryPath, 0755); err != nil {
		t.Fatalf("create armory: %v", err)
	}
	shimPath := filepath.Join(armoryPath, "clawrden-shim")
	if err := os.WriteFile(shimPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("create shim: %v", err)
	}

	mgr, _ := NewManager(Config{
		ArmoryPath:    armoryPath,
		JailhousePath: jailhousePath,
		StatePath:     filepath.Join(tempDir, "state.json"),
	})
	if err := mgr.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Create a jail
	jailID := "test-jail-456"
	if err := mgr.CreateJail(jailID, []string{"ls", "cat"}, false); err != nil {
		t.Fatalf("CreateJail: %v", err)
	}

	// Verify jail exists
	jailPath := filepath.Join(jailhousePath, jailID)
	if _, err := os.Stat(jailPath); os.IsNotExist(err) {
		t.Fatal("jail was not created")
	}

	// Destroy the jail
	if err := mgr.DestroyJail(jailID); err != nil {
		t.Fatalf("DestroyJail failed: %v", err)
	}

	// Verify jail directory is gone
	if _, err := os.Stat(jailPath); !os.IsNotExist(err) {
		t.Errorf("jail directory still exists after destroy")
	}

	// Verify state was updated
	if _, err := mgr.GetJail(jailID); err == nil {
		t.Errorf("GetJail should fail after destroy, but succeeded")
	}
}

func TestReconcileJail(t *testing.T) {
	tempDir := t.TempDir()
	armoryPath := filepath.Join(tempDir, "armory")
	jailhousePath := filepath.Join(tempDir, "jailhouse")

	// Setup
	if err := os.MkdirAll(armoryPath, 0755); err != nil {
		t.Fatalf("create armory: %v", err)
	}
	shimPath := filepath.Join(armoryPath, "clawrden-shim")
	if err := os.WriteFile(shimPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("create shim: %v", err)
	}

	mgr, _ := NewManager(Config{
		ArmoryPath:    armoryPath,
		JailhousePath: jailhousePath,
		StatePath:     filepath.Join(tempDir, "state.json"),
	})
	if err := mgr.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Create initial jail
	jailID := "test-jail-789"
	initialCmds := []string{"ls", "cat", "grep"}
	if err := mgr.CreateJail(jailID, initialCmds, false); err != nil {
		t.Fatalf("CreateJail: %v", err)
	}

	// Reconcile with new command set (remove grep, add npm)
	newCmds := []string{"ls", "cat", "npm"}
	if err := mgr.ReconcileJail(jailID, newCmds); err != nil {
		t.Fatalf("ReconcileJail: %v", err)
	}

	// Verify symlinks
	binPath := filepath.Join(jailhousePath, jailID, "bin")

	// Should have ls, cat, npm
	for _, cmd := range []string{"ls", "cat", "npm"} {
		linkPath := filepath.Join(binPath, cmd)
		if _, err := os.Lstat(linkPath); err != nil {
			t.Errorf("expected symlink for %s not found", cmd)
		}
	}

	// Should NOT have grep
	grepLink := filepath.Join(binPath, "grep")
	if _, err := os.Lstat(grepLink); !os.IsNotExist(err) {
		t.Errorf("grep symlink should have been removed")
	}

	// Verify state
	state, _ := mgr.GetJail(jailID)
	if len(state.Commands) != 3 {
		t.Errorf("state has %d commands, want 3", len(state.Commands))
	}
}

func TestStatePersistence(t *testing.T) {
	tempDir := t.TempDir()
	armoryPath := filepath.Join(tempDir, "armory")
	jailhousePath := filepath.Join(tempDir, "jailhouse")
	statePath := filepath.Join(tempDir, "state.json")

	// Setup
	if err := os.MkdirAll(armoryPath, 0755); err != nil {
		t.Fatalf("create armory: %v", err)
	}
	shimPath := filepath.Join(armoryPath, "clawrden-shim")
	if err := os.WriteFile(shimPath, []byte("#!/bin/sh\necho test"), 0755); err != nil {
		t.Fatalf("create shim: %v", err)
	}

	// Create first manager and add jails
	mgr1, _ := NewManager(Config{
		ArmoryPath:    armoryPath,
		JailhousePath: jailhousePath,
		StatePath:     statePath,
	})
	if err := mgr1.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	if err := mgr1.CreateJail("jail1", []string{"ls", "cat"}, false); err != nil {
		t.Fatalf("CreateJail: %v", err)
	}
	if err := mgr1.CreateJail("jail2", []string{"npm"}, true); err != nil {
		t.Fatalf("CreateJail: %v", err)
	}

	// Create second manager and load state
	mgr2, _ := NewManager(Config{
		ArmoryPath:    armoryPath,
		JailhousePath: jailhousePath,
		StatePath:     statePath,
	})
	if err := mgr2.Start(); err != nil {
		t.Fatalf("Start second manager: %v", err)
	}

	// Verify state was loaded
	jails := mgr2.ListJails()
	if len(jails) != 2 {
		t.Errorf("loaded %d jails, want 2", len(jails))
	}

	// Verify specific jail details
	state, err := mgr2.GetJail("jail1")
	if err != nil {
		t.Errorf("GetJail(jail1) failed: %v", err)
	}
	if len(state.Commands) != 2 {
		t.Errorf("jail1 has %d commands, want 2", len(state.Commands))
	}
	if state.Hardened {
		t.Errorf("jail1 should not be hardened")
	}

	state2, err := mgr2.GetJail("jail2")
	if err != nil {
		t.Errorf("GetJail(jail2) failed: %v", err)
	}
	if !state2.Hardened {
		t.Errorf("jail2 should be hardened")
	}
}
