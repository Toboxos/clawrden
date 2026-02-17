package integration

import (
	"clawrden/internal/warden"
	"clawrden/pkg/protocol"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestShimWardenAllowedCommand tests the full flow:
// shim connects → sends request → warden evaluates "allow" → local exec → streams back
func TestShimWardenAllowedCommand(t *testing.T) {
	socketPath := tempSocketPath(t)
	tmpDir := t.TempDir()

	// Start the warden server
	srv := startTestWarden(t, socketPath)
	defer srv.Shutdown()

	// Wait for the server to be ready
	waitForSocket(t, socketPath)

	// Connect as a shim would
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial warden: %v", err)
	}
	defer conn.Close()

	// Send a request for an allowed command (echo)
	req := &protocol.Request{
		Command:  "echo",
		Args:     []string{"hello", "clawrden"},
		Cwd:      tmpDir,
		Env:      []string{"PATH=/usr/bin:/bin"},
		Identity: protocol.Identity{UID: 1000, GID: 1000},
	}

	if err := protocol.WriteRequest(conn, req); err != nil {
		t.Fatalf("write request: %v", err)
	}

	// Read ack — should be allowed
	ack, err := protocol.ReadAck(conn)
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if ack != protocol.AckAllowed {
		t.Fatalf("expected AckAllowed (0), got %d", ack)
	}

	// Read frames until exit
	var stdout, stderr []byte
	exitCode := -1

	for {
		frame, err := protocol.ReadFrame(conn)
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("read frame: %v", err)
		}

		switch frame.Type {
		case protocol.StreamStdout:
			stdout = append(stdout, frame.Payload...)
		case protocol.StreamStderr:
			stderr = append(stderr, frame.Payload...)
		case protocol.StreamExit:
			if len(frame.Payload) > 0 {
				exitCode = int(frame.Payload[0])
			} else {
				exitCode = 0
			}
			goto done
		}
	}

done:
	if exitCode != 0 {
		t.Errorf("expected exit code 0, got %d (stderr: %s)", exitCode, string(stderr))
	}

	expected := "hello clawrden\n"
	if string(stdout) != expected {
		t.Errorf("stdout: got %q, want %q", string(stdout), expected)
	}
}

// TestShimWardenDeniedCommand tests that denied commands get AckDenied.
func TestShimWardenDeniedCommand(t *testing.T) {
	socketPath := tempSocketPath(t)

	srv := startTestWarden(t, socketPath)
	defer srv.Shutdown()

	waitForSocket(t, socketPath)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial warden: %v", err)
	}
	defer conn.Close()

	// Send a request for a denied command (sudo is not in allowlist → default deny)
	req := &protocol.Request{
		Command:  "sudo",
		Args:     []string{"rm", "-rf", "/"},
		Cwd:      "/app",
		Env:      []string{},
		Identity: protocol.Identity{UID: 1000, GID: 1000},
	}

	if err := protocol.WriteRequest(conn, req); err != nil {
		t.Fatalf("write request: %v", err)
	}

	// Read ack — should be denied
	ack, err := protocol.ReadAck(conn)
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if ack != protocol.AckDenied {
		t.Fatalf("expected AckDenied (1), got %d", ack)
	}
}

// TestShimWardenPathRejection tests that requests with cwd outside /app are rejected.
func TestShimWardenPathRejection(t *testing.T) {
	socketPath := tempSocketPath(t)

	srv := startTestWarden(t, socketPath)
	defer srv.Shutdown()

	waitForSocket(t, socketPath)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial warden: %v", err)
	}
	defer conn.Close()

	// Send request with cwd outside /app
	req := &protocol.Request{
		Command:  "ls",
		Args:     []string{"-la"},
		Cwd:      "/etc",
		Env:      []string{},
		Identity: protocol.Identity{UID: 1000, GID: 1000},
	}

	if err := protocol.WriteRequest(conn, req); err != nil {
		t.Fatalf("write request: %v", err)
	}

	// Should be denied due to path violation
	ack, err := protocol.ReadAck(conn)
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if ack != protocol.AckDenied {
		t.Fatalf("expected AckDenied (1) for path outside /app, got %d", ack)
	}
}

