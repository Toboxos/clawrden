package warden

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// containerIDPattern matches a 64-character hex string (Docker container ID).
var containerIDPattern = regexp.MustCompile(`[a-f0-9]{64}`)

// resolveContainerID reads /proc/<pid>/cgroup to extract the Docker container ID.
// Returns the 64-char hex container ID, or "" if the process is not in a container.
// Returns error only on actual read failures.
func resolveContainerID(pid int32) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return "", fmt.Errorf("read cgroup for pid %d: %w", pid, err)
	}
	return parseContainerIDFromCgroup(string(data)), nil
}

// parseContainerIDFromCgroup extracts a Docker container ID from cgroup file contents.
// Supports cgroup v1 (/docker/<id>) and cgroup v2 (docker-<id>.scope).
// Returns "" if no container ID is found (host process).
func parseContainerIDFromCgroup(cgroupContent string) string {
	for _, line := range strings.Split(cgroupContent, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// cgroup v1: "12:memory:/docker/<64-hex-id>"
		// cgroup v1: "12:memory:/system.slice/docker-<64-hex-id>.scope"
		// cgroup v2: "0::/system.slice/docker-<64-hex-id>.scope"
		if strings.Contains(line, "docker") {
			if id := containerIDPattern.FindString(line); id != "" {
				return id
			}
		}

		// Kubernetes/containerd: "0::/kubepods/pod<uuid>/<64-hex-id>"
		if strings.Contains(line, "kubepods") {
			if id := containerIDPattern.FindString(line); id != "" {
				return id
			}
		}
	}
	return ""
}
