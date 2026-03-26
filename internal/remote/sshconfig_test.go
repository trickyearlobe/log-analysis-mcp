package remote

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandTilde(t *testing.T) {
	home := "/home/testuser"

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "bare tilde", path: "~", want: "/home/testuser"},
		{name: "tilde slash", path: "~/.ssh/agent.sock", want: "/home/testuser/.ssh/agent.sock"},
		{name: "absolute path", path: "/var/run/agent.sock", want: "/var/run/agent.sock"},
		{name: "relative path", path: "relative/path", want: "relative/path"},
		{name: "tilde in middle", path: "/some/~/path", want: "/some/~/path"},
		{name: "empty", path: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandTilde(tt.path, home)
			if got != tt.want {
				t.Errorf("expandTilde(%q, %q) = %q, want %q", tt.path, home, got, tt.want)
			}
		})
	}
}

func TestResolveSSHConfig(t *testing.T) {
	// Save and restore HOME so we can point at a temp dir with our test config.
	origHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	tests := []struct {
		name   string
		config string
		host   string
		check  func(t *testing.T, got SSHHostConfig)
	}{
		{
			name: "all directives for matching host",
			config: `Host myserver
    User deploy
    Port 2222
    IdentityAgent ~/agent.sock
    IdentityFile ~/.ssh/mykey
`,
			host: "myserver",
			check: func(t *testing.T, got SSHHostConfig) {
				t.Helper()
				if got.User != "deploy" {
					t.Errorf("User = %q, want %q", got.User, "deploy")
				}
				if got.Port != 2222 {
					t.Errorf("Port = %d, want %d", got.Port, 2222)
				}
				if !filepath.IsAbs(got.IdentityAgent) {
					t.Errorf("IdentityAgent = %q, want absolute path (tilde expanded)", got.IdentityAgent)
				}
				if !filepath.IsAbs(got.IdentityFile) {
					t.Errorf("IdentityFile = %q, want absolute path (tilde expanded)", got.IdentityFile)
				}
			},
		},
		{
			name: "non-matching host returns zero values",
			config: `Host other
    User admin
    Port 3333
`,
			host: "myserver",
			check: func(t *testing.T, got SSHHostConfig) {
				t.Helper()
				if got.User != "" {
					t.Errorf("User = %q, want empty", got.User)
				}
				if got.Port != 0 {
					t.Errorf("Port = %d, want 0", got.Port)
				}
			},
		},
		{
			name: "wildcard host matches",
			config: `Host *.example.com
    User wildcard-user
    IdentityAgent /run/agent.sock
`,
			host: "web.example.com",
			check: func(t *testing.T, got SSHHostConfig) {
				t.Helper()
				if got.User != "wildcard-user" {
					t.Errorf("User = %q, want %q", got.User, "wildcard-user")
				}
				if got.IdentityAgent != "/run/agent.sock" {
					t.Errorf("IdentityAgent = %q, want %q", got.IdentityAgent, "/run/agent.sock")
				}
			},
		},
		{
			name: "star-star matches all hosts",
			config: `Host *
    User globaluser
    Port 22
`,
			host: "anything.test",
			check: func(t *testing.T, got SSHHostConfig) {
				t.Helper()
				if got.User != "globaluser" {
					t.Errorf("User = %q, want %q", got.User, "globaluser")
				}
				if got.Port != 22 {
					t.Errorf("Port = %d, want %d", got.Port, 22)
				}
			},
		},
		{
			name: "specific host overrides wildcard (first match wins)",
			config: `Host myserver
    User specific

Host *
    User default
`,
			host: "myserver",
			check: func(t *testing.T, got SSHHostConfig) {
				t.Helper()
				if got.User != "specific" {
					t.Errorf("User = %q, want %q", got.User, "specific")
				}
			},
		},
		{
			name:   "empty config returns zero values",
			config: "",
			host:   "myserver",
			check: func(t *testing.T, got SSHHostConfig) {
				t.Helper()
				if got.User != "" {
					t.Errorf("User = %q, want empty", got.User)
				}
				if got.Port != 0 {
					t.Errorf("Port = %d, want 0", got.Port)
				}
				if got.IdentityAgent != "" {
					t.Errorf("IdentityAgent = %q, want empty", got.IdentityAgent)
				}
				if got.IdentityFile != "" {
					t.Errorf("IdentityFile = %q, want empty", got.IdentityFile)
				}
			},
		},
		{
			name: "identity agent with 1password path",
			config: `Host *
    IdentityAgent "~/Library/Group Containers/2BUA8C4S2C.com.1password/t/agent.sock"
`,
			host: "anyhost",
			check: func(t *testing.T, got SSHHostConfig) {
				t.Helper()
				if got.IdentityAgent == "" {
					t.Fatal("IdentityAgent is empty, want 1Password agent path")
				}
				if filepath.Base(got.IdentityAgent) != "agent.sock" {
					t.Errorf("IdentityAgent = %q, want path ending in agent.sock", got.IdentityAgent)
				}
			},
		},
		{
			name: "absolute identity agent path not expanded",
			config: `Host myserver
    IdentityAgent /run/custom/agent.sock
`,
			host: "myserver",
			check: func(t *testing.T, got SSHHostConfig) {
				t.Helper()
				if got.IdentityAgent != "/run/custom/agent.sock" {
					t.Errorf("IdentityAgent = %q, want %q", got.IdentityAgent, "/run/custom/agent.sock")
				}
			},
		},
		{
			name: "invalid port ignored",
			config: `Host myserver
    Port notanumber
    User validuser
`,
			host: "myserver",
			check: func(t *testing.T, got SSHHostConfig) {
				t.Helper()
				if got.Port != 0 {
					t.Errorf("Port = %d, want 0 (invalid port should be ignored)", got.Port)
				}
				if got.User != "validuser" {
					t.Errorf("User = %q, want %q", got.User, "validuser")
				}
			},
		},
		{
			name: "port out of range ignored",
			config: `Host myserver
    Port 99999
`,
			host: "myserver",
			check: func(t *testing.T, got SSHHostConfig) {
				t.Helper()
				if got.Port != 0 {
					t.Errorf("Port = %d, want 0 (out of range port should be ignored)", got.Port)
				}
			},
		},
		{
			name: "only User set",
			config: `Host myserver
    User onlyuser
`,
			host: "myserver",
			check: func(t *testing.T, got SSHHostConfig) {
				t.Helper()
				if got.User != "onlyuser" {
					t.Errorf("User = %q, want %q", got.User, "onlyuser")
				}
				if got.Port != 0 {
					t.Errorf("Port = %d, want 0", got.Port)
				}
				if got.IdentityAgent != "" {
					t.Errorf("IdentityAgent = %q, want empty", got.IdentityAgent)
				}
				if got.IdentityFile != "" {
					t.Errorf("IdentityFile = %q, want empty", got.IdentityFile)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpHome := t.TempDir()
			sshDir := filepath.Join(tmpHome, ".ssh")
			if err := os.MkdirAll(sshDir, 0700); err != nil {
				t.Fatalf("failed to create .ssh dir: %v", err)
			}

			configPath := filepath.Join(sshDir, "config")
			if err := os.WriteFile(configPath, []byte(tt.config), 0600); err != nil {
				t.Fatalf("failed to write config: %v", err)
			}

			os.Setenv("HOME", tmpHome)

			got := ResolveSSHConfig(tt.host)
			tt.check(t, got)
		})
	}
}

func TestResolveSSHConfigMissingFile(t *testing.T) {
	origHome := os.Getenv("HOME")
	t.Cleanup(func() { os.Setenv("HOME", origHome) })

	tmpHome := t.TempDir()
	// Don't create .ssh/config — it should return zero values gracefully.
	os.Setenv("HOME", tmpHome)

	got := ResolveSSHConfig("anyhost")

	if got.User != "" {
		t.Errorf("User = %q, want empty", got.User)
	}
	if got.Port != 0 {
		t.Errorf("Port = %d, want 0", got.Port)
	}
	if got.IdentityAgent != "" {
		t.Errorf("IdentityAgent = %q, want empty", got.IdentityAgent)
	}
	if got.IdentityFile != "" {
		t.Errorf("IdentityFile = %q, want empty", got.IdentityFile)
	}
}
