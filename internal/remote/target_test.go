package remote

import (
	"os"
	"os/user"
	"testing"
)

func currentUser(t *testing.T) string {
	t.Helper()
	if u := os.Getenv("USER"); u != "" {
		return u
	}
	u, err := user.Current()
	if err != nil {
		t.Fatalf("cannot determine current user: %v", err)
	}
	return u.Username
}

func TestParseTarget(t *testing.T) {
	osUser := currentUser(t)

	tests := []struct {
		name        string
		input       string
		wantErr     bool
		errContains string
		check       func(t *testing.T, got Target)
	}{
		{
			name:  "user@host:port",
			input: "user@host:2222",
			check: func(t *testing.T, got Target) {
				t.Helper()
				if got.User != "user" {
					t.Errorf("User = %q, want %q", got.User, "user")
				}
				if got.Host != "host" {
					t.Errorf("Host = %q, want %q", got.Host, "host")
				}
				if got.Port != 2222 {
					t.Errorf("Port = %d, want %d", got.Port, 2222)
				}
			},
		},
		{
			name:  "user@fqdn defaults port 22",
			input: "deploy@server.example.com",
			check: func(t *testing.T, got Target) {
				t.Helper()
				if got.User != "deploy" {
					t.Errorf("User = %q, want %q", got.User, "deploy")
				}
				if got.Host != "server.example.com" {
					t.Errorf("Host = %q, want %q", got.Host, "server.example.com")
				}
				if got.Port != 22 {
					t.Errorf("Port = %d, want %d", got.Port, 22)
				}
			},
		},
		{
			name:  "host:port defaults user to OS user",
			input: "host:2222",
			check: func(t *testing.T, got Target) {
				t.Helper()
				if got.User != osUser {
					t.Errorf("User = %q, want OS user %q", got.User, osUser)
				}
				if got.Host != "host" {
					t.Errorf("Host = %q, want %q", got.Host, "host")
				}
				if got.Port != 2222 {
					t.Errorf("Port = %d, want %d", got.Port, 2222)
				}
			},
		},
		{
			name:  "bare hostname defaults user and port",
			input: "myserver",
			check: func(t *testing.T, got Target) {
				t.Helper()
				if got.User != osUser {
					t.Errorf("User = %q, want OS user %q", got.User, osUser)
				}
				if got.Host != "myserver" {
					t.Errorf("Host = %q, want %q", got.Host, "myserver")
				}
				if got.Port != 22 {
					t.Errorf("Port = %d, want %d", got.Port, 22)
				}
			},
		},
		{
			name:  "root@ip:22",
			input: "root@10.0.0.1:22",
			check: func(t *testing.T, got Target) {
				t.Helper()
				if got.User != "root" {
					t.Errorf("User = %q, want %q", got.User, "root")
				}
				if got.Host != "10.0.0.1" {
					t.Errorf("Host = %q, want %q", got.Host, "10.0.0.1")
				}
				if got.Port != 22 {
					t.Errorf("Port = %d, want %d", got.Port, 22)
				}
			},
		},
		{
			name:        "empty string errors",
			input:       "",
			wantErr:     true,
			errContains: "host must not be empty",
		},
		{
			name:        "user@ with no host errors",
			input:       "user@",
			wantErr:     true,
			errContains: "host must not be empty",
		},
		{
			name:        "port zero out of range",
			input:       "user@host:0",
			wantErr:     true,
			errContains: "port",
		},
		{
			name:        "port 99999 out of range",
			input:       "user@host:99999",
			wantErr:     true,
			errContains: "port",
		},
		{
			name:        "non-numeric port",
			input:       "user@host:abc",
			wantErr:     true,
			errContains: "invalid port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTarget(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ParseTarget(%q) succeeded, want error containing %q", tt.input, tt.errContains)
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Fatalf("ParseTarget(%q) error = %q, want substring %q", tt.input, err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseTarget(%q) unexpected error: %v", tt.input, err)
			}
			if tt.check != nil {
				tt.check(t, got)
			}
		})
	}
}

func TestTargetString(t *testing.T) {
	target := Target{User: "admin", Host: "server", Port: 22}
	got := target.String()
	want := "admin@server:22"
	if got != want {
		t.Errorf("Target.String() = %q, want %q", got, want)
	}
}

func TestShellEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "absolute path",
			input: "/var/log/app.log",
			want:  "'/var/log/app.log'",
		},
		{
			name:  "single quote in string",
			input: "it's a file",
			want:  "'it'\\''s a file'",
		},
		{
			name:  "simple word",
			input: "simple",
			want:  "'simple'",
		},
		{
			name:  "empty string",
			input: "",
			want:  "''",
		},
		{
			name:  "string with spaces",
			input: "hello world",
			want:  "'hello world'",
		},
		{
			name:  "command substitution neutralized",
			input: "$(rm -rf /)",
			want:  "'$(rm -rf /)'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShellEscape(tt.input)
			if got != tt.want {
				t.Errorf("ShellEscape(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// contains reports whether s contains substr (case-sensitive).
func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
