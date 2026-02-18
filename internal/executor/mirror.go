package executor

import (
	"clawrden/pkg/protocol"
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

// DockerExecutor uses the Docker SDK to execute commands.
// It supports both Mirror (exec in originating container) and Ghost (ephemeral container) modes.
// The target container ID is provided per-request via req.ContainerID.
type DockerExecutor struct {
	client *client.Client
	logger *log.Logger
}

// NewDockerExecutor creates a Docker-based executor.
// The executor does not hold a fixed container ID; it reads the target
// container from each request's ContainerID field (set by peer credential resolution).
func NewDockerExecutor(dockerClient *client.Client, logger *log.Logger) *DockerExecutor {
	return &DockerExecutor{
		client: dockerClient,
		logger: logger,
	}
}

// Execute runs a command using the Mirror strategy (exec back in the originating container).
// For commands requiring external tools, it falls back to Ghost strategy.
// The target container is identified by req.ContainerID.
func (de *DockerExecutor) Execute(ctx context.Context, req *protocol.Request, conn net.Conn) error {
	if err := ValidatePath(req.Cwd); err != nil {
		return err
	}

	if req.ContainerID == "" {
		return fmt.Errorf("no container ID on request (cannot mirror)")
	}

	// Determine execution strategy
	if de.shouldUseGhost(req.Command) {
		return de.executeGhost(ctx, req, conn)
	}
	return de.executeMirror(ctx, req, conn)
}

// shouldUseGhost determines if a command needs Ghost (ephemeral container) execution.
func (de *DockerExecutor) shouldUseGhost(command string) bool {
	ghostCommands := map[string]bool{
		"npm":       true,
		"npx":       true,
		"node":      true,
		"pip":       true,
		"python":    true,
		"terraform": true,
		"kubectl":   true,
		"docker":    true,
	}
	return ghostCommands[command]
}

// executeMirror runs the command back inside the Prisoner container.
func (de *DockerExecutor) executeMirror(ctx context.Context, req *protocol.Request, conn net.Conn) error {
	de.logger.Printf("mirror exec: %s %v in container %s", req.Command, req.Args, req.ContainerID)

	// Build the full command
	cmd := append([]string{req.Command}, req.Args...)

	// Determine the user to run as
	user := fmt.Sprintf("%d:%d", req.Identity.UID, req.Identity.GID)

	// Create exec configuration with UID/GID impersonation
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		WorkingDir:   req.Cwd,
		Env:          req.Env,
		AttachStdout: true,
		AttachStderr: true,
		User:         user,
	}

	// Create the exec instance
	execID, err := de.client.ContainerExecCreate(ctx, req.ContainerID, execConfig)
	if err != nil {
		return fmt.Errorf("create exec: %w", err)
	}

	// Attach to the exec instance
	resp, err := de.client.ContainerExecAttach(ctx, execID.ID, container.ExecAttachOptions{})
	if err != nil {
		return fmt.Errorf("attach exec: %w", err)
	}
	defer resp.Close()

	// Stream output to the shim
	streamDone := make(chan error, 1)
	go func() {
		streamDone <- streamDockerOutput(resp.Reader, conn)
	}()

	// Wait for streaming to complete
	select {
	case err := <-streamDone:
		if err != nil {
			de.logger.Printf("stream error: %v", err)
		}
	case <-ctx.Done():
		return ctx.Err()
	}

	// Get the exit code
	inspect, err := de.client.ContainerExecInspect(ctx, execID.ID)
	if err != nil {
		de.logger.Printf("exec inspect error: %v", err)
		return protocol.WriteExitCode(conn, 1)
	}

	return protocol.WriteExitCode(conn, inspect.ExitCode)
}

