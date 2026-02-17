package warden

import (
	"testing"
)

func TestPolicyValidatePath(t *testing.T) {
	tests := []struct {
		name         string
		allowedPaths []string
		path         string
		wantErr      bool
	}{
		{
			name:         "simple prefix - allowed",
			allowedPaths: []string{"/app/*"},
			path:         "/app/test",
			wantErr:      false,
		},
		{
			name:         "simple prefix - nested allowed",
			allowedPaths: []string{"/app/*"},
			path:         "/app/sub/dir/file.txt",
			wantErr:      false,
		},
		{
			name:         "simple prefix - denied",
			allowedPaths: []string{"/app/*"},
			path:         "/etc/passwd",
			wantErr:      true,
		},
		{
			name:         "multiple patterns - first match",
			allowedPaths: []string{"/app/*", "/tmp/*"},
			path:         "/app/test",
			wantErr:      false,
		},
		{
			name:         "multiple patterns - second match",
			allowedPaths: []string{"/app/*", "/tmp/*"},
			path:         "/tmp/test",
			wantErr:      false,
		},
		{
			name:         "multiple patterns - no match",
			allowedPaths: []string{"/app/*", "/tmp/*"},
			path:         "/home/user/file",
			wantErr:      true,
		},
		{
			name:         "wildcard in middle - allowed",
			allowedPaths: []string{"/home/*/workspace/*"},
			path:         "/home/alice/workspace/project",
			wantErr:      false,
		},
		{
			name:         "wildcard in middle - denied",
			allowedPaths: []string{"/home/*/workspace/*"},
			path:         "/home/alice/documents/file",
			wantErr:      true,
		},
		{
			name:         "exact match",
			allowedPaths: []string{"/app"},
			path:         "/app",
			wantErr:      false,
		},
		{
			name:         "exact match with trailing slash",
			allowedPaths: []string{"/app"},
			path:         "/app/",
			wantErr:      false, // filepath.Clean removes trailing slash
		},
		{
			name:         "empty allowed paths - allow all",
			allowedPaths: []string{},
			path:         "/anywhere/at/all",
			wantErr:      false,
		},
		{
			name:         "root path with pattern",
			allowedPaths: []string{"/app/*"},
			path:         "/app",
			wantErr:      false, // prefix match should allow /app itself
		},
		{
			name:         "complex glob pattern",
			allowedPaths: []string{"/var/lib/*/data/*"},
			path:         "/var/lib/myapp/data/file.txt",
			wantErr:      false,
		},
		{
			name:         "path traversal attempt - blocked",
			allowedPaths: []string{"/app/*"},
			path:         "/app/../etc/passwd",
			wantErr:      true, // Clean resolves to /etc/passwd
		},
		{
			name:         "multiple specific paths",
			allowedPaths: []string{"/opt/data/*", "/var/cache/*", "/usr/local/bin/*"},
			path:         "/var/cache/temp",
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pe := &PolicyEngine{
				config: PolicyConfig{
					AllowedPaths: tt.allowedPaths,
				},
			}

			err := pe.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestPolicyValidatePathEdgeCases(t *testing.T) {
	pe := &PolicyEngine{
		config: PolicyConfig{
			AllowedPaths: []string{"/app/*", "/tmp/*"},
		},
	}

	tests := []struct {
		path    string
		wantErr bool
	}{
		{"/app", false},              // Root of allowed path
		{"/app/", false},             // With trailing slash
		{"/app/sub", false},          // Subdirectory
		{"/app/sub/deep/path", false}, // Deep subdirectory
		{"/tmp", false},              // Another root
		{"/home", true},              // Not allowed
		{"/", true},                  // Root filesystem
		{"/etc", true},               // System directory
		{"app/relative", true},       // Relative path
		{"/app/../etc", true},        // Path traversal
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			err := pe.ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}
