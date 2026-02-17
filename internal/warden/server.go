// Package warden implements the Clawrden supervisor server.
// It listens on a Unix Domain Socket, evaluates policy,
// and dispatches commands to the appropriate executor.
package warden

import (
	"clawrden/internal/executor"
	"clawrden/pkg/protocol"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
)

// Config holds the configuration for the Warden server.
type Config struct {
	SocketPath string
	PolicyPath string
	PrisonerID string
	Logger     *log.Logger
}

// Server is the Warden supervisor.
type Server struct {
	config   Config
	listener net.Listener
	policy   *PolicyEngine
	hitl     *HITLQueue
	executor executor.Executor
	logger   *log.Logger

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewServer creates a new Warden server with the given configuration.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Logger == nil {
		cfg.Logger = log.New(os.Stdout, "[warden] ", log.LstdFlags|log.Lmsgprefix)
	}

	// Load policy
	policy, err := LoadPolicy(cfg.PolicyPath)
	if err != nil {
		cfg.Logger.Printf("warning: could not load policy from %s: %v (using default deny-all)", cfg.PolicyPath, err)
		policy = DefaultPolicy()
	}

	// Create executor
	exec, err := executor.New(executor.Config{
		PrisonerContainerID: cfg.PrisonerID,
		Logger:              cfg.Logger,
	})
	if err != nil {
		// If Docker is not available, create a local executor for development
		cfg.Logger.Printf("warning: docker executor unavailable: %v (using local executor)", err)
		exec = executor.NewLocalExecutor(cfg.Logger)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Server{
		config:   cfg,
		policy:   policy,
		hitl:     NewHITLQueue(),
		executor: exec,
		logger:   cfg.Logger,
		ctx:      ctx,
		cancel:   cancel,
	}, nil
}

// ListenAndServe starts the Unix socket listener and accepts connections.
func (s *Server) ListenAndServe() error {
	// Remove existing socket file if it exists
	os.Remove(s.config.SocketPath)

	// Ensure the socket directory exists
	socketDir := s.config.SocketPath[:strings.LastIndex(s.config.SocketPath, "/")]
	if err := os.MkdirAll(socketDir, 0755); err != nil {
		return fmt.Errorf("create socket directory: %w", err)
	}

	var err error
	s.listener, err = net.Listen("unix", s.config.SocketPath)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", s.config.SocketPath, err)
	}
	defer s.listener.Close()

	// Make the socket accessible
	if err := os.Chmod(s.config.SocketPath, 0666); err != nil {
		s.logger.Printf("warning: could not chmod socket: %v", err)
	}

	s.logger.Printf("listening on %s", s.config.SocketPath)

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.ctx.Done():
				return nil // Clean shutdown
			default:
				s.logger.Printf("accept error: %v", err)
				continue
			}
		}

		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConnection(conn)
		}()
	}
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown() {
	s.cancel()
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
}

// handleConnection processes a single shim connection.
func (s *Server) handleConnection(conn net.Conn) {
	defer conn.Close()

	connCtx, connCancel := context.WithCancel(s.ctx)
	defer connCancel()

	// Monitor for cancel frames from the shim
	go s.monitorCancel(conn, connCancel)

	// Read the request
	req, err := protocol.ReadRequest(conn)
	if err != nil {
		s.logger.Printf("read request error: %v", err)
		return
	}

	s.logger.Printf("request: %s %v (cwd=%s, uid=%d)",
		req.Command, req.Args, req.Cwd, req.Identity.UID)

	// Validate path security boundary
	if !strings.HasPrefix(req.Cwd, "/app") && req.Cwd != "/" {
		s.logger.Printf("SECURITY: rejected request with cwd outside /app: %s", req.Cwd)
		protocol.WriteAck(conn, protocol.AckDenied)
		return
	}

	// Scrub the environment
	req.Env = ScrubEnvironment(req.Env)

	// Evaluate policy
	action := s.policy.Evaluate(req)
	s.logger.Printf("policy decision: %s for %s", action, req.Command)

	switch action {
	case ActionDeny:
		protocol.WriteAck(conn, protocol.AckDenied)
		return

	case ActionAsk:
		protocol.WriteAck(conn, protocol.AckPendingHITL)

		// Enqueue for human approval
		decision := s.hitl.Enqueue(connCtx, req)
		if decision == DecisionDeny {
			protocol.WriteAck(conn, protocol.AckDenied)
			return
		}
		// Approved — send allowed ack and proceed
		protocol.WriteAck(conn, protocol.AckAllowed)

	case ActionAllow:
		protocol.WriteAck(conn, protocol.AckAllowed)
	}

	// Execute the command
	if err := s.executor.Execute(connCtx, req, conn); err != nil {
		s.logger.Printf("execution error: %v", err)
		// Send error via stderr frame
		protocol.WriteFrame(conn, protocol.Frame{
			Type:    protocol.StreamStderr,
			Payload: []byte(fmt.Sprintf("clawrden: execution error: %v\n", err)),
		})
		protocol.WriteExitCode(conn, 1)
		return
	}
}

// monitorCancel watches for cancel frames from the shim.
// This runs in a separate goroutine reading from a buffered copy of the connection,
// but in practice the shim only sends cancel frames after the initial request.
func (s *Server) monitorCancel(conn net.Conn, cancel context.CancelFunc) {
	// This is a simplified version. In a production system, we'd multiplex
	// the connection to separate reads between the main handler and this monitor.
	// For now, cancellation is handled via connection close detection.
	buf := make([]byte, 1)
	for {
		_, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				s.logger.Printf("cancel monitor: connection error: %v", err)
			}
			cancel()
			return
		}
	}
}

// GetHITLQueue returns the HITL queue for external access (e.g., from a web UI).
func (s *Server) GetHITLQueue() *HITLQueue {
	return s.hitl
}
