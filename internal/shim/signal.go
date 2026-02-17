package shim

import (
	"clawrden/pkg/protocol"
	"net"
	"os"
	"os/signal"
	"syscall"
)

// cancelSignals sets up signal handlers for SIGINT and SIGTERM.
// When received, it sends a cancel frame to the Warden and closes the connection.
func cancelSignals(conn net.Conn) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigCh

		// Best-effort: send a cancel frame to the Warden
		_ = protocol.WriteFrame(conn, protocol.Frame{
			Type:    protocol.StreamCancel,
			Payload: nil,
		})

		// Close the connection to unblock any pending reads
		conn.Close()

		// Exit with signal-killed code (128 + signal number)
		os.Exit(130) // 128 + SIGINT(2)
	}()
}
