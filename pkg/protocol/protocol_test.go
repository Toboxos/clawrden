package protocol

import (
	"bytes"
	"testing"
)

func TestRequestRoundTrip(t *testing.T) {
	original := &Request{
		Command: "npm",
		Args:    []string{"install", "express"},
		Cwd:     "/app/backend",
		Env:     []string{"NODE_ENV=development", "PATH=/usr/bin"},
		Identity: Identity{
			UID: 1000,
			GID: 1000,
		},
	}

	var buf bytes.Buffer

	// Write
	if err := WriteRequest(&buf, original); err != nil {
		t.Fatalf("WriteRequest failed: %v", err)
	}

	// Read
	decoded, err := ReadRequest(&buf)
	if err != nil {
		t.Fatalf("ReadRequest failed: %v", err)
	}

	// Verify
	if decoded.Command != original.Command {
		t.Errorf("Command: got %q, want %q", decoded.Command, original.Command)
	}
	if len(decoded.Args) != len(original.Args) {
		t.Fatalf("Args length: got %d, want %d", len(decoded.Args), len(original.Args))
	}
	for i, arg := range decoded.Args {
		if arg != original.Args[i] {
			t.Errorf("Args[%d]: got %q, want %q", i, arg, original.Args[i])
		}
	}
	if decoded.Cwd != original.Cwd {
		t.Errorf("Cwd: got %q, want %q", decoded.Cwd, original.Cwd)
	}
	if decoded.Identity.UID != original.Identity.UID {
		t.Errorf("UID: got %d, want %d", decoded.Identity.UID, original.Identity.UID)
	}
	if decoded.Identity.GID != original.Identity.GID {
		t.Errorf("GID: got %d, want %d", decoded.Identity.GID, original.Identity.GID)
	}
}

func TestFrameRoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		frame Frame
	}{
		{
			name:  "stdout frame",
			frame: Frame{Type: StreamStdout, Payload: []byte("hello world\n")},
		},
		{
			name:  "stderr frame",
			frame: Frame{Type: StreamStderr, Payload: []byte("error: not found\n")},
		},
		{
			name:  "exit code frame",
			frame: Frame{Type: StreamExit, Payload: []byte{0}},
		},
		{
			name:  "cancel frame",
			frame: Frame{Type: StreamCancel, Payload: nil},
		},
		{
			name:  "empty payload",
			frame: Frame{Type: StreamStdout, Payload: nil},
		},
		{
			name:  "large payload",
			frame: Frame{Type: StreamStdout, Payload: bytes.Repeat([]byte("x"), 65536)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer

			if err := WriteFrame(&buf, tt.frame); err != nil {
				t.Fatalf("WriteFrame failed: %v", err)
			}

			decoded, err := ReadFrame(&buf)
			if err != nil {
				t.Fatalf("ReadFrame failed: %v", err)
			}

			if decoded.Type != tt.frame.Type {
				t.Errorf("Type: got %d, want %d", decoded.Type, tt.frame.Type)
			}
			if !bytes.Equal(decoded.Payload, tt.frame.Payload) {
				t.Errorf("Payload mismatch: got %d bytes, want %d bytes",
					len(decoded.Payload), len(tt.frame.Payload))
			}
		})
	}
}

func TestAckRoundTrip(t *testing.T) {
	for _, ack := range []byte{AckAllowed, AckDenied, AckPendingHITL} {
		var buf bytes.Buffer

		if err := WriteAck(&buf, ack); err != nil {
			t.Fatalf("WriteAck(%d) failed: %v", ack, err)
		}

		decoded, err := ReadAck(&buf)
		if err != nil {
			t.Fatalf("ReadAck failed: %v", err)
		}

		if decoded != ack {
			t.Errorf("Ack: got %d, want %d", decoded, ack)
		}
	}
}

func TestRequestEmptyArgs(t *testing.T) {
	original := &Request{
		Command:  "ls",
		Args:     []string{},
		Cwd:      "/app",
		Env:      []string{},
		Identity: Identity{UID: 0, GID: 0},
	}

	var buf bytes.Buffer

	if err := WriteRequest(&buf, original); err != nil {
		t.Fatalf("WriteRequest failed: %v", err)
	}

	decoded, err := ReadRequest(&buf)
	if err != nil {
		t.Fatalf("ReadRequest failed: %v", err)
	}

	if decoded.Command != "ls" {
		t.Errorf("Command: got %q, want %q", decoded.Command, "ls")
	}
}

func TestRequestUnicodePaths(t *testing.T) {
	original := &Request{
		Command:  "cat",
		Args:     []string{"日本語ファイル.txt"},
		Cwd:      "/app/données",
		Env:      []string{},
		Identity: Identity{UID: 1000, GID: 1000},
	}

	var buf bytes.Buffer

	if err := WriteRequest(&buf, original); err != nil {
		t.Fatalf("WriteRequest failed: %v", err)
	}

	decoded, err := ReadRequest(&buf)
	if err != nil {
		t.Fatalf("ReadRequest failed: %v", err)
	}

	if decoded.Args[0] != "日本語ファイル.txt" {
		t.Errorf("Unicode arg: got %q, want %q", decoded.Args[0], "日本語ファイル.txt")
	}
	if decoded.Cwd != "/app/données" {
		t.Errorf("Unicode cwd: got %q, want %q", decoded.Cwd, "/app/données")
	}
}
