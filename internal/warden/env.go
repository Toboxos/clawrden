package warden

import (
	"strings"
)

// Environment variable security controls.
// The Warden must filter the Prisoner's environment to prevent
// confused deputy attacks and privilege escalation.

// envAllowlist contains variables that are safe to pass through.
var envAllowlist = map[string]bool{
	"PATH":     true,
	"LANG":     true,
	"LANGUAGE": true,
	"LC_ALL":   true,
	"TERM":     true,
	"HOME":     true,
	"USER":     true,
	"SHELL":    true,
	"NODE_ENV": true,
	"GOPATH":   true,
	"GOROOT":   true,
	"PYTHONPATH": true,
}

// envBlocklist contains variables that must NEVER be passed through,
// even if they appear in the allowlist (belt-and-suspenders).
var envBlocklist = map[string]bool{
	"LD_PRELOAD":    true,
	"LD_LIBRARY_PATH": true,
	"DOCKER_HOST":   true,
	"KUBECONFIG":    true,
	"AWS_ACCESS_KEY_ID":     true,
	"AWS_SECRET_ACCESS_KEY": true,
	"GOOGLE_APPLICATION_CREDENTIALS": true,
	"CLAWRDEN_SOCKET": true, // Prevent the prisoner from discovering/manipulating our socket
}

// ScrubEnvironment filters environment variables through the allowlist
// and blocklist to prevent security issues.
func ScrubEnvironment(env []string) []string {
	scrubbed := make([]string, 0, len(env))

	for _, entry := range env {
		key := envKey(entry)

		// Check blocklist first (highest priority)
		if envBlocklist[key] {
			continue
		}

		// Only pass through allowlisted variables
		if envAllowlist[key] {
			scrubbed = append(scrubbed, entry)
		}
	}

	return scrubbed
}

// envKey extracts the key from a "KEY=VALUE" environment entry.
func envKey(entry string) string {
	if idx := strings.IndexByte(entry, '='); idx >= 0 {
		return entry[:idx]
	}
	return entry
}
