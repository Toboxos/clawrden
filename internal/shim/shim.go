// Package shim implements the Prisoner-side shim logic.
// It captures the OS context, serializes it as a protocol.Request,
// sends it to the Warden over a Unix socket, and streams the response.
package shim

import (
	"clawrden/pkg/protocol"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
)

// Run executes the shim logic and returns the exit code.
func Run() int {
	// Determine which tool we're impersonating
	toolName := filepath.Base(os.Args[0])

	// If invoked as "clawrden-shim" directly (not via symlink), show usage
	if toolName == "clawrden-shim" {
		fmt.Fprintf(os.Stderr, "clawrden-shim: must be invoked via a tool symlink (e.g., npm, docker)\n")
		fmt.Fprintf(os.Stderr, "usage: create a symlink: ln -s clawrden-shim <tool-name>\n")
		return 1
	}

	// Build the args list (everything after the tool name)
	args := []string{}
	if len(os.Args) > 1 {
		args = os.Args[1:]
	}

	// Capture current working directory
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "clawrden-shim [%s]: failed to get working directory: %v\n", toolName, err)
		return 1
	}

	// Capture environment
	env := os.Environ()

	// Capture identity
	uid := os.Getuid()
	gid := os.Getgid()

	// Build the request
	req := &protocol.Request{
		Command: toolName,
		Args:    args,
		Cwd:     cwd,
		Env:     env,
		Identity: protocol.Identity{
			UID: uid,
			GID: gid,
		},
	}

	// Determine socket path (allow override via env)
	socketPath := os.Getenv("CLAWRDEN_SOCKET")
	if socketPath == "" {
		socketPath = protocol.DefaultSocketPath
	}

	// Connect to the Warden
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "clawrden-shim [%s]: failed to connect to warden at %s: %v\n",
			toolName, socketPath, err)
		return 1
	}
	defer conn.Close()

	// Set up signal handling (must happen before any blocking I/O)
	cancelSignals(conn)

	// Send the request
	if err := protocol.WriteRequest(conn, req); err != nil {
		fmt.Fprintf(os.Stderr, "clawrden-shim [%s]: failed to send request: %v\n", toolName, err)
		return 1
	}

	// Read the ack byte
	ack, err := protocol.ReadAck(conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "clawrden-shim [%s]: failed to read ack: %v\n", toolName, err)
		return 1
	}

	switch ack {
	case protocol.AckDenied:
		fmt.Fprintf(os.Stderr, "clawrden-shim [%s]: command denied by policy\n", toolName)
		return 1
	case protocol.AckPendingHITL:
		fmt.Fprintf(os.Stderr, "clawrden-shim [%s]: awaiting approval...\n", toolName)
		// After the pending message, the Warden will send another ack when resolved
		resolvedAck, err := protocol.ReadAck(conn)
		if err != nil {
			fmt.Fprintf(os.Stderr, "clawrden-shim [%s]: lost connection while awaiting approval: %v\n", toolName, err)
			return 1
		}
		if resolvedAck == protocol.AckDenied {
			fmt.Fprintf(os.Stderr, "clawrden-shim [%s]: command denied by reviewer\n", toolName)
			return 1
		}
	case protocol.AckAllowed:
		// Proceed to streaming
	default:
		fmt.Fprintf(os.Stderr, "clawrden-shim [%s]: unknown ack: %d\n", toolName, ack)
		return 1
	}

	// Stream frames from the Warden
	return streamFrames(conn, toolName)
}

// streamFrames reads and dispatches frames from the Warden connection.
func streamFrames(conn net.Conn, toolName string) int {
	for {
		frame, err := protocol.ReadFrame(conn)
		if err != nil {
			if err == io.EOF {
				// Connection closed without an exit frame
				return 1
			}
			fmt.Fprintf(os.Stderr, "clawrden-shim [%s]: stream error: %v\n", toolName, err)
			return 1
		}

		switch frame.Type {
		case protocol.StreamStdout:
			os.Stdout.Write(frame.Payload)
		case protocol.StreamStderr:
			os.Stderr.Write(frame.Payload)
		case protocol.StreamExit:
			if len(frame.Payload) > 0 {
				return int(frame.Payload[0])
			}
			return 0
		default:
			// Unknown frame type, ignore
		}
	}
}
