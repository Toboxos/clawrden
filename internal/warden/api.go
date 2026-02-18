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
	mux.HandleFunc("/api/jails", api.handleJails)
	mux.HandleFunc("/api/jails/", api.handleJailByID)

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

// handleJails lists all jails or creates a new one.
func (api *APIServer) handleJails(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		api.listJails(w, r)
	case http.MethodPost:
		api.createJail(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listJails returns all active jails.
func (api *APIServer) listJails(w http.ResponseWriter, r *http.Request) {
	jailhouse := api.warden.GetJailhouse()
	if jailhouse == nil {
		http.Error(w, "Jailhouse not initialized", http.StatusServiceUnavailable)
		return
	}

	jails := jailhouse.ListJails()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jails)
}

// createJail creates a new jail.
func (api *APIServer) createJail(w http.ResponseWriter, r *http.Request) {
	jailhouse := api.warden.GetJailhouse()
	if jailhouse == nil {
		http.Error(w, "Jailhouse not initialized", http.StatusServiceUnavailable)
		return
	}

	var req struct {
		JailID   string   `json:"jail_id"`
		Commands []string `json:"commands"`
		Hardened bool     `json:"hardened"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.JailID == "" {
		http.Error(w, "jail_id is required", http.StatusBadRequest)
		return
	}
	if len(req.Commands) == 0 {
		http.Error(w, "commands is required", http.StatusBadRequest)
		return
	}

	if err := jailhouse.CreateJail(req.JailID, req.Commands, req.Hardened); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create jail: %v", err), http.StatusConflict)
		return
	}

	api.logger.Printf("created jail %s via API: %v", req.JailID, req.Commands)
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{"status": "created", "jail_id": req.JailID})
}

// handleJailByID handles GET and DELETE for a specific jail.
func (api *APIServer) handleJailByID(w http.ResponseWriter, r *http.Request) {
	jailhouse := api.warden.GetJailhouse()
	if jailhouse == nil {
		http.Error(w, "Jailhouse not initialized", http.StatusServiceUnavailable)
		return
	}

	jailID := strings.TrimPrefix(r.URL.Path, "/api/jails/")
	if jailID == "" {
		http.Error(w, "jail ID is required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		jail, err := jailhouse.GetJail(jailID)
		if err != nil {
			http.Error(w, fmt.Sprintf("Jail not found: %v", err), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(jail)

	case http.MethodDelete:
		if err := jailhouse.DestroyJail(jailID); err != nil {
			http.Error(w, fmt.Sprintf("Failed to delete jail: %v", err), http.StatusNotFound)
			return
		}
		api.logger.Printf("deleted jail %s via API", jailID)
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "jail_id": jailID})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
