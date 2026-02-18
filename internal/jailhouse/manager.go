package jailhouse

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// NewManager creates a new jailhouse manager.
func NewManager(cfg Config) (*Manager, error) {
	if cfg.Logger == nil {
		cfg.Logger = log.New(os.Stdout, "[jailhouse] ", log.LstdFlags|log.Lmsgprefix)
	}

	m := &Manager{
		armoryPath:    cfg.ArmoryPath,
		jailhousePath: cfg.JailhousePath,
		statePath:     cfg.StatePath,
		jails:         make(map[string]*JailState),
		logger:        cfg.Logger,
	}

	return m, nil
}

// Start initializes the jailhouse manager, loading persisted state.
func (m *Manager) Start() error {
	// Ensure directories exist
	if err := os.MkdirAll(m.armoryPath, 0755); err != nil {
		return fmt.Errorf("create armory directory: %w", err)
	}
	if err := os.MkdirAll(m.jailhousePath, 0755); err != nil {
		return fmt.Errorf("create jailhouse directory: %w", err)
	}

	// Load persisted state (ignore errors - state may not exist on first run)
	if err := m.LoadState(); err != nil {
		m.logger.Printf("warning: could not load state: %v", err)
	}

	// Ensure armory is properly set up
	if err := m.EnsureArmory(); err != nil {
		return fmt.Errorf("ensure armory: %w", err)
	}

	m.logger.Printf("started (armory=%s, jailhouse=%s)", m.armoryPath, m.jailhousePath)
	return nil
}

// EnsureArmory verifies that the master shim binary exists with correct permissions.
func (m *Manager) EnsureArmory() error {
	shimPath := filepath.Join(m.armoryPath, "clawrden-shim")

	// Check if shim exists
	stat, err := os.Stat(shimPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("master shim not found at %s (run 'make build-shim' and copy to armory)", shimPath)
		}
		return fmt.Errorf("stat shim: %w", err)
	}

	// Verify it's a regular file
	if !stat.Mode().IsRegular() {
		return fmt.Errorf("shim at %s is not a regular file", shimPath)
	}

	// Verify permissions (should be 0555 or similar - readable and executable by all)
	mode := stat.Mode()
	if mode&0111 == 0 {
		return fmt.Errorf("shim at %s is not executable (mode: %o)", shimPath, mode)
	}

	m.logger.Printf("armory verified: shim at %s (mode: %o)", shimPath, mode)
	return nil
}

