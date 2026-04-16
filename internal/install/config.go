package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Action describes what happened during an install/uninstall operation.
type Action string

const (
	ActionInstalled  Action = "installed"
	ActionUpdated    Action = "updated"
	ActionUpToDate   Action = "already up to date"
	ActionSkipped    Action = "skipped (IDE not installed)"
	ActionRemoved    Action = "removed"
	ActionNotPresent Action = "not present"
)

// serverEntry is the JSON structure written for each MCP server.
type serverEntry struct {
	Command string `json:"command"`
}

// UpsertServer adds or updates the MCP server entry in an IDE config file.
// Returns the action taken.
func UpsertServer(configPath, topLevelKey, serverName, binaryPath string) (Action, error) {
	// Check if the parent directory exists (IDE installed?)
	dir := filepath.Dir(configPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return ActionSkipped, nil
	}

	cf, err := readConfig(configPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", configPath, err)
	}

	servers, _ := cf.data[topLevelKey].(map[string]any)
	if servers == nil {
		servers = make(map[string]any)
	}

	entry := map[string]any{"command": binaryPath}
	// Zed requires args and env fields to be present.
	if topLevelKey == "context_servers" {
		entry["args"] = []any{}
		entry["env"] = map[string]any{}
	}

	// Copilot CLI requires type, args, env, and tools fields.
	if strings.Contains(configPath, ".copilot") {
		entry["type"] = "local"
		entry["args"] = []any{}
		entry["env"] = map[string]any{}
		entry["tools"] = []any{"*"}
	}

	existing, hasExisting := servers[serverName]
	if hasExisting {
		existingMap, ok := existing.(map[string]any)
		if ok {
			existingCmd, _ := existingMap["command"].(string)
			if existingCmd == binaryPath && entryMatchesShape(existingMap, entry) {
				return ActionUpToDate, nil
			}
		}
		servers[serverName] = entry
		cf.data[topLevelKey] = servers
		if err := writeConfig(configPath, cf); err != nil {
			return "", fmt.Errorf("write %s: %w", configPath, err)
		}
		return ActionUpdated, nil
	}

	servers[serverName] = entry
	cf.data[topLevelKey] = servers
	if err := writeConfig(configPath, cf); err != nil {
		return "", fmt.Errorf("write %s: %w", configPath, err)
	}
	return ActionInstalled, nil
}

// RemoveServer removes the MCP server entry from an IDE config file.
// Returns the action taken.
func RemoveServer(configPath, topLevelKey, serverName string) (Action, error) {
	dir := filepath.Dir(configPath)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return ActionSkipped, nil
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return ActionNotPresent, nil
	}

	cf, err := readConfig(configPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", configPath, err)
	}

	servers, _ := cf.data[topLevelKey].(map[string]any)
	if servers == nil {
		return ActionNotPresent, nil
	}

	if _, exists := servers[serverName]; !exists {
		return ActionNotPresent, nil
	}

	delete(servers, serverName)

	// If the servers map is now empty, remove the top-level key entirely.
	if len(servers) == 0 {
		delete(cf.data, topLevelKey)
	} else {
		cf.data[topLevelKey] = servers
	}

	if err := writeConfig(configPath, cf); err != nil {
		return "", fmt.Errorf("write %s: %w", configPath, err)
	}
	return ActionRemoved, nil
}

// configFile holds both the parsed config and any leading comment block that
// appeared before the opening '{'. This lets us preserve Zed-style JSONC
// headers (// Zed settings ...) when we rewrite the file.
type configFile struct {
	data     map[string]any
	preamble string // text before the first '{', preserved verbatim on write
}

// entryMatchesShape checks that the existing config entry has all the keys
// present in the desired entry. This catches cases where a previous install
// wrote a minimal entry (e.g. missing args/env for Zed) that needs updating.
func entryMatchesShape(existing, desired map[string]any) bool {
	for key := range desired {
		if _, ok := existing[key]; !ok {
			return false
		}
	}
	return true
}

// readConfig reads a JSON or JSONC config file into a map. Returns empty map if file doesn't exist.
// JSONC (JSON with Comments) is used by Zed's settings.json — line comments (//) and
// block comments (/* */) are stripped before parsing. Trailing commas are also removed.
// The leading comment preamble is preserved for writeConfig.
func readConfig(path string) (configFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return configFile{data: make(map[string]any)}, nil
		}
		return configFile{}, err
	}

	// Handle empty files
	if len(data) == 0 {
		return configFile{data: make(map[string]any)}, nil
	}

	raw := string(data)

	// Extract preamble: everything before the first '{'.
	preamble := ""
	if idx := strings.Index(raw, "{"); idx > 0 {
		preamble = raw[:idx]
	}

	cleaned := stripJSONCComments(raw)

	var config map[string]any
	if err := json.Unmarshal([]byte(cleaned), &config); err != nil {
		return configFile{}, fmt.Errorf("parse JSON: %w", err)
	}
	return configFile{data: config, preamble: preamble}, nil
}

// stripJSONCComments removes // line comments, /* block comments */, and trailing
// commas from JSONC content. It respects strings — comments inside quoted strings
// are left alone.
func stripJSONCComments(s string) string {
	var b strings.Builder
	b.Grow(len(s))

	i := 0
	for i < len(s) {
		// Inside a string literal — copy until closing quote.
		if s[i] == '"' {
			b.WriteByte(s[i])
			i++
			for i < len(s) {
				b.WriteByte(s[i])
				if s[i] == '\\' {
					i++
					if i < len(s) {
						b.WriteByte(s[i])
						i++
					}
					continue
				}
				if s[i] == '"' {
					i++
					break
				}
				i++
			}
			continue
		}

		// Line comment — skip to end of line.
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '/' {
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		}

		// Block comment — skip to closing */.
		if i+1 < len(s) && s[i] == '/' && s[i+1] == '*' {
			i += 2
			for i+1 < len(s) {
				if s[i] == '*' && s[i+1] == '/' {
					i += 2
					break
				}
				i++
			}
			continue
		}

		b.WriteByte(s[i])
		i++
	}

	// Remove trailing commas before } or ] (with optional whitespace between).
	result := b.String()
	for {
		cleaned := removeOneTrailingComma(result)
		if cleaned == result {
			break
		}
		result = cleaned
	}
	return result
}

// removeOneTrailingComma removes one instance of a trailing comma before } or ].
func removeOneTrailingComma(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] != ',' {
			continue
		}
		// Look ahead past whitespace for } or ].
		j := i + 1
		for j < len(s) && (s[j] == ' ' || s[j] == '\t' || s[j] == '\n' || s[j] == '\r') {
			j++
		}
		if j < len(s) && (s[j] == '}' || s[j] == ']') {
			return s[:i] + s[i+1:]
		}
	}
	return s
}

// writeConfig writes a configFile back to disk, restoring any leading preamble.
func writeConfig(path string, cf configFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cf.data, "", "  ")
	if err != nil {
		return err
	}

	var out []byte
	if cf.preamble != "" {
		out = append(out, []byte(cf.preamble)...)
	}
	out = append(out, data...)
	out = append(out, '\n')

	return os.WriteFile(path, out, 0o644)
}