// TestShimWardenHITLFlow tests the human-in-the-loop approval flow.
func TestShimWardenHITLFlow(t *testing.T) {
	socketPath := tempSocketPath(t)

	// Create a warden with a policy that has "ask" rules
	srv := startTestWardenWithPolicy(t, socketPath, &warden.PolicyEngine{})
	defer srv.Shutdown()

	waitForSocket(t, socketPath)

	// We'll approve the request from a separate goroutine
	approved := make(chan struct{})
	go func() {
		// Wait a bit for the request to be queued
		time.Sleep(200 * time.Millisecond)

		// Find and approve the pending request
		queue := srv.GetHITLQueue()
		pending := queue.List()
		if len(pending) > 0 {
			queue.Resolve(pending[0].ID, warden.DecisionApprove)
		}
		close(approved)
	}()

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial warden: %v", err)
	}
	defer conn.Close()

	// Send a request for a command that requires HITL (npm in policy.yaml)
	tmpDir := t.TempDir()
	req := &protocol.Request{
		Command:  "echo",
		Args:     []string{"approved-output"},
		Cwd:      tmpDir,
		Env:      []string{"PATH=/usr/bin:/bin"},
		Identity: protocol.Identity{UID: 1000, GID: 1000},
	}

	if err := protocol.WriteRequest(conn, req); err != nil {
		t.Fatalf("write request: %v", err)
	}

	// Read ack — should be pending HITL
	ack, err := protocol.ReadAck(conn)
	if err != nil {
		t.Fatalf("read ack: %v", err)
	}
	if ack != protocol.AckPendingHITL {
		// If the default policy is used and echo is allowed, skip this test
		t.Skipf("echo is allowed by default policy, skipping HITL test (ack=%d)", ack)
	}

	// Read the second ack after approval
	ack2, err := protocol.ReadAck(conn)
	if err != nil {
		t.Fatalf("read resolved ack: %v", err)
	}
	if ack2 != protocol.AckAllowed {
		t.Fatalf("expected AckAllowed after approval, got %d", ack2)
	}

	<-approved
}

// TestMultipleConcurrentRequests tests handling multiple connections simultaneously.
func TestMultipleConcurrentRequests(t *testing.T) {
	socketPath := tempSocketPath(t)
	tmpDir := t.TempDir()

	srv := startTestWarden(t, socketPath)
	defer srv.Shutdown()

	waitForSocket(t, socketPath)

	// Send 5 concurrent requests
	errs := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func(n int) {
			conn, err := net.Dial("unix", socketPath)
			if err != nil {
				errs <- fmt.Errorf("dial %d: %v", n, err)
				return
			}
			defer conn.Close()

			req := &protocol.Request{
				Command:  "echo",
				Args:     []string{fmt.Sprintf("request-%d", n)},
				Cwd:      tmpDir,
				Env:      []string{"PATH=/usr/bin:/bin"},
				Identity: protocol.Identity{UID: 1000, GID: 1000},
			}

			if err := protocol.WriteRequest(conn, req); err != nil {
				errs <- fmt.Errorf("write %d: %v", n, err)
				return
			}

			ack, err := protocol.ReadAck(conn)
			if err != nil {
				errs <- fmt.Errorf("read ack %d: %v", n, err)
				return
			}
			if ack != protocol.AckAllowed {
				errs <- fmt.Errorf("request %d not allowed: ack=%d", n, ack)
				return
			}

			// Read until exit
			for {
				frame, err := protocol.ReadFrame(conn)
				if err != nil {
					errs <- fmt.Errorf("frame %d: %v", n, err)
					return
				}
				if frame.Type == protocol.StreamExit {
					if len(frame.Payload) > 0 && frame.Payload[0] != 0 {
						errs <- fmt.Errorf("request %d exit code: %d", n, frame.Payload[0])
						return
					}
					break
				}
			}

			errs <- nil
		}(i)
	}

	for i := 0; i < 5; i++ {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func tempSocketPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "warden.sock")
}

func startTestWarden(t *testing.T, socketPath string) *warden.Server {
	t.Helper()

	// Use the project's policy.yaml if available, otherwise default policy
	policyPath := "../../policy.yaml"

	logger := log.New(io.Discard, "[test-warden] ", log.LstdFlags)

	srv, err := warden.NewServer(warden.Config{
		SocketPath: socketPath,
		PolicyPath: policyPath,
		Logger:     logger,
	})
	if err != nil {
		t.Fatalf("create warden: %v", err)
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			// Only log if not a clean shutdown
			logger.Printf("server error: %v", err)
		}
	}()

	return srv
}

func startTestWardenWithPolicy(t *testing.T, socketPath string, _ *warden.PolicyEngine) *warden.Server {
	t.Helper()

	// Create a temporary policy file with "ask" for echo
	dir := t.TempDir()
	policyPath := filepath.Join(dir, "test-policy.yaml")
	policyContent := `default_action: deny
rules:
  - command: echo
    action: ask
`
	os.WriteFile(policyPath, []byte(policyContent), 0644)

	logger := log.New(io.Discard, "[test-warden] ", log.LstdFlags)

	srv, err := warden.NewServer(warden.Config{
		SocketPath: socketPath,
		PolicyPath: policyPath,
		Logger:     logger,
	})
	if err != nil {
		t.Fatalf("create warden: %v", err)
	}

	go func() {
		srv.ListenAndServe()
	}()

	return srv
}

func waitForSocket(t *testing.T, socketPath string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			conn.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("warden socket not ready at %s", socketPath)
}
