// Command warden is the Clawrden supervisor process.
// It listens on a Unix Domain Socket, evaluates policy,
// and executes approved commands via Mirror or Ghost strategies.
package main

import (
	"clawrden/internal/warden"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	socketPath := flag.String("socket", "/var/run/clawrden/warden.sock", "Path to the Unix Domain Socket")
	policyPath := flag.String("policy", "policy.yaml", "Path to the policy configuration file")
	auditPath := flag.String("audit", "/var/log/clawrden/audit.log", "Audit log file path")
	apiAddr := flag.String("api", ":8080", "HTTP API server address")

	// Jailhouse paths (always enabled)
	armoryPath := flag.String("armory-path", "/var/lib/clawrden/armory", "Path to the armory (master shim location)")
	jailhousePath := flag.String("jailhouse-path", "/var/lib/clawrden/jailhouse", "Path to the jailhouse root directory")
	statePath := flag.String("state-path", "/var/lib/clawrden/jailhouse.state.json", "Path to the jailhouse state file")

	flag.Parse()

	logger := log.New(os.Stdout, "[warden] ", log.LstdFlags|log.Lmsgprefix)

	srv, err := warden.NewServer(warden.Config{
		SocketPath:      *socketPath,
		PolicyPath:      *policyPath,
		AuditPath:       *auditPath,
		APIAddr:         *apiAddr,
		JailhouseArmory: *armoryPath,
		JailhouseRoot:   *jailhousePath,
		JailhouseState:  *statePath,
		Logger:          logger,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "warden: failed to initialize: %v\n", err)
		os.Exit(1)
	}

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		logger.Printf("received signal %v, shutting down...", sig)
		srv.Shutdown()
	}()

	logger.Printf("starting warden on %s", *socketPath)
	if err := srv.ListenAndServe(); err != nil {
		fmt.Fprintf(os.Stderr, "warden: %v\n", err)
		os.Exit(1)
	}
}
