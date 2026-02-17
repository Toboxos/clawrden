package executor

import (
	"bufio"
	"clawrden/pkg/protocol"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
)

// LocalExecutor runs commands directly on the host.
// This is used for development and testing when Docker is not available.
type LocalExecutor struct {
	logger *log.Logger
}

// NewLocalExecutor creates a local command executor.
func NewLocalExecutor(logger *log.Logger) *LocalExecutor {
	if logger == nil {
		logger = log.New(os.Stdout, "[local-exec] ", log.LstdFlags|log.Lmsgprefix)
	}
	return &LocalExecutor{logger: logger}
}

// Execute runs the command locally and streams output.
func (le *LocalExecutor) Execute(ctx context.Context, req *protocol.Request, conn net.Conn) error {
	if err := ValidatePath(req.Cwd); err != nil {
		return err
	}

	le.logger.Printf("local exec: %s %v (cwd=%s)", req.Command, req.Args, req.Cwd)

	// Find the real binary (skip our own shims)
	cmdPath, err := le.findRealBinary(req.Command)
	if err != nil {
		return fmt.Errorf("find binary %q: %w", req.Command, err)
	}

	cmd := exec.CommandContext(ctx, cmdPath, req.Args...)
	cmd.Dir = req.Cwd
	cmd.Env = req.Env

	// Set up pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	// Stream stdout in a goroutine
	done := make(chan struct{}, 2)
	go func() {
		defer func() { done <- struct{}{} }()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := append(scanner.Bytes(), '\n')
			protocol.WriteFrame(conn, protocol.Frame{
				Type:    protocol.StreamStdout,
				Payload: line,
			})
		}
	}()

	// Stream stderr in a goroutine
	go func() {
		defer func() { done <- struct{}{} }()
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			line := append(scanner.Bytes(), '\n')
			protocol.WriteFrame(conn, protocol.Frame{
				Type:    protocol.StreamStderr,
				Payload: line,
			})
		}
	}()

	// Wait for both streams to finish
	<-done
	<-done

	// Wait for the command to exit
	exitCode := 0
	if err := cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			le.logger.Printf("wait error: %v", err)
			exitCode = 1
		}
	}

	return protocol.WriteExitCode(conn, exitCode)
}

// findRealBinary locates the actual binary, skipping shim paths.
func (le *LocalExecutor) findRealBinary(name string) (string, error) {
	// Look in standard locations, skipping /clawrden/bin
	paths := []string{
		"/usr/local/bin/" + name,
		"/usr/bin/" + name,
		"/bin/" + name,
		"/usr/local/sbin/" + name,
		"/usr/sbin/" + name,
		"/sbin/" + name,
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	// Fall back to PATH lookup
	return exec.LookPath(name)
}
