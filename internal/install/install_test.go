package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsertServerNewFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	action, err := UpsertServer(configPath, "mcpServers", "test-server", "/usr/local/bin/test-server")
	if err != nil {
		t.Fatalf("UpsertServer: %v", err)
	}
	if action != ActionInstalled {
		t.Errorf("action = %q, want %q", action, ActionInstalled)
	}

	config := readJSON(t, configPath)
	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers key missing or not a map")
	}
	entry, ok := servers["test-server"].(map[string]any)
	if !ok {
		t.Fatal("test-server entry missing or not a map")
	}
	if cmd := entry["command"]; cmd != "/usr/local/bin/test-server" {
		t.Errorf("command = %q, want %q", cmd, "/usr/local/bin/test-server")
	}
}

func TestUpsertServerExistingFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	// Write an existing config with other keys.
	initial := map[string]any{
		"theme": "dark",
		"mcpServers": map[string]any{
			"other-server": map[string]any{
				"command": "/usr/local/bin/other",
			},
		},
	}
	writeJSON(t, configPath, initial)

	action, err := UpsertServer(configPath, "mcpServers", "test-server", "/usr/local/bin/test-server")
	if err != nil {
		t.Fatalf("UpsertServer: %v", err)
	}
	if action != ActionInstalled {
		t.Errorf("action = %q, want %q", action, ActionInstalled)
	}

	config := readJSON(t, configPath)

	// Existing keys preserved.
	if config["theme"] != "dark" {
		t.Error("existing 'theme' key was lost")
	}

	servers := config["mcpServers"].(map[string]any)

	// Other server preserved.
	if _, ok := servers["other-server"]; !ok {
		t.Error("existing 'other-server' entry was lost")
	}

	// New server added.
	entry, ok := servers["test-server"].(map[string]any)
	if !ok {
		t.Fatal("test-server entry missing")
	}
	if cmd := entry["command"]; cmd != "/usr/local/bin/test-server" {
		t.Errorf("command = %q, want %q", cmd, "/usr/local/bin/test-server")
	}
}

func TestUpsertServerUpdate(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	initial := map[string]any{
		"mcpServers": map[string]any{
			"test-server": map[string]any{
				"command": "/old/path/test-server",
			},
		},
	}
	writeJSON(t, configPath, initial)

	action, err := UpsertServer(configPath, "mcpServers", "test-server", "/new/path/test-server")
	if err != nil {
		t.Fatalf("UpsertServer: %v", err)
	}
	if action != ActionUpdated {
		t.Errorf("action = %q, want %q", action, ActionUpdated)
	}

	config := readJSON(t, configPath)
	servers := config["mcpServers"].(map[string]any)
	entry := servers["test-server"].(map[string]any)
	if cmd := entry["command"]; cmd != "/new/path/test-server" {
		t.Errorf("command = %q, want %q", cmd, "/new/path/test-server")
	}
}

func TestUpsertServerAlreadyUpToDate(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	initial := map[string]any{
		"mcpServers": map[string]any{
			"test-server": map[string]any{
				"command": "/usr/local/bin/test-server",
			},
		},
	}
	writeJSON(t, configPath, initial)

	action, err := UpsertServer(configPath, "mcpServers", "test-server", "/usr/local/bin/test-server")
	if err != nil {
		t.Fatalf("UpsertServer: %v", err)
	}
	if action != ActionUpToDate {
		t.Errorf("action = %q, want %q", action, ActionUpToDate)
	}
}

func TestUpsertServerSkippedWhenDirMissing(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "nonexistent", "subdir", "config.json")

	action, err := UpsertServer(configPath, "mcpServers", "test-server", "/usr/local/bin/test-server")
	if err != nil {
		t.Fatalf("UpsertServer: %v", err)
	}
	if action != ActionSkipped {
		t.Errorf("action = %q, want %q", action, ActionSkipped)
	}
}

