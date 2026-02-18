package warden

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// PolicyWatcher watches the policy file for changes and triggers hot-reload.
type PolicyWatcher struct {
	policyPath string
	policy     *PolicyEngine
	watcher    *fsnotify.Watcher
	logger     *log.Logger

	mu       sync.RWMutex
	onReload []func(*PolicyEngine) // Callbacks to invoke on policy reload

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewPolicyWatcher creates a new policy file watcher.
func NewPolicyWatcher(policyPath string, policy *PolicyEngine, logger *log.Logger) (*PolicyWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}

	return &PolicyWatcher{
		policyPath: policyPath,
		policy:     policy,
		watcher:    watcher,
		logger:     logger,
		onReload:   make([]func(*PolicyEngine), 0),
	}, nil
}

// Start begins watching the policy file for changes.
func (pw *PolicyWatcher) Start(ctx context.Context) error {
	pw.ctx, pw.cancel = context.WithCancel(ctx)

	// Watch the policy file
	if err := pw.watcher.Add(pw.policyPath); err != nil {
		// If the file doesn't exist, watch the directory instead
		dir := filepath.Dir(pw.policyPath)
		if err := pw.watcher.Add(dir); err != nil {
			return fmt.Errorf("watch policy file/dir: %w", err)
		}
		pw.logger.Printf("watching directory %s for policy changes", dir)
	} else {
		pw.logger.Printf("watching policy file %s for changes", pw.policyPath)
	}

	// Start the watch loop
	pw.wg.Add(1)
	go func() {
		defer pw.wg.Done()
		pw.watchLoop()
	}()

	pw.logger.Printf("policy watcher started")
	return nil
}

// Stop gracefully shuts down the policy watcher.
func (pw *PolicyWatcher) Stop() error {
	if pw.cancel != nil {
		pw.cancel()
	}
	if pw.watcher != nil {
		pw.watcher.Close()
	}
	pw.wg.Wait()
	pw.logger.Printf("policy watcher stopped")
	return nil
}

// OnReload registers a callback to be invoked when the policy is reloaded.
func (pw *PolicyWatcher) OnReload(callback func(*PolicyEngine)) {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	pw.onReload = append(pw.onReload, callback)
}

// watchLoop processes file system events.
func (pw *PolicyWatcher) watchLoop() {
	// Debounce timer to avoid reloading on every write
	var debounceTimer *time.Timer
	debounceDuration := 500 * time.Millisecond

	for {
		select {
		case <-pw.ctx.Done():
			return

		case event, ok := <-pw.watcher.Events:
			if !ok {
				return
			}

			// Only care about writes and creates to the policy file
			if event.Name != pw.policyPath {
				continue
			}

			if event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Create == fsnotify.Create {
				pw.logger.Printf("detected policy file change: %s", event.Op)

				// Reset debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(debounceDuration, func() {
					if err := pw.handlePolicyChange(); err != nil {
						pw.logger.Printf("error handling policy change: %v", err)
					}
				})
			}

		case err, ok := <-pw.watcher.Errors:
			if !ok {
				return
			}
			pw.logger.Printf("watcher error: %v", err)
		}
	}
}

// handlePolicyChange reloads the policy.
func (pw *PolicyWatcher) handlePolicyChange() error {
	pw.logger.Printf("reloading policy from %s", pw.policyPath)

	// Load new policy
	newPolicy, err := LoadPolicy(pw.policyPath)
	if err != nil {
		return fmt.Errorf("load policy: %w", err)
	}

	// Update policy reference
	pw.mu.Lock()
	pw.policy = newPolicy
	callbacks := make([]func(*PolicyEngine), len(pw.onReload))
	copy(callbacks, pw.onReload)
	pw.mu.Unlock()

	pw.logger.Printf("policy reloaded successfully")

	// Invoke callbacks
	for _, callback := range callbacks {
		callback(newPolicy)
	}

	return nil
}

// GetPolicy returns the current policy (thread-safe).
func (pw *PolicyWatcher) GetPolicy() *PolicyEngine {
	pw.mu.RLock()
	defer pw.mu.RUnlock()
	return pw.policy
}
