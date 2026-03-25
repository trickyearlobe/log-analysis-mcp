// Package remote provides SSH connection target parsing and shell utilities.
package remote

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
)

// Target represents a parsed SSH connection target.
type Target struct {
	User string
	Host string
	Port int
}

// String returns the canonical "user@host:port" form.
func (t Target) String() string {
	return fmt.Sprintf("%s@%s:%d", t.User, t.Host, t.Port)
}

// ParseTarget parses a string in [user@]host[:port] format into a Target.
func ParseTarget(s string) (Target, error) {
	var t Target

	remaining := s

	// Extract user if present
	if idx := strings.Index(remaining, "@"); idx != -1 {
		t.User = remaining[:idx]
		remaining = remaining[idx+1:]
	}

	// Extract host and optional port
	if idx := strings.LastIndex(remaining, ":"); idx != -1 {
		t.Host = remaining[:idx]
		portStr := remaining[idx+1:]
		p, err := strconv.Atoi(portStr)
		if err != nil {
			return Target{}, fmt.Errorf("remote: parse target %q: invalid port %q: %w", s, portStr, err)
		}
		t.Port = p
	} else {
		t.Host = remaining
		t.Port = 22
	}

	// Default user from OS if not provided
	if t.User == "" {
		t.User = os.Getenv("USER")
		if t.User == "" {
			u, err := user.Current()
			if err != nil {
				return Target{}, fmt.Errorf("remote: parse target %q: cannot determine user: %w", s, err)
			}
			t.User = u.Username
		}
	}

	// Validate
	if t.Host == "" {
		return Target{}, fmt.Errorf("remote: parse target %q: host must not be empty", s)
	}
	if t.Port < 1 || t.Port > 65535 {
		return Target{}, fmt.Errorf("remote: parse target %q: port %d out of range 1-65535", s, t.Port)
	}
	if t.User == "" {
		return Target{}, fmt.Errorf("remote: parse target %q: user must not be empty", s)
	}

	return t, nil
}

// ShellEscape wraps a string in single quotes for safe use in sh commands.
func ShellEscape(s string) string {
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	return "'" + escaped + "'"
}
