// Package jailhouse manages the host-side filesystem structure for
// dynamic shim deployment (the "jailhouse" and "armory" architecture).
package jailhouse

import (
	"log"
	"sync"
	"time"
)

// Manager manages the jailhouse filesystem structure on the host.
// It creates and destroys per-jail directories containing
// symlinks to the master shim binary.
type Manager struct {
	armoryPath    string              // /var/lib/clawrden/armory
	jailhousePath string              // /var/lib/clawrden/jailhouse
	statePath     string              // /var/lib/clawrden/jailhouse.state.json
	mu            sync.RWMutex        // Protects jails map
	jails         map[string]*JailState // jailID -> state
	logger        *log.Logger
}

// JailState represents the state of a single jail.
type JailState struct {
	JailID    string    `json:"jail_id"`
	Commands  []string  `json:"commands"`
	Hardened  bool      `json:"hardened"`
	CreatedAt time.Time `json:"created_at"`
	JailPath  string    `json:"jail_path"`
}

// Config holds configuration for creating a new Manager.
type Config struct {
	ArmoryPath    string
	JailhousePath string
	StatePath     string
	Logger        *log.Logger
}