func TestUpsertServerDifferentTopLevelKeys(t *testing.T) {
	tests := []struct {
		name        string
		topLevelKey string
	}{
		{"mcpServers (Claude/Windsurf)", "mcpServers"},
		{"servers (VS Code/Cursor)", "servers"},
		{"context_servers (Zed)", "context_servers"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			configPath := filepath.Join(dir, "config.json")

			action, err := UpsertServer(configPath, tt.topLevelKey, "test-server", "/usr/local/bin/test")
			if err != nil {
				t.Fatalf("UpsertServer: %v", err)
			}
			if action != ActionInstalled {
				t.Errorf("action = %q, want %q", action, ActionInstalled)
			}

			config := readJSON(t, configPath)
			servers, ok := config[tt.topLevelKey].(map[string]any)
			if !ok {
				t.Fatalf("top-level key %q missing or not a map", tt.topLevelKey)
			}
			if _, ok := servers["test-server"]; !ok {
				t.Errorf("test-server missing under %q", tt.topLevelKey)
			}
		})
	}
}

func TestUpsertServerIdempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	// First install.
	action1, err := UpsertServer(configPath, "mcpServers", "test-server", "/usr/local/bin/test")
	if err != nil {
		t.Fatalf("first UpsertServer: %v", err)
	}
	if action1 != ActionInstalled {
		t.Errorf("first action = %q, want %q", action1, ActionInstalled)
	}

	// Second install — same path.
	action2, err := UpsertServer(configPath, "mcpServers", "test-server", "/usr/local/bin/test")
	if err != nil {
		t.Fatalf("second UpsertServer: %v", err)
	}
	if action2 != ActionUpToDate {
		t.Errorf("second action = %q, want %q", action2, ActionUpToDate)
	}
}

func TestUpsertServerZedPreservesOtherSettings(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	// Zed settings.json has many other keys.
	initial := map[string]any{
		"theme":     "One Dark",
		"vim_mode":  true,
		"tab_size":  4,
		"formatter": "prettier",
		"lsp": map[string]any{
			"rust-analyzer": map[string]any{
				"binary": map[string]any{
					"path": "/usr/local/bin/rust-analyzer",
				},
			},
		},
	}
	writeJSON(t, configPath, initial)

	action, err := UpsertServer(configPath, "context_servers", "test-server", "/usr/local/bin/test")
	if err != nil {
		t.Fatalf("UpsertServer: %v", err)
	}
	if action != ActionInstalled {
		t.Errorf("action = %q, want %q", action, ActionInstalled)
	}

	config := readJSON(t, configPath)

	// All original keys preserved.
	if config["theme"] != "One Dark" {
		t.Error("theme key was lost")
	}
	if config["vim_mode"] != true {
		t.Error("vim_mode key was lost")
	}
	// tab_size comes back as float64 from JSON.
	if config["tab_size"] != float64(4) {
		t.Error("tab_size key was lost")
	}
	if _, ok := config["lsp"]; !ok {
		t.Error("lsp key was lost")
	}

	// New server added.
	servers, ok := config["context_servers"].(map[string]any)
	if !ok {
		t.Fatal("context_servers key missing")
	}
	if _, ok := servers["test-server"]; !ok {
		t.Error("test-server missing under context_servers")
	}
}

func TestUpsertServerEmptyFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	// Write an empty file.
	if err := os.WriteFile(configPath, []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}

	action, err := UpsertServer(configPath, "mcpServers", "test-server", "/usr/local/bin/test")
	if err != nil {
		t.Fatalf("UpsertServer: %v", err)
	}
	if action != ActionInstalled {
		t.Errorf("action = %q, want %q", action, ActionInstalled)
	}
}

func TestUpsertServerInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	if err := os.WriteFile(configPath, []byte("{not valid json"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := UpsertServer(configPath, "mcpServers", "test-server", "/usr/local/bin/test")
	if err == nil {
		t.Fatal("expected error for invalid JSON, got nil")
	}
}

// --- RemoveServer tests ---

func TestRemoveServerPresent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	initial := map[string]any{
		"mcpServers": map[string]any{
			"test-server": map[string]any{
				"command": "/usr/local/bin/test",
			},
			"other-server": map[string]any{
				"command": "/usr/local/bin/other",
			},
		},
	}
	writeJSON(t, configPath, initial)

	action, err := RemoveServer(configPath, "mcpServers", "test-server")
	if err != nil {
		t.Fatalf("RemoveServer: %v", err)
	}
	if action != ActionRemoved {
		t.Errorf("action = %q, want %q", action, ActionRemoved)
	}

	config := readJSON(t, configPath)
	servers := config["mcpServers"].(map[string]any)

	// Target removed.
	if _, ok := servers["test-server"]; ok {
		t.Error("test-server should have been removed")
	}
	// Other preserved.
	if _, ok := servers["other-server"]; !ok {
		t.Error("other-server should have been preserved")
	}
}

