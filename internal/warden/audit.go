// Package warden provides audit logging capabilities.
package warden

import (
	"clawrden/pkg/protocol"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// AuditEntry represents a single command execution record.
type AuditEntry struct {
	Timestamp string             `json:"timestamp"`
	Command   string             `json:"command"`
	Args      []string           `json:"args"`
	Cwd       string             `json:"cwd"`
	Identity  protocol.Identity  `json:"identity"`
	Decision  string             `json:"decision"` // "allow", "deny", "ask"
	ExitCode  int                `json:"exit_code,omitempty"`
	Duration  float64            `json:"duration_ms,omitempty"`
	Error     string             `json:"error,omitempty"`
}

// AuditLogger writes structured audit logs in JSON-lines format.
type AuditLogger struct {
	writer io.WriteCloser
	mu     sync.Mutex
}

// NewAuditLogger creates a new audit logger writing to the specified file.
// If path is empty, audit logging is disabled.
func NewAuditLogger(path string) (*AuditLogger, error) {
	if path == "" {
		return &AuditLogger{writer: nopWriteCloser{}}, nil
	}

	// Ensure directory exists
	dir := path[:lastIndex(path, "/")]
	if dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create audit log directory: %w", err)
		}
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open audit log: %w", err)
	}

	return &AuditLogger{writer: file}, nil
}

// Log writes an audit entry to the log file.
func (al *AuditLogger) Log(entry AuditEntry) error {
	if al.writer == nil {
		return nil
	}

	al.mu.Lock()
	defer al.mu.Unlock()

	// Set timestamp if not already set
	if entry.Timestamp == "" {
		entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal audit entry: %w", err)
	}

	data = append(data, '\n')
	if _, err := al.writer.Write(data); err != nil {
		return fmt.Errorf("write audit entry: %w", err)
	}

	return nil
}

// Close closes the audit log file.
func (al *AuditLogger) Close() error {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.writer != nil {
		return al.writer.Close()
	}
	return nil
}

// ReadAuditLog reads all audit entries from the specified file.
func ReadAuditLog(path string) ([]AuditEntry, error) {
	if path == "" {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open audit log: %w", err)
	}
	defer file.Close()

	var entries []AuditEntry
	decoder := json.NewDecoder(file)
	for {
		var entry AuditEntry
		if err := decoder.Decode(&entry); err != nil {
			if err == io.EOF {
				break
			}
			// Skip malformed lines
			continue
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// lastIndex returns the last index of sep in s, or 0 if not found.
func lastIndex(s, sep string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == sep[0] {
			return i
		}
	}
	return 0
}

// nopWriteCloser is a no-op io.WriteCloser for disabled audit logging.
type nopWriteCloser struct{}

func (nopWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (nopWriteCloser) Close() error                { return nil }
