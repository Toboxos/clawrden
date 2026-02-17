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
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Config holds the configuration for the Warden server.
type Config struct {
	SocketPath string
	PolicyPath string
	PrisonerID string
	AuditPath  string
	APIAddr    string
	Logger     *log.Logger
}

// Server is the Warden supervisor.
type Server struct {
	config   Config
	listener net.Listener
	policy   *PolicyEngine
	hitl     *HITLQueue
	executor executor.Executor
	audit    *AuditLogger
	api      *APIServer
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

	// Create audit logger
	auditLogger, err := NewAuditLogger(cfg.AuditPath)
	if err != nil {
		return nil, fmt.Errorf("create audit logger: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	srv := &Server{
		config:   cfg,
		policy:   policy,
		hitl:     NewHITLQueue(),
		executor: exec,
		audit:    auditLogger,
		logger:   cfg.Logger,
		ctx:      ctx,
		cancel:   cancel,
	}

	// Create HTTP API server if address is provided
	if cfg.APIAddr != "" {
		srv.api = NewAPIServer(srv, cfg.APIAddr, cfg.Logger)
	}

	return srv, nil
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

	// Start HTTP API server if configured
	if s.api != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			if err := s.api.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				s.logger.Printf("HTTP API server error: %v", err)
			}
		}()
	}

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
	if s.api != nil {
		s.api.Shutdown()
	}
	if s.listener != nil {
		s.listener.Close()
	}
	s.wg.Wait()
	if s.audit != nil {
		s.audit.Close()
	}
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

	// Prepare audit entry
	startTime := time.Now()
	auditEntry := AuditEntry{
		Command:  req.Command,
		Args:     req.Args,
		Cwd:      req.Cwd,
		Identity: req.Identity,
	}

	// Validate path security boundary using policy
	if err := s.policy.ValidatePath(req.Cwd); err != nil {
		s.logger.Printf("SECURITY: %v", err)
		auditEntry.Decision = "deny (path violation)"
		auditEntry.Error = err.Error()
		s.audit.Log(auditEntry)
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
		auditEntry.Decision = "deny"
		s.audit.Log(auditEntry)
		protocol.WriteAck(conn, protocol.AckDenied)
		return

	case ActionAsk:
		protocol.WriteAck(conn, protocol.AckPendingHITL)

		// Enqueue for human approval
		decision := s.hitl.Enqueue(connCtx, req)
		if decision == DecisionDeny {
			auditEntry.Decision = "deny (after HITL)"
			s.audit.Log(auditEntry)
			protocol.WriteAck(conn, protocol.AckDenied)
			return
		}
		// Approved — send allowed ack and proceed
		auditEntry.Decision = "allow (after HITL)"
		protocol.WriteAck(conn, protocol.AckAllowed)

	case ActionAllow:
		auditEntry.Decision = "allow"
		protocol.WriteAck(conn, protocol.AckAllowed)
	}

	// Execute the command
	execErr := s.executor.Execute(connCtx, req, conn)

	// Calculate duration and update audit entry
	auditEntry.Duration = float64(time.Since(startTime).Milliseconds())

	if execErr != nil {
		s.logger.Printf("execution error: %v", execErr)
		auditEntry.ExitCode = 1
		auditEntry.Error = execErr.Error()
		s.audit.Log(auditEntry)

		// Send error via stderr frame
		protocol.WriteFrame(conn, protocol.Frame{
			Type:    protocol.StreamStderr,
			Payload: []byte(fmt.Sprintf("clawrden: execution error: %v\n", execErr)),
		})
		protocol.WriteExitCode(conn, 1)
		return
	}

	// Success case
	auditEntry.ExitCode = 0
	s.audit.Log(auditEntry)
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