func TestRemoveServerLastEntry(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	initial := map[string]any{
		"theme": "dark",
		"mcpServers": map[string]any{
			"test-server": map[string]any{
				"command": "/usr/local/bin/test",
			},
		},
	}
	writeJSON(t, configPath, initial)

	action, err := RemoveServer(configPath, "mcpServers", "test-server")
	if err != nil {
		t.Fatalf("RemoveServer: %v", err)
	}
	if action != ActionRemoved {
		t.Errorf("action = %q, want %q", action, ActionRemoved)
	}

	config := readJSON(t, configPath)

	// When the last server is removed, the top-level key should be gone.
	if _, ok := config["mcpServers"]; ok {
		t.Error("mcpServers key should have been removed when empty")
	}
	// Other keys preserved.
	if config["theme"] != "dark" {
		t.Error("theme key was lost")
	}
}

func TestRemoveServerNotPresent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	initial := map[string]any{
		"mcpServers": map[string]any{
			"other-server": map[string]any{
				"command": "/usr/local/bin/other",
			},
		},
	}
	writeJSON(t, configPath, initial)

	action, err := RemoveServer(configPath, "mcpServers", "test-server")
	if err != nil {
		t.Fatalf("RemoveServer: %v", err)
	}
	if action != ActionNotPresent {
		t.Errorf("action = %q, want %q", action, ActionNotPresent)
	}
}

func TestRemoveServerNoFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")
	// File doesn't exist, but dir does.

	action, err := RemoveServer(configPath, "mcpServers", "test-server")
	if err != nil {
		t.Fatalf("RemoveServer: %v", err)
	}
	if action != ActionNotPresent {
		t.Errorf("action = %q, want %q", action, ActionNotPresent)
	}
}

func TestRemoveServerSkippedWhenDirMissing(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "nonexistent", "subdir", "config.json")

	action, err := RemoveServer(configPath, "mcpServers", "test-server")
	if err != nil {
		t.Fatalf("RemoveServer: %v", err)
	}
	if action != ActionSkipped {
		t.Errorf("action = %q, want %q", action, ActionSkipped)
	}
}

func TestRemoveServerNoTopLevelKey(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	initial := map[string]any{
		"theme": "dark",
	}
	writeJSON(t, configPath, initial)

	action, err := RemoveServer(configPath, "mcpServers", "test-server")
	if err != nil {
		t.Fatalf("RemoveServer: %v", err)
	}
	if action != ActionNotPresent {
		t.Errorf("action = %q, want %q", action, ActionNotPresent)
	}
}

// --- Round-trip: install then uninstall ---

func TestInstallThenUninstall(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	// Install.
	action, err := UpsertServer(configPath, "mcpServers", "test-server", "/usr/local/bin/test")
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if action != ActionInstalled {
		t.Errorf("install action = %q, want %q", action, ActionInstalled)
	}

	// Verify installed.
	config := readJSON(t, configPath)
	servers := config["mcpServers"].(map[string]any)
	if _, ok := servers["test-server"]; !ok {
		t.Fatal("test-server missing after install")
	}

	// Uninstall.
	action, err = RemoveServer(configPath, "mcpServers", "test-server")
	if err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if action != ActionRemoved {
		t.Errorf("uninstall action = %q, want %q", action, ActionRemoved)
	}

	// Verify removed.
	config = readJSON(t, configPath)
	if _, ok := config["mcpServers"]; ok {
		t.Error("mcpServers key should be gone after removing the only entry")
	}

	// Uninstall again — idempotent.
	action, err = RemoveServer(configPath, "mcpServers", "test-server")
	if err != nil {
		t.Fatalf("second uninstall: %v", err)
	}
	if action != ActionNotPresent {
		t.Errorf("second uninstall action = %q, want %q", action, ActionNotPresent)
	}
}

