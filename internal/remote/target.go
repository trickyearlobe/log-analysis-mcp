// Package remote provides SSH connection target parsing and shell utilities.
package remote

import (
	"fmt"
	"log/slog"
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

	// SSHConfig holds the resolved SSH config for this target's host.
	// Populated during ParseTarget so callers (e.g. ClientPool.Get) can
	// access IdentityAgent and IdentityFile without re-resolving.
	SSHConfig SSHHostConfig
}

// String returns the canonical "user@host:port" form.
func (t Target) String() string {
	return fmt.Sprintf("%s@%s:%d", t.User, t.Host, t.Port)
}

// ParseTarget parses a string in [user@]host[:port] format into a Target.
// After parsing explicit values, it resolves ~/.ssh/config for the host.
// Precedence: explicit input > SSH config > OS defaults.
func ParseTarget(s string) (Target, error) {
	var t Target

	remaining := s
	userExplicit := false
	portExplicit := false

	// Extract user if present
	if idx := strings.Index(remaining, "@"); idx != -1 {
		t.User = remaining[:idx]
		remaining = remaining[idx+1:]
		userExplicit = true
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
		portExplicit = true
	} else {
		t.Host = remaining
	}

	// Validate host before SSH config lookup
	if t.Host == "" {
		return Target{}, fmt.Errorf("remote: parse target %q: host must not be empty", s)
	}

	// Resolve SSH config for this host
	t.SSHConfig = ResolveSSHConfig(t.Host)

	// Apply SSH config defaults for fields not explicitly provided
	if !userExplicit && t.SSHConfig.User != "" {
		t.User = t.SSHConfig.User
		slog.Info("remote: target: using User from SSH config", "host", t.Host, "user", t.User)
	}

	if !portExplicit && t.SSHConfig.Port != 0 {
		t.Port = t.SSHConfig.Port
		slog.Info("remote: target: using Port from SSH config", "host", t.Host, "port", t.Port)
	}

	// Fall back to OS defaults for anything still unset
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

	if !portExplicit && t.Port == 0 {
		t.Port = 22
	}

	// Validate
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
