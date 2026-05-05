package install

import (
	"fmt"
	"os"
	"path/filepath"
)

// Result holds the outcome of an install/uninstall for one IDE.
type Result struct {
	IDE    string
	Action Action
	Error  error
}

// Install registers the running binary as an MCP server in all detected IDEs.
func Install() []Result {
	binaryPath, err := resolveBinaryPath()
	if err != nil {
		return []Result{{IDE: "all", Error: fmt.Errorf("resolve binary path: %w", err)}}
	}

	ides := SupportedIDEs()
	results := make([]Result, len(ides))

	for i, ide := range ides {
		results[i].IDE = ide.Name
		action, err := UpsertServerWithOpts(ide.ConfigPath, ide.TopLevelKey, ServerName, binaryPath, ide.ExtraFields, ide.NeedsExistingConfig)
		results[i].Action = action
		results[i].Error = err
	}

	return results
}

// Uninstall removes the MCP server entry from all detected IDE configs.
func Uninstall() []Result {
	ides := SupportedIDEs()
	results := make([]Result, len(ides))

	for i, ide := range ides {
		results[i].IDE = ide.Name
		action, err := RemoveServer(ide.ConfigPath, ide.TopLevelKey, ServerName)
		results[i].Action = action
		results[i].Error = err
	}

	return results
}

// PrintResults writes a summary table of install/uninstall results to stderr.
func PrintResults(results []Result) {
	for _, r := range results {
		if r.Error != nil {
			fmt.Fprintf(os.Stderr, "  %-20s ERROR: %v\n", r.IDE, r.Error)
		} else {
			fmt.Fprintf(os.Stderr, "  %-20s %s\n", r.IDE, r.Action)
		}
	}
}

// resolveBinaryPath returns the absolute path of the currently running binary.
func resolveBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return resolved, nil
}