// --- SupportedIDEs tests ---

func TestSupportedIDEsReturnsEntries(t *testing.T) {
	ides := SupportedIDEs()
	if len(ides) == 0 {
		t.Fatal("SupportedIDEs returned no entries")
	}

	names := make(map[string]bool)
	for _, ide := range ides {
		if ide.Name == "" {
			t.Error("IDE with empty Name")
		}
		if ide.ConfigPath == "" {
			t.Errorf("IDE %q has empty ConfigPath", ide.Name)
		}
		if ide.TopLevelKey == "" {
			t.Errorf("IDE %q has empty TopLevelKey", ide.Name)
		}
		names[ide.Name] = true
	}

	for _, want := range []string{"Claude Desktop", "VS Code", "Cursor", "Windsurf", "Zed", "Copilot CLI"} {
		if !names[want] {
			t.Errorf("missing IDE: %s", want)
		}
	}
}

func TestServerNameConstant(t *testing.T) {
	if ServerName != "log-analysis-mcp" {
		t.Errorf("ServerName = %q, want %q", ServerName, "log-analysis-mcp")
	}
}

// --- helpers ---

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	// Strip JSONC preamble (comments before the opening '{') so the test
	// helper works on files written back with a preserved preamble.
	raw := string(data)
	if idx := strings.Index(raw, "{"); idx > 0 {
		raw = raw[idx:]
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("parse %s: %v\nraw: %s", path, err, data)
	}
	return m
}

func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// --- JSONC (JSON with Comments) tests ---

func TestUpsertServerJSONCWithLineComments(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	// Zed-style JSONC with line comments and trailing commas.
	jsonc := `// Zed settings
//
// For information on how to configure Zed, see the Zed
// documentation: https://zed.dev/docs/configuring-zed
{
  "theme": "One Dark",
  "vim_mode": true,
  "tab_size": 4,
}
`
	if err := os.WriteFile(configPath, []byte(jsonc), 0o644); err != nil {
		t.Fatal(err)
	}

	action, err := UpsertServer(configPath, "context_servers", "test-server", "/usr/local/bin/test")
	if err != nil {
		t.Fatalf("UpsertServer on JSONC: %v", err)
	}
	if action != ActionInstalled {
		t.Errorf("action = %q, want %q", action, ActionInstalled)
	}

	config := readJSON(t, configPath)

	// Original settings preserved.
	if config["theme"] != "One Dark" {
		t.Error("theme key was lost")
	}
	if config["vim_mode"] != true {
		t.Error("vim_mode key was lost")
	}

	// New server added.
	servers, ok := config["context_servers"].(map[string]any)
	if !ok {
		t.Fatal("context_servers key missing")
	}
	if _, ok := servers["test-server"]; !ok {
		t.Error("test-server missing under context_servers")
	}
}

func TestUpsertServerJSONCWithBlockComments(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	jsonc := `/* Block comment at the top */
{
  "theme": "Solarized", /* inline block comment */
  "font_size": 14
}
`
	if err := os.WriteFile(configPath, []byte(jsonc), 0o644); err != nil {
		t.Fatal(err)
	}

	action, err := UpsertServer(configPath, "context_servers", "test-server", "/usr/local/bin/test")
	if err != nil {
		t.Fatalf("UpsertServer on JSONC with block comments: %v", err)
	}
	if action != ActionInstalled {
		t.Errorf("action = %q, want %q", action, ActionInstalled)
	}

	config := readJSON(t, configPath)
	if config["theme"] != "Solarized" {
		t.Error("theme key was lost")
	}
	if config["font_size"] != float64(14) {
		t.Error("font_size key was lost")
	}
}

