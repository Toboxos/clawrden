// Package executor implements the command execution strategies
// used by the Warden: Mirror (exec back in prisoner), Ghost (ephemeral container),
// and Local (for development/testing).
package executor

import (
	"clawrden/pkg/protocol"
	"context"
	"fmt"
	"log"
	"net"
	"strings"
)

// Executor is the interface for command execution strategies.
type Executor interface {
	// Execute runs the command described in req and streams output to conn.
	// It must send an exit code frame at the end.
	Execute(ctx context.Context, req *protocol.Request, conn net.Conn) error
}

// Config holds executor configuration.
type Config struct {
	PrisonerContainerID string
	Logger              *log.Logger
}

// New creates the appropriate executor based on configuration.
// If Docker is available and a prisoner ID is set, it returns a DockerExecutor.
// Otherwise, it falls back to a LocalExecutor.
func New(cfg Config) (Executor, error) {
	if cfg.PrisonerContainerID == "" {
		return nil, fmt.Errorf("no prisoner container ID specified")
	}

	// Try to create a Docker-based executor
	return NewDockerExecutor(cfg)
}

// ValidatePath checks that the working directory is within the /app boundary.
func ValidatePath(cwd string) error {
	if !strings.HasPrefix(cwd, "/app") {
		return fmt.Errorf("working directory %q is outside /app boundary", cwd)
	}
	return nil
}