// CreateJail creates a jail directory with symlinks to the shim.
func (m *Manager) CreateJail(jailID string, commands []string, hardened bool) error {
	if jailID == "" {
		return fmt.Errorf("jail ID cannot be empty")
	}

	// Validate command names
	for _, cmd := range commands {
		if err := validateCommandName(cmd); err != nil {
			return fmt.Errorf("invalid command %q: %w", cmd, err)
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if jail already exists
	if _, exists := m.jails[jailID]; exists {
		return fmt.Errorf("jail already exists for %s", jailID)
	}

	// Create jail directory structure
	jailPath := filepath.Join(m.jailhousePath, jailID)
	binPath := filepath.Join(jailPath, "bin")

	if err := os.MkdirAll(binPath, 0755); err != nil {
		return fmt.Errorf("create jail directory: %w", err)
	}

	// Create symlinks for each command
	shimPath := filepath.Join(m.armoryPath, "clawrden-shim")
	for _, cmd := range commands {
		linkPath := filepath.Join(binPath, cmd)
		if err := os.Symlink(shimPath, linkPath); err != nil {
			// Clean up on error
			os.RemoveAll(jailPath)
			return fmt.Errorf("create symlink for %s: %w", cmd, err)
		}
	}

	// Record jail state
	state := &JailState{
		JailID:    jailID,
		Commands:  commands,
		Hardened:  hardened,
		CreatedAt: time.Now(),
		JailPath:  jailPath,
	}
	m.jails[jailID] = state

	// Persist state (unlocked version - we already hold the lock)
	if err := m.saveStateUnlocked(); err != nil {
		m.logger.Printf("warning: failed to save state: %v", err)
	}

	m.logger.Printf("created jail %s: %d commands at %s", jailID, len(commands), jailPath)
	return nil
}

// DestroyJail removes a jail directory and all its contents.
func (m *Manager) DestroyJail(jailID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.jails[jailID]
	if !exists {
		return fmt.Errorf("jail not found: %s", jailID)
	}

	// Remove the jail directory
	if err := os.RemoveAll(state.JailPath); err != nil {
		return fmt.Errorf("remove jail directory: %w", err)
	}

	// Remove from state
	delete(m.jails, jailID)

	// Persist state (unlocked version - we already hold the lock)
	if err := m.saveStateUnlocked(); err != nil {
		m.logger.Printf("warning: failed to save state: %v", err)
	}

	m.logger.Printf("destroyed jail %s", jailID)
	return nil
}

// ListJails returns a list of all active jails.
func (m *Manager) ListJails() []*JailState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jails := make([]*JailState, 0, len(m.jails))
	for _, state := range m.jails {
		// Make a copy to avoid data races
		stateCopy := *state
		jails = append(jails, &stateCopy)
	}
	return jails
}

// GetJail returns the state of a specific jail.
func (m *Manager) GetJail(jailID string) (*JailState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.jails[jailID]
	if !exists {
		return nil, fmt.Errorf("jail not found: %s", jailID)
	}

	// Return a copy
	stateCopy := *state
	return &stateCopy, nil
}

// ReconcileJail updates an existing jail with a new set of commands.
// It adds missing symlinks and removes extra ones.
func (m *Manager) ReconcileJail(jailID string, commands []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.jails[jailID]
	if !exists {
		return fmt.Errorf("jail not found: %s", jailID)
	}

	// Validate new command names
	for _, cmd := range commands {
		if err := validateCommandName(cmd); err != nil {
			return fmt.Errorf("invalid command %q: %w", cmd, err)
		}
	}

	binPath := filepath.Join(state.JailPath, "bin")
	shimPath := filepath.Join(m.armoryPath, "clawrden-shim")

	// Determine which commands to add and remove
	oldCmds := makeSet(state.Commands)
	newCmds := makeSet(commands)

	// Remove symlinks for commands no longer needed
	for cmd := range oldCmds {
		if !newCmds[cmd] {
			linkPath := filepath.Join(binPath, cmd)
			if err := os.Remove(linkPath); err != nil && !os.IsNotExist(err) {
				m.logger.Printf("warning: failed to remove symlink %s: %v", linkPath, err)
			} else {
				m.logger.Printf("removed symlink for %s in jail %s", cmd, jailID)
			}
		}
	}

	// Add symlinks for new commands
	for cmd := range newCmds {
		if !oldCmds[cmd] {
			linkPath := filepath.Join(binPath, cmd)
			if err := os.Symlink(shimPath, linkPath); err != nil {
				return fmt.Errorf("create symlink for %s: %w", cmd, err)
			}
			m.logger.Printf("added symlink for %s in jail %s", cmd, jailID)
		}
	}

	// Update state
	state.Commands = commands

	// Persist state (unlocked version - we already hold the lock)
	if err := m.saveStateUnlocked(); err != nil {
		m.logger.Printf("warning: failed to save state: %v", err)
	}

	m.logger.Printf("reconciled jail %s: %d commands", jailID, len(commands))
	return nil
}

// validateCommandName ensures a command name is safe (no path traversal).
func validateCommandName(name string) error {
	if name == "" {
		return fmt.Errorf("command name cannot be empty")
	}
	if strings.Contains(name, "/") {
		return fmt.Errorf("command name cannot contain /")
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("command name cannot contain ..")
	}
	if strings.Contains(name, "\x00") {
		return fmt.Errorf("command name cannot contain null bytes")
	}
	return nil
}

// makeSet converts a slice to a set (map).
func makeSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}
