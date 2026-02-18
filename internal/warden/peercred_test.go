package warden

import (
	"testing"
)

func TestParseContainerIDFromCgroup(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name: "cgroup v1 docker",
			content: `12:memory:/docker/a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2
11:cpuset:/docker/a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2
`,
			expected: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		},
		{
			name: "cgroup v2 docker scope",
			content: `0::/system.slice/docker-a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2.scope
`,
			expected: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		},
		{
			name: "host process (no container)",
			content: `12:memory:/user.slice/user-1000.slice/session-1.scope
11:cpuset:/
0::/user.slice/user-1000.slice/session-1.scope
`,
			expected: "",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "",
		},
		{
			name: "kubernetes containerd",
			content: `0::/kubepods/besteffort/pod12345/a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2
`,
			expected: "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		},
		{
			name: "cgroup v1 mixed lines",
			content: `12:memory:/
11:cpuset:/
10:blkio:/docker/deadbeef0123456789abcdef0123456789abcdef0123456789abcdef01234567
9:net_cls:/
`,
			expected: "deadbeef0123456789abcdef0123456789abcdef0123456789abcdef01234567",
		},
		{
			name: "short hex not 64 chars",
			content: `0::/docker/abc123
`,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseContainerIDFromCgroup(tt.content)
			if got != tt.expected {
				t.Errorf("parseContainerIDFromCgroup() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestTruncateID(t *testing.T) {
	tests := []struct {
		id       string
		expected string
	}{
		{"", "(host)"},
		{"abc", "abc"},
		{"a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2", "a1b2c3d4e5f6"},
	}

	for _, tt := range tests {
		got := truncateID(tt.id)
		if got != tt.expected {
			t.Errorf("truncateID(%q) = %q, want %q", tt.id, got, tt.expected)
		}
	}
}
