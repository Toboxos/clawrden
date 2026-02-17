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
	prisonerID := flag.String("prisoner-id", "", "Docker container ID of the Prisoner")
	flag.Parse()

	if *prisonerID == "" {
		// Try environment variable
		*prisonerID = os.Getenv("CLAWRDEN_PRISONER_ID")
	}

	logger := log.New(os.Stdout, "[warden] ", log.LstdFlags|log.Lmsgprefix)

	srv, err := warden.NewServer(warden.Config{
		SocketPath: *socketPath,
		PolicyPath: *policyPath,
		PrisonerID: *prisonerID,
		Logger:     logger,
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