// executeGhost runs the command in an ephemeral container.
func (de *DockerExecutor) executeGhost(ctx context.Context, req *protocol.Request, conn net.Conn) error {
	de.logger.Printf("ghost exec: %s %v", req.Command, req.Args)

	// Determine the image to use
	image := de.ghostImage(req.Command)

	// Build the command
	cmd := append([]string{req.Command}, req.Args...)

	// Create the container
	containerConfig := &container.Config{
		Image:      image,
		Cmd:        cmd,
		WorkingDir: req.Cwd,
		Env:        req.Env,
	}

	hostConfig := &container.HostConfig{
		Binds: []string{
			// Mount the shared /app volume
			"clawrden_app-data:/app",
		},
	}

	resp, err := de.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return fmt.Errorf("create ghost container: %w", err)
	}

	// Ensure cleanup
	defer func() {
		removeCtx := context.Background()
		de.client.ContainerRemove(removeCtx, resp.ID, container.RemoveOptions{Force: true})
	}()

	// Attach to the container before starting
	attachResp, err := de.client.ContainerAttach(ctx, resp.ID, container.AttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return fmt.Errorf("attach ghost container: %w", err)
	}
	defer attachResp.Close()

	// Start the container
	if err := de.client.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start ghost container: %w", err)
	}

	// Stream output
	streamDone := make(chan error, 1)
	go func() {
		streamDone <- streamDockerOutput(attachResp.Reader, conn)
	}()

	// Wait for container to finish
	statusCh, errCh := de.client.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("wait ghost container: %w", err)
		}
	case status := <-statusCh:
		// Wait for streaming to complete
		<-streamDone

		// Fix file ownership (chown back to agent's UID/GID)
		de.fixOwnership(ctx, req)

		return protocol.WriteExitCode(conn, int(status.StatusCode))
	case <-ctx.Done():
		// Kill the container on cancellation
		de.client.ContainerKill(context.Background(), resp.ID, "SIGKILL")
		return ctx.Err()
	}

	return nil
}

// ghostImage returns the Docker image to use for a given command.
func (de *DockerExecutor) ghostImage(command string) string {
	images := map[string]string{
		"npm":       "node:18-alpine",
		"npx":       "node:18-alpine",
		"node":      "node:18-alpine",
		"pip":       "python:3.11-slim",
		"python":    "python:3.11-slim",
		"terraform": "hashicorp/terraform:latest",
		"kubectl":   "bitnami/kubectl:latest",
	}
	if img, ok := images[command]; ok {
		return img
	}
	return "alpine:latest"
}

// fixOwnership runs chown on /app to fix file ownership after ghost execution.
func (de *DockerExecutor) fixOwnership(ctx context.Context, req *protocol.Request) {
	cmd := []string{"chown", "-R",
		fmt.Sprintf("%d:%d", req.Identity.UID, req.Identity.GID),
		"/app",
	}

	execConfig := container.ExecOptions{
		Cmd: cmd,
	}

	execID, err := de.client.ContainerExecCreate(ctx, req.ContainerID, execConfig)
	if err != nil {
		de.logger.Printf("chown exec create error: %v", err)
		return
	}

	if err := de.client.ContainerExecStart(ctx, execID.ID, container.ExecStartOptions{}); err != nil {
		de.logger.Printf("chown exec start error: %v", err)
	}
}

// streamDockerOutput reads multiplexed Docker output and writes frames to the connection.
func streamDockerOutput(reader interface{ Read([]byte) (int, error) }, conn net.Conn) error {
	// Docker multiplexed stream format:
	// [8]byte header: [1]byte stream type, [3]byte padding, [4]byte size
	// Followed by the payload
	header := make([]byte, 8)
	for {
		_, err := reader.Read(header)
		if err != nil {
			return nil // EOF is normal
		}

		// Docker stream types: 0=stdin, 1=stdout, 2=stderr
		streamType := header[0]
		size := int(header[4])<<24 | int(header[5])<<16 | int(header[6])<<8 | int(header[7])

		if size == 0 {
			continue
		}

		payload := make([]byte, size)
		n := 0
		for n < size {
			nn, err := reader.Read(payload[n:])
			if err != nil {
				break
			}
			n += nn
		}

		var frameType byte
		switch streamType {
		case 1:
			frameType = protocol.StreamStdout
		case 2:
			frameType = protocol.StreamStderr
		default:
			continue
		}

		if err := protocol.WriteFrame(conn, protocol.Frame{
			Type:    frameType,
			Payload: payload[:n],
		}); err != nil {
			return err
		}
	}
}

func init() {
	// Suppress Docker client warnings about missing env vars
	if os.Getenv("DOCKER_HOST") == "" {
		os.Setenv("DOCKER_HOST", "unix:///var/run/docker.sock")
	}
}
