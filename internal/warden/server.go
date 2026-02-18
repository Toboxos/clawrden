// Package warden implements the Clawrden supervisor server.
// It listens on a Unix Domain Socket, evaluates policy,
// and dispatches commands to the appropriate executor.
package warden

import (
	"clawrden/internal/executor"
	"clawrden/internal/jailhouse"
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

	"github.com/docker/docker/client"
)

// Config holds the configuration for the Warden server.
type Config struct {
	SocketPath      string
	PolicyPath      string
	AuditPath       string
	APIAddr         string
	Logger          *log.Logger
	JailhouseArmory string // Path to armory (default: /var/lib/clawrden/armory)
	JailhouseRoot   string // Path to jailhouse root (default: /var/lib/clawrden/jailhouse)
	JailhouseState  string // Path to state file (default: /var/lib/clawrden/jailhouse.state.json)
}

// Server is the Warden supervisor.
type Server struct {
	config   Config
	listener net.Listener
	policy   *PolicyEngine
	hitl     *HITLQueue
	audit    *AuditLogger
	api      *APIServer
	logger   *log.Logger

	// Executors: dockerExec for containerized requests, localExec for host/dev
	dockerExec *executor.DockerExecutor // nil if Docker unavailable
	localExec  *executor.LocalExecutor

	// Jailhouse components
	jailhouse     *jailhouse.Manager
	policyWatcher *PolicyWatcher

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

	ctx, cancel := context.WithCancel(context.Background())

	srv := &Server{
		config: cfg,
		policy: policy,
		hitl:   NewHITLQueue(),
		logger: cfg.Logger,
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize jailhouse (always enabled)
	if err := srv.initializeJailhouse(); err != nil {
		// Jailhouse initialization failure is not fatal, but log it
		cfg.Logger.Printf("warning: failed to initialize jailhouse: %v", err)
	}

	// Create executors — Docker for containerized requests, local as fallback
	srv.localExec = executor.NewLocalExecutor(cfg.Logger)

	dockerClient, dockerErr := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if dockerErr != nil {
		cfg.Logger.Printf("warning: docker unavailable: %v (mirror execution disabled)", dockerErr)
	} else {
		srv.dockerExec = executor.NewDockerExecutor(dockerClient, cfg.Logger)
	}

	// Create audit logger
	auditLogger, err := NewAuditLogger(cfg.AuditPath)
	if err != nil {
		return nil, fmt.Errorf("create audit logger: %w", err)
	}

	srv.audit = auditLogger

	// Create HTTP API server if address is provided
	if cfg.APIAddr != "" {
		srv.api = NewAPIServer(srv, cfg.APIAddr, cfg.Logger)
	}

	return srv, nil
}

// initializeJailhouse sets up the jailhouse manager and creates jails from policy config.
func (s *Server) initializeJailhouse() error {
	// Set default paths if not specified
	armoryPath := s.config.JailhouseArmory
	if armoryPath == "" {
		armoryPath = "/var/lib/clawrden/armory"
	}

	jailhousePath := s.config.JailhouseRoot
	if jailhousePath == "" {
		jailhousePath = "/var/lib/clawrden/jailhouse"
	}

	statePath := s.config.JailhouseState
	if statePath == "" {
		statePath = "/var/lib/clawrden/jailhouse.state.json"
	}

	// Create jailhouse manager
	jailhouseMgr, err := jailhouse.NewManager(jailhouse.Config{
		ArmoryPath:    armoryPath,
		JailhousePath: jailhousePath,
		StatePath:     statePath,
		Logger:        s.logger,
	})
	if err != nil {
		return fmt.Errorf("create jailhouse manager: %w", err)
	}

	// Start jailhouse manager
	if err := jailhouseMgr.Start(); err != nil {
		return fmt.Errorf("start jailhouse manager: %w", err)
	}

	s.jailhouse = jailhouseMgr

	// Create jails from policy config
	for jailID, cfg := range s.policy.GetJails() {
		if _, err := s.jailhouse.GetJail(jailID); err == nil {
			s.logger.Printf("jail %s already exists (from persisted state), skipping", jailID)
			continue
		}
		if err := s.jailhouse.CreateJail(jailID, cfg.Commands, cfg.Hardened); err != nil {
			s.logger.Printf("warning: failed to create jail %s: %v", jailID, err)
		} else {
			s.logger.Printf("created jail %s: %v", jailID, cfg.Commands)
		}
	}

	// Create policy watcher for hot-reload
	if s.config.PolicyPath != "" {
		policyWatcher, err := NewPolicyWatcher(s.config.PolicyPath, s.policy, s.logger)
		if err != nil {
			s.logger.Printf("warning: failed to create policy watcher: %v (hot-reload disabled)", err)
		} else {
			s.policyWatcher = policyWatcher

			// Register callback to update server's policy reference
			s.policyWatcher.OnReload(func(newPolicy *PolicyEngine) {
				s.policy = newPolicy
				s.logger.Printf("server policy updated after hot-reload")
			})
		}
	}

	s.logger.Printf("jailhouse initialized (armory=%s, jailhouse=%s)", armoryPath, jailhousePath)
	return nil
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

	// Start policy watcher if enabled
	if s.policyWatcher != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			if err := s.policyWatcher.Start(s.ctx); err != nil {
				s.logger.Printf("Policy watcher error: %v", err)
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
	if s.policyWatcher != nil {
		s.policyWatcher.Stop()
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

	// Extract peer credentials (kernel-enforced, unfakeable)
	peerCreds, peerErr := extractPeerCreds(conn)
	if peerErr != nil {
		s.logger.Printf("warning: could not extract peer credentials: %v", peerErr)
		// Continue without peer creds — local/dev mode will still work
	}

	// Monitor for cancel frames from the shim
	go s.monitorCancel(conn, connCancel)

	// Read the request
	req, err := protocol.ReadRequest(conn)
	if err != nil {
		s.logger.Printf("read request error: %v", err)
		return
	}

	// Resolve container ID from peer credentials
	if peerCreds != nil {
		// Override self-reported identity with kernel-enforced values
		req.Identity.UID = int(peerCreds.UID)
		req.Identity.GID = int(peerCreds.GID)

		// Resolve which container the peer process belongs to
		containerID, resolveErr := resolveContainerID(peerCreds.PID)
		if resolveErr != nil {
			s.logger.Printf("warning: could not resolve container ID for pid %d: %v", peerCreds.PID, resolveErr)
		} else if containerID != "" {
			req.ContainerID = containerID
		}
	}

	s.logger.Printf("request: %s %v (cwd=%s, uid=%d, container=%s)",
		req.Command, req.Args, req.Cwd, req.Identity.UID, truncateID(req.ContainerID))

	// Prepare audit entry
	startTime := time.Now()
	auditEntry := AuditEntry{
		Command:     req.Command,
		Args:        req.Args,
		Cwd:         req.Cwd,
		Identity:    req.Identity,
		ContainerID: req.ContainerID,
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
	evalResult := s.policy.Evaluate(req)
	s.logger.Printf("policy decision: %s for %s (timeout: %v)", evalResult.Action, req.Command, evalResult.Timeout)

	switch evalResult.Action {
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

	// Execute the command with timeout
	execCtx := connCtx
	var execCancel context.CancelFunc
	if evalResult.Timeout > 0 {
		execCtx, execCancel = context.WithTimeout(connCtx, evalResult.Timeout)
		defer execCancel()
	}

	// Select executor: Docker mirror for containerized requests, local for host/dev
	var exec executor.Executor
	if req.ContainerID != "" && s.dockerExec != nil {
		exec = s.dockerExec
	} else {
		exec = s.localExec
	}

	execErr := exec.Execute(execCtx, req, conn)

	// Calculate duration and update audit entry
	auditEntry.Duration = float64(time.Since(startTime).Milliseconds())

	if execErr != nil {
		s.logger.Printf("execution error: %v", execErr)
		auditEntry.ExitCode = 1
		auditEntry.Error = execErr.Error()

		// Check if this was a timeout violation
		if execCtx.Err() == context.DeadlineExceeded {
			auditEntry.TimeoutViolation = true
			auditEntry.Error = fmt.Sprintf("timeout exceeded (%v): %v", evalResult.Timeout, execErr)
			s.logger.Printf("TIMEOUT: command %s exceeded %v", req.Command, evalResult.Timeout)
		}

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
func (s *Server) monitorCancel(conn net.Conn, cancel context.CancelFunc) {
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

// GetJailhouse returns the jailhouse manager for external access (e.g., from the API).
func (s *Server) GetJailhouse() *jailhouse.Manager {
	return s.jailhouse
}

// truncateID returns the first 12 characters of a container ID, or "(host)" if empty.
func truncateID(id string) string {
	if id == "" {
		return "(host)"
	}
	if len(id) > 12 {
		return id[:12]
	}
	return id
}
