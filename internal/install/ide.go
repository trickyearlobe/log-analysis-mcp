package install

import (
	"os"
	"path/filepath"
	"runtime"
)

// ServerName is the key used to register the MCP server in IDE configs.
const ServerName = "log-analysis-mcp"

// IDE describes an IDE's MCP configuration location and format.
type IDE struct {
	Name               string         // human-readable name
	ConfigPath         string         // absolute path to config file
	TopLevelKey        string         // JSON key containing server definitions
	ExtraFields        map[string]any // additional fields to include in the server entry
	NeedsExistingConfig bool          // if true, skip when config file doesn't exist (vs checking parent dir)
}

// SupportedIDEs returns all known IDEs with their config paths resolved for the current OS.
func SupportedIDEs() []IDE {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	ides := []IDE{
		{
			Name:        "Claude Desktop",
			TopLevelKey: "mcpServers",
		},
		{
			Name:        "VS Code",
			ConfigPath:  filepath.Join(home, ".vscode", "mcp.json"),
			TopLevelKey: "servers",
		},
		{
			Name:        "Cursor",
			ConfigPath:  filepath.Join(home, ".cursor", "mcp.json"),
			TopLevelKey: "servers",
		},
		{
			Name:        "Windsurf",
			ConfigPath:  filepath.Join(home, ".codeium", "windsurf", "mcp_config.json"),
			TopLevelKey: "mcpServers",
		},
		{
			Name:        "Zed",
			ConfigPath:  filepath.Join(home, ".config", "zed", "settings.json"),
			TopLevelKey: "context_servers",
			ExtraFields: map[string]any{"args": []any{}, "env": map[string]any{}},
		},
		{
			Name:        "Copilot CLI",
			ConfigPath:  filepath.Join(home, ".copilot", "mcp-config.json"),
			TopLevelKey: "mcpServers",
			ExtraFields: map[string]any{"type": "local", "args": []any{}, "env": map[string]any{}, "tools": []any{"*"}},
		},
		{
			Name:        "JetBrains",
			ConfigPath:  filepath.Join(home, ".junie", "mcp", "mcp.json"),
			TopLevelKey: "mcpServers",
		},
		{
			Name:               "Claude Code",
			ConfigPath:         filepath.Join(home, ".claude.json"),
			TopLevelKey:        "mcpServers",
			ExtraFields:        map[string]any{"args": []any{}, "env": map[string]any{}},
			NeedsExistingConfig: true,
		},
	}

	// Claude Desktop path varies by OS
	switch runtime.GOOS {
	case "darwin":
		ides[0].ConfigPath = filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(home, "AppData", "Roaming")
		}
		ides[0].ConfigPath = filepath.Join(appData, "Claude", "claude_desktop_config.json")
	default: // linux
		ides[0].ConfigPath = filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
	}

	return ides
}
