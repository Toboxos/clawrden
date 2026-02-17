// Package warden provides HTTP API for remote control.
package warden

import (
	"clawrden/pkg/protocol"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

//go:embed web/dashboard.html
var dashboardHTML string

// APIServer provides HTTP endpoints for warden control.
type APIServer struct {
	warden *Server
	server *http.Server
	logger *log.Logger
	mu     sync.Mutex
}

// NewAPIServer creates a new HTTP API server.
func NewAPIServer(warden *Server, addr string, logger *log.Logger) *APIServer {
	api := &APIServer{
		warden: warden,
		logger: logger,
	}

	mux := http.NewServeMux()

	// Dashboard UI
	mux.HandleFunc("/", api.handleDashboard)

	// API endpoints
	mux.HandleFunc("/api/status", api.handleStatus)
	mux.HandleFunc("/api/queue", api.handleQueue)
	mux.HandleFunc("/api/queue/", api.handleQueueAction)
	mux.HandleFunc("/api/history", api.handleHistory)
	mux.HandleFunc("/api/kill", api.handleKill)

	api.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return api
}

// ListenAndServe starts the HTTP API server.
func (api *APIServer) ListenAndServe() error {
	api.logger.Printf("HTTP API listening on %s", api.server.Addr)
	return api.server.ListenAndServe()
}

// Shutdown gracefully shuts down the API server.
func (api *APIServer) Shutdown() error {
	return api.server.Close()
}

// handleDashboard serves the web dashboard UI.
func (api *APIServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(dashboardHTML))
}

// handleStatus returns the current warden status.
func (api *APIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	queue := api.warden.GetHITLQueue()
	pending := queue.List()

	status := map[string]interface{}{
		"status":        "running",
		"pending_count": len(pending),
		"uptime":        time.Since(time.Now()).Seconds(), // TODO: track actual uptime
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleQueue lists all pending HITL requests.
func (api *APIServer) handleQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	queue := api.warden.GetHITLQueue()
	pending := queue.List()

	// Convert to JSON-friendly format
	type QueueEntry struct {
		ID       string            `json:"id"`
		Command  string            `json:"command"`
		Args     []string          `json:"args"`
		Cwd      string            `json:"cwd"`
		Identity protocol.Identity `json:"identity"`
	}

	entries := make([]QueueEntry, len(pending))
	for i, p := range pending {
		entries[i] = QueueEntry{
			ID:       p.ID,
			Command:  p.Request.Command,
			Args:     p.Request.Args,
			Cwd:      p.Request.Cwd,
			Identity: p.Request.Identity,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// handleQueueAction approves or denies a pending request.
func (api *APIServer) handleQueueAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse URL: /api/queue/{id}/approve or /api/queue/{id}/deny
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/queue/"), "/")
	if len(parts) != 2 {
		http.Error(w, "Invalid request path", http.StatusBadRequest)
		return
	}

	id := parts[0]
	action := parts[1]

	queue := api.warden.GetHITLQueue()

	switch action {
	case "approve":
		queue.Resolve(id, DecisionApprove)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "approved"})

	case "deny":
		queue.Resolve(id, DecisionDeny)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "denied"})

	default:
		http.Error(w, "Invalid action", http.StatusBadRequest)
	}
}

// handleHistory returns the command audit log.
func (api *APIServer) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read audit log from the configured path
	entries, err := ReadAuditLog(api.warden.config.AuditPath)
	if err != nil {
		api.logger.Printf("read audit log error: %v", err)
		http.Error(w, fmt.Sprintf("Failed to read audit log: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// handleKill pauses or kills the prisoner container.
func (api *APIServer) handleKill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// TODO: Implement actual container pause/kill using Docker SDK
	// For now, just acknowledge the request
	api.logger.Printf("KILL SWITCH ACTIVATED")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "acknowledged",
		"message": "Kill switch not yet implemented in executor",
	})
}