func TestUpsertServerJSONCPreservesPreamble(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	preamble := "// Zed settings\n// Documentation: https://zed.dev/docs\n"
	jsonc := preamble + `{
  "theme": "One Dark"
}
`
	if err := os.WriteFile(configPath, []byte(jsonc), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := UpsertServer(configPath, "context_servers", "test-server", "/usr/local/bin/test")
	if err != nil {
		t.Fatalf("UpsertServer: %v", err)
	}

	// Read the raw file and verify preamble is still there.
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	rawStr := string(raw)
	if !strings.HasPrefix(rawStr, preamble) {
		t.Errorf("preamble was not preserved.\ngot:\n%s", rawStr[:min(len(rawStr), 200)])
	}
}

func TestRemoveServerJSONCPreservesPreamble(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	preamble := "// Zed settings\n"
	jsonc := preamble + `{
  "theme": "One Dark",
  "context_servers": {
    "test-server": {
      "command": "/usr/local/bin/test"
    }
  }
}
`
	if err := os.WriteFile(configPath, []byte(jsonc), 0o644); err != nil {
		t.Fatal(err)
	}

	action, err := RemoveServer(configPath, "context_servers", "test-server")
	if err != nil {
		t.Fatalf("RemoveServer: %v", err)
	}
	if action != ActionRemoved {
		t.Errorf("action = %q, want %q", action, ActionRemoved)
	}

	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}

	rawStr := string(raw)
	if !strings.HasPrefix(rawStr, preamble) {
		t.Errorf("preamble was not preserved after uninstall.\ngot:\n%s", rawStr[:min(len(rawStr), 200)])
	}
}

func TestUpsertServerZedEntryIncludesArgsAndEnv(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	action, err := UpsertServer(configPath, "context_servers", "test-server", "/usr/local/bin/test")
	if err != nil {
		t.Fatalf("UpsertServer: %v", err)
	}
	if action != ActionInstalled {
		t.Errorf("action = %q, want %q", action, ActionInstalled)
	}

	config := readJSON(t, configPath)
	servers := config["context_servers"].(map[string]any)
	entry := servers["test-server"].(map[string]any)

	if entry["command"] != "/usr/local/bin/test" {
		t.Errorf("command = %v, want %q", entry["command"], "/usr/local/bin/test")
	}
	if entry["args"] == nil {
		t.Error("Zed entry missing 'args' field")
	}
	if entry["env"] == nil {
		t.Error("Zed entry missing 'env' field")
	}
}

func TestUpsertServerNonZedEntryOmitsArgsAndEnv(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	action, err := UpsertServer(configPath, "mcpServers", "test-server", "/usr/local/bin/test")
	if err != nil {
		t.Fatalf("UpsertServer: %v", err)
	}
	if action != ActionInstalled {
		t.Errorf("action = %q, want %q", action, ActionInstalled)
	}

	config := readJSON(t, configPath)
	servers := config["mcpServers"].(map[string]any)
	entry := servers["test-server"].(map[string]any)

	if entry["command"] != "/usr/local/bin/test" {
		t.Errorf("command = %v, want %q", entry["command"], "/usr/local/bin/test")
	}
	if _, ok := entry["args"]; ok {
		t.Error("non-Zed entry should NOT have 'args' field")
	}
	if _, ok := entry["env"]; ok {
		t.Error("non-Zed entry should NOT have 'env' field")
	}
}

func TestUpsertServerZedUpdateAddsArgsAndEnv(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "settings.json")

	// Simulate an old install that only wrote {"command": "..."} without args/env.
	initial := map[string]any{
		"context_servers": map[string]any{
			"test-server": map[string]any{
				"command": "/usr/local/bin/test",
			},
		},
	}
	writeJSON(t, configPath, initial)

	// Re-install with the same binary path — should detect missing fields and update.
	action, err := UpsertServer(configPath, "context_servers", "test-server", "/usr/local/bin/test")
	if err != nil {
		t.Fatalf("UpsertServer: %v", err)
	}
	if action != ActionUpdated {
		t.Errorf("action = %q, want %q (should update to add missing args/env)", action, ActionUpdated)
	}

	config := readJSON(t, configPath)
	servers := config["context_servers"].(map[string]any)
	entry := servers["test-server"].(map[string]any)

	if entry["args"] == nil {
		t.Error("after update, Zed entry still missing 'args' field")
	}
	if entry["env"] == nil {
		t.Error("after update, Zed entry still missing 'env' field")
	}
}

