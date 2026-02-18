// Package executor implements the command execution strategies
// used by the Warden: Mirror (exec back in originating container), Ghost (ephemeral container),
// and Local (for development/testing).
package executor

import (
	"clawrden/pkg/protocol"
	"context"
	"fmt"
	"net"
	"strings"
)

// Executor is the interface for command execution strategies.
type Executor interface {
	// Execute runs the command described in req and streams output to conn.
	// It must send an exit code frame at the end.
	Execute(ctx context.Context, req *protocol.Request, conn net.Conn) error
}

// ValidatePath checks that the working directory is within the /app boundary.
func ValidatePath(cwd string) error {
	if !strings.HasPrefix(cwd, "/app") {
		return fmt.Errorf("working directory %q is outside /app boundary", cwd)
	}
	return nil
}
