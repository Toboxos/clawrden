package jailhouse

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// persistedState represents the JSON structure saved to disk.
type persistedState struct {
	Version string                 `json:"version"`
	Updated time.Time              `json:"updated"`
	Jails   map[string]*JailState `json:"jails"`
}

// SaveState persists the current jailhouse state to disk.
// This method acquires a read lock.
func (m *Manager) SaveState() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.saveStateUnlocked()
}

// saveStateUnlocked persists state without acquiring locks.
// Caller must hold at least a read lock.
func (m *Manager) saveStateUnlocked() error {
	state := persistedState{
		Version: "1.0",
		Updated: time.Now(),
		Jails:   m.jails,
	}

	// Marshal to JSON
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	// Write atomically (write to temp file, then rename)
	tempPath := m.statePath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0600); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}

	if err := os.Rename(tempPath, m.statePath); err != nil {
		os.Remove(tempPath) // Clean up temp file
		return fmt.Errorf("rename state file: %w", err)
	}

	return nil
}

// LoadState restores the jailhouse state from disk.
func (m *Manager) LoadState() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Read state file
	data, err := os.ReadFile(m.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No state file yet - this is OK on first run
			return nil
		}
		return fmt.Errorf("read state file: %w", err)
	}

	// Unmarshal JSON
	var state persistedState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("unmarshal state: %w", err)
	}

	// Restore state
	m.jails = state.Jails
	if m.jails == nil {
		m.jails = make(map[string]*JailState)
	}

	m.logger.Printf("loaded state: %d jails (version=%s, updated=%s)",
		len(m.jails), state.Version, state.Updated.Format(time.RFC3339))

	return nil
}

// ReconcileState reconciles the in-memory state with the actual filesystem.
// It removes state entries for jails that no longer exist on disk.
func (m *Manager) ReconcileState() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	removed := 0
	for jailID, state := range m.jails {
		// Check if jail directory still exists
		if _, err := os.Stat(state.JailPath); os.IsNotExist(err) {
			m.logger.Printf("removing stale state for jail %s (directory not found)", jailID)
			delete(m.jails, jailID)
			removed++
		}
	}

	if removed > 0 {
		m.logger.Printf("reconciled state: removed %d stale entries", removed)
		// Persist the cleaned-up state (unlocked version - we already hold the lock)
		if err := m.saveStateUnlocked(); err != nil {
			return fmt.Errorf("save reconciled state: %w", err)
		}
	}

	return nil
}

// CleanStaleJails scans the jailhouse directory and removes any jail folders
// that are not tracked in the current state (orphaned jails).
func (m *Manager) CleanStaleJails() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// List all directories in jailhouse
	entries, err := os.ReadDir(m.jailhousePath)
	if err != nil {
		return fmt.Errorf("read jailhouse directory: %w", err)
	}

	removed := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		jailID := entry.Name()

		// Check if this jail is tracked in state
		if _, exists := m.jails[jailID]; !exists {
			jailPath := filepath.Join(m.jailhousePath, jailID)
			m.logger.Printf("removing orphaned jail directory: %s", jailPath)

			if err := os.RemoveAll(jailPath); err != nil {
				m.logger.Printf("warning: failed to remove orphaned jail %s: %v", jailPath, err)
			} else {
				removed++
			}
		}
	}

	if removed > 0 {
		m.logger.Printf("cleaned %d orphaned jail directories", removed)
	}

	return nil
}
