package remote

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kevinburke/ssh_config"
)

// SSHHostConfig holds resolved SSH config values for a specific host.
// Zero-value fields indicate the directive was not set in the config.
type SSHHostConfig struct {
	User          string
	Port          int
	IdentityAgent string
	IdentityFile  string
}

// ResolveSSHConfig queries ~/.ssh/config for directives matching the given host.
// It returns resolved values with ~ expanded in paths. Fields that are not
// configured in the SSH config remain at their zero value.
func ResolveSSHConfig(host string) SSHHostConfig {
	var result SSHHostConfig

	home, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("remote: sshconfig: cannot determine home directory", "error", err)
		return result
	}

	configPath := filepath.Join(home, ".ssh", "config")
	f, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Info("remote: sshconfig: no config file found", "path", configPath)
		} else {
			slog.Warn("remote: sshconfig: failed to open config", "path", configPath, "error", err)
		}
		return result
	}
	defer f.Close()

	cfg, err := ssh_config.Decode(f)
	if err != nil {
		slog.Warn("remote: sshconfig: failed to parse config", "path", configPath, "error", err)
		return result
	}

	// Resolve User
	if user, err := cfg.Get(host, "User"); err == nil && user != "" {
		result.User = user
		slog.Info("remote: sshconfig: resolved User", "host", host, "user", user)
	}

	// Resolve Port
	if portStr, err := cfg.Get(host, "Port"); err == nil && portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p >= 1 && p <= 65535 {
			result.Port = p
			slog.Info("remote: sshconfig: resolved Port", "host", host, "port", p)
		} else {
			slog.Warn("remote: sshconfig: invalid Port value", "host", host, "port", portStr)
		}
	}

	// Resolve IdentityAgent
	if agent, err := cfg.Get(host, "IdentityAgent"); err == nil && agent != "" {
		result.IdentityAgent = expandTilde(agent, home)
		slog.Info("remote: sshconfig: resolved IdentityAgent", "host", host, "path", result.IdentityAgent)
	}

	// Resolve IdentityFile
	if identFile, err := cfg.Get(host, "IdentityFile"); err == nil && identFile != "" {
		result.IdentityFile = expandTilde(identFile, home)
		slog.Info("remote: sshconfig: resolved IdentityFile", "host", host, "path", result.IdentityFile)
	}

	return result
}

// expandTilde replaces a leading ~ or ~/ with the user's home directory.
func expandTilde(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}