func TestUpsertServerCopilotEntryIncludesRequiredFields(t *testing.T) {
	dir := t.TempDir()
	copilotDir := filepath.Join(dir, ".copilot")
	if err := os.MkdirAll(copilotDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(copilotDir, "mcp-config.json")

	action, err := UpsertServer(configPath, "mcpServers", "test-server", "/usr/local/bin/test")
	if err != nil {
		t.Fatalf("UpsertServer: %v", err)
	}
	if action != ActionInstalled {
		t.Errorf("action = %q, want %q", action, ActionInstalled)
	}

	config := readJSON(t, configPath)
	servers := config["mcpServers"].(map[string]any)
	entry := servers["test-server"].(map[string]any)

	if entry["command"] != "/usr/local/bin/test" {
		t.Errorf("command = %v, want %q", entry["command"], "/usr/local/bin/test")
	}
	if entry["type"] != "local" {
		t.Errorf("type = %v, want %q", entry["type"], "local")
	}
	if entry["args"] == nil {
		t.Error("Copilot entry missing 'args' field")
	}
	if entry["env"] == nil {
		t.Error("Copilot entry missing 'env' field")
	}
	if entry["tools"] == nil {
		t.Error("Copilot entry missing 'tools' field")
	}
}

func TestUpsertServerCopilotUpdateAddsRequiredFields(t *testing.T) {
	dir := t.TempDir()
	copilotDir := filepath.Join(dir, ".copilot")
	if err := os.MkdirAll(copilotDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(copilotDir, "mcp-config.json")

	// Simulate an old install that only wrote {"command": "..."} without type/tools.
	initial := map[string]any{
		"mcpServers": map[string]any{
			"test-server": map[string]any{
				"command": "/usr/local/bin/test",
			},
		},
	}
	writeJSON(t, configPath, initial)

	// Re-install with the same binary path — should detect missing fields and update.
	action, err := UpsertServer(configPath, "mcpServers", "test-server", "/usr/local/bin/test")
	if err != nil {
		t.Fatalf("UpsertServer: %v", err)
	}
	if action != ActionUpdated {
		t.Errorf("action = %q, want %q (should update to add missing type/tools)", action, ActionUpdated)
	}

	config := readJSON(t, configPath)
	servers := config["mcpServers"].(map[string]any)
	entry := servers["test-server"].(map[string]any)

	if entry["type"] != "local" {
		t.Errorf("after update, type = %v, want %q", entry["type"], "local")
	}
	if entry["tools"] == nil {
		t.Error("after update, Copilot entry still missing 'tools' field")
	}
}

func TestUpsertServerNonCopilotMcpServersOmitsCopilotFields(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	action, err := UpsertServer(configPath, "mcpServers", "test-server", "/usr/local/bin/test")
	if err != nil {
		t.Fatalf("UpsertServer: %v", err)
	}
	if action != ActionInstalled {
		t.Errorf("action = %q, want %q", action, ActionInstalled)
	}

	config := readJSON(t, configPath)
	servers := config["mcpServers"].(map[string]any)
	entry := servers["test-server"].(map[string]any)

	if _, ok := entry["type"]; ok {
		t.Error("non-Copilot mcpServers entry should NOT have 'type' field")
	}
	if _, ok := entry["tools"]; ok {
		t.Error("non-Copilot mcpServers entry should NOT have 'tools' field")
	}
}

func TestJSONCCommentInsideString(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.json")

	// The // inside the string value must NOT be treated as a comment.
	jsonc := `{
  "url": "https://example.com/path",
  "description": "this has // slashes"
}
`
	if err := os.WriteFile(configPath, []byte(jsonc), 0o644); err != nil {
		t.Fatal(err)
	}

	action, err := UpsertServer(configPath, "mcpServers", "test-server", "/usr/local/bin/test")
	if err != nil {
		t.Fatalf("UpsertServer: %v", err)
	}
	if action != ActionInstalled {
		t.Errorf("action = %q, want %q", action, ActionInstalled)
	}

	config := readJSON(t, configPath)
	if config["url"] != "https://example.com/path" {
		t.Errorf("url = %q, want %q", config["url"], "https://example.com/path")
	}
	if config["description"] != "this has // slashes" {
		t.Errorf("description = %q, want %q", config["description"], "this has // slashes")
	}
}
