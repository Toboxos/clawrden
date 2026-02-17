package warden

import (
	"clawrden/pkg/protocol"
	"os"
	"path/filepath"
	"testing"
)

func TestAuditLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	// Create logger
	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("create audit logger: %v", err)
	}
	defer logger.Close()

	// Write some entries
	entries := []AuditEntry{
		{
			Command:  "echo",
			Args:     []string{"hello"},
			Cwd:      "/app",
			Identity: protocol.Identity{UID: 1000, GID: 1000},
			Decision: "allow",
			ExitCode: 0,
			Duration: 123.45,
		},
		{
			Command:  "sudo",
			Args:     []string{"rm", "-rf", "/"},
			Cwd:      "/app",
			Identity: protocol.Identity{UID: 1000, GID: 1000},
			Decision: "deny",
		},
		{
			Command:  "npm",
			Args:     []string{"install"},
			Cwd:      "/app",
			Identity: protocol.Identity{UID: 1000, GID: 1000},
			Decision: "ask",
			ExitCode: 0,
			Duration: 5432.1,
		},
	}

	for _, entry := range entries {
		if err := logger.Log(entry); err != nil {
			t.Fatalf("log entry: %v", err)
		}
	}

	// Close to flush
	logger.Close()

	// Read back
	readEntries, err := ReadAuditLog(logPath)
	if err != nil {
		t.Fatalf("read audit log: %v", err)
	}

	if len(readEntries) != len(entries) {
		t.Fatalf("expected %d entries, got %d", len(entries), len(readEntries))
	}

	for i, entry := range readEntries {
		if entry.Command != entries[i].Command {
			t.Errorf("entry %d: expected command %q, got %q", i, entries[i].Command, entry.Command)
		}
		if entry.Decision != entries[i].Decision {
			t.Errorf("entry %d: expected decision %q, got %q", i, entries[i].Decision, entry.Decision)
		}
		if entry.Timestamp == "" {
			t.Errorf("entry %d: timestamp is empty", i)
		}
	}
}

func TestAuditLoggerDisabled(t *testing.T) {
	// Empty path should disable logging
	logger, err := NewAuditLogger("")
	if err != nil {
		t.Fatalf("create disabled logger: %v", err)
	}
	defer logger.Close()

	// Should not error
	err = logger.Log(AuditEntry{
		Command:  "test",
		Decision: "allow",
	})
	if err != nil {
		t.Errorf("log to disabled logger: %v", err)
	}
}

func TestReadAuditLogNonexistent(t *testing.T) {
	entries, err := ReadAuditLog("/nonexistent/path/audit.log")
	if err != nil {
		t.Errorf("expected no error for nonexistent file, got: %v", err)
	}
	if entries != nil {
		t.Errorf("expected nil entries for nonexistent file, got: %v", entries)
	}
}

func TestAuditLoggerCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "nested", "dir", "audit.log")

	logger, err := NewAuditLogger(logPath)
	if err != nil {
		t.Fatalf("create audit logger with nested path: %v", err)
	}
	defer logger.Close()

	// Verify directory was created
	if _, err := os.Stat(filepath.Dir(logPath)); os.IsNotExist(err) {
		t.Error("audit log directory was not created")
	}

	// Verify we can write
	err = logger.Log(AuditEntry{Command: "test", Decision: "allow"})
	if err != nil {
		t.Errorf("log entry: %v", err)
	}
}
