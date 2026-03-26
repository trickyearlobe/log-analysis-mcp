package integration

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Remote integration tests are guarded by the SSH_TEST_HOST environment variable.
// When not set, all tests in this file are skipped.
//
// Usage:
//   SSH_TEST_HOST=root@example.com go test -race -v ./internal/integration/ -run TestRemote
//
// The host must be in ~/.ssh/known_hosts and accessible via SSH agent or key file.

func sshTestHost(t *testing.T) string {
	t.Helper()
	host := os.Getenv("SSH_TEST_HOST")
	if host == "" {
		t.Skip("set SSH_TEST_HOST to enable remote integration tests")
	}
	return host
}

// remoteRunCommandResult mirrors the JSON output of run_remote_command for unmarshaling.
type remoteRunCommandResult struct {
	Results []struct {
		Host     string `json:"host"`
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
		ExitCode int    `json:"exit_code"`
		Error    string `json:"error,omitempty"`
	} `json:"results"`
}

// remoteDiscoverResult mirrors the JSON output of discover_remote_logs.
type remoteDiscoverResult struct {
	Results []struct {
		Host  string `json:"host"`
		Error string `json:"error,omitempty"`
		Logs  []struct {
			Path      string `json:"path"`
			SizeHuman string `json:"size_human,omitempty"`
		} `json:"logs"`
		JournalUnits []struct {
			Unit string `json:"unit"`
		} `json:"journal_units"`
	} `json:"results"`
}

// remoteGatherResult mirrors the flat GatherRemoteLogsOutput JSON structure.
type remoteGatherResult struct {
	Files []struct {
		Host       string `json:"host"`
		RemotePath string `json:"remote_path"`
		LocalPath  string `json:"local_path"`
		SizeBytes  int64  `json:"size_bytes"`
		Type       string `json:"type"`
		Error      string `json:"error,omitempty"`
	} `json:"files"`
	TempDir string `json:"temp_dir"`
}

// remoteSummarizeResult mirrors enough of summarize_logs output for our checks.
type remoteSummarizeResult struct {
	LinesAnalyzed int `json:"lines_analyzed"`
	FileInfo      struct {
		SizeBytes int64 `json:"size_bytes"`
	} `json:"file_info"`
	DetectedFormat string `json:"detected_format"`
}

func TestRemoteRunCommand(t *testing.T) {
	host := sshTestHost(t)
	session := setupTestServer(t)

	out := callToolRemote[remoteRunCommandResult](t, session, "run_remote_command", map[string]any{
		"hosts":   []string{host},
		"command": "echo hello",
	})

	if len(out.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out.Results))
	}

	r := out.Results[0]
	if r.Error != "" {
		t.Fatalf("unexpected error: %s", r.Error)
	}
	if r.ExitCode != 0 {
		t.Errorf("expected exit_code=0, got %d", r.ExitCode)
	}
	if !strings.Contains(r.Stdout, "hello") {
		t.Errorf("expected stdout to contain 'hello', got %q", r.Stdout)
	}
}

func TestRemoteRunCommandExitCode(t *testing.T) {
	host := sshTestHost(t)
	session := setupTestServer(t)

	out := callToolRemote[remoteRunCommandResult](t, session, "run_remote_command", map[string]any{
		"hosts":   []string{host},
		"command": "exit 42",
	})

	if len(out.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out.Results))
	}

	r := out.Results[0]
	if r.Error != "" {
		t.Fatalf("unexpected error: %s", r.Error)
	}
	if r.ExitCode != 42 {
		t.Errorf("expected exit_code=42, got %d", r.ExitCode)
	}
}

func TestRemoteDiscoverLogs(t *testing.T) {
	host := sshTestHost(t)
	session := setupTestServer(t)

	out := callToolRemote[remoteDiscoverResult](t, session, "discover_remote_logs", map[string]any{
		"hosts": []string{host},
	})

	if len(out.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(out.Results))
	}

	r := out.Results[0]
	if r.Error != "" {
		t.Fatalf("unexpected error for host %s: %s", r.Host, r.Error)
	}

	// Any Linux host should have at least one log file in /var/log.
	if len(r.Logs) == 0 && len(r.JournalUnits) == 0 {
		t.Error("expected at least one discovered log file or journal unit")
	}

	t.Logf("discovered %d log files and %d journal units on %s", len(r.Logs), len(r.JournalUnits), r.Host)
}

func TestRemoteGatherAndSummarize(t *testing.T) {
	host := sshTestHost(t)
	session := setupTestServer(t)

	// First, find a log file that exists and has content.
	cmdOut := callToolRemote[remoteRunCommandResult](t, session, "run_remote_command", map[string]any{
		"hosts":   []string{host},
		"command": "find /var/log -maxdepth 1 -name '*.log' -size +0c -type f 2>/dev/null | head -1",
	})

	if len(cmdOut.Results) != 1 || cmdOut.Results[0].Error != "" {
		t.Fatalf("failed to find a log file on remote host: %v", cmdOut.Results)
	}

	logPath := strings.TrimSpace(cmdOut.Results[0].Stdout)
	if logPath == "" {
		t.Skip("no non-empty .log files found in /var/log on remote host")
	}

	t.Logf("using remote log file: %s", logPath)

	// Gather it locally.
	gatherOut := callToolRemote[remoteGatherResult](t, session, "gather_remote_logs", map[string]any{
		"hosts":          []string{host},
		"paths":          []string{logPath},
		"max_file_bytes": 1048576, // 1MB cap
	})

	if len(gatherOut.Files) != 1 {
		t.Fatalf("expected 1 gathered file, got %d", len(gatherOut.Files))
	}

	gf := gatherOut.Files[0]
	if gf.Error != "" {
		t.Fatalf("file gather error: %s", gf.Error)
	}
	if gf.LocalPath == "" {
		t.Fatal("gathered file has empty local_path")
	}

	t.Logf("gathered %d bytes to %s", gf.SizeBytes, gf.LocalPath)

	// Now summarize the local copy.
	sumOut := callToolRemote[remoteSummarizeResult](t, session, "summarize_logs", map[string]any{
		"path": gf.LocalPath,
	})

	if sumOut.LinesAnalyzed == 0 {
		t.Error("summarize_logs returned 0 lines_analyzed")
	}

	t.Logf("summarized: %d lines, format=%s", sumOut.LinesAnalyzed, sumOut.DetectedFormat)
}

func TestRemoteGatherAndDiff(t *testing.T) {
	host := sshTestHost(t)
	session := setupTestServer(t)

	// Find two log files to diff.
	cmdOut := callToolRemote[remoteRunCommandResult](t, session, "run_remote_command", map[string]any{
		"hosts":   []string{host},
		"command": "find /var/log -maxdepth 1 -name '*.log' -size +0c -type f 2>/dev/null | head -2",
	})

	if len(cmdOut.Results) != 1 || cmdOut.Results[0].Error != "" {
		t.Fatalf("failed to find log files on remote host: %v", cmdOut.Results)
	}

	lines := strings.Split(strings.TrimSpace(cmdOut.Results[0].Stdout), "\n")
	if len(lines) < 2 {
		t.Skip("fewer than 2 log files found in /var/log on remote host")
	}

	path1 := strings.TrimSpace(lines[0])
	path2 := strings.TrimSpace(lines[1])
	t.Logf("diffing remote files: %s vs %s", path1, path2)

	// Gather both files.
	gatherOut := callToolRemote[remoteGatherResult](t, session, "gather_remote_logs", map[string]any{
		"hosts":          []string{host},
		"paths":          []string{path1, path2},
		"max_file_bytes": 1048576,
	})

	// Filter to files without errors.
	var goodFiles []struct {
		Host       string `json:"host"`
		RemotePath string `json:"remote_path"`
		LocalPath  string `json:"local_path"`
		SizeBytes  int64  `json:"size_bytes"`
		Type       string `json:"type"`
		Error      string `json:"error,omitempty"`
	}
	for _, f := range gatherOut.Files {
		if f.Error == "" && f.LocalPath != "" {
			goodFiles = append(goodFiles, f)
		}
	}

	if len(goodFiles) < 2 {
		t.Skipf("only %d files gathered without errors (expected 2), skipping diff", len(goodFiles))
	}

	local1 := goodFiles[0].LocalPath
	local2 := goodFiles[1].LocalPath

	// Diff the two local copies.
	diffOut := callToolRemote[diffLogsResult](t, session, "diff_logs", map[string]any{
		"base_path":   local1,
		"target_path": local2,
	})

	// We just verify the diff ran and returned structured output.
	t.Logf("diff: base=%d lines, target=%d lines, new_errors=%d, resolved=%d",
		diffOut.BaseSummary.TotalLines, diffOut.TargetSummary.TotalLines,
		len(diffOut.NewErrors), len(diffOut.ResolvedErrors))
}

// callToolRemote is the same as callTool but with a longer name to distinguish
// from the existing helper. It handles the JSON unmarshaling of tool results.
func callToolRemote[T any](t *testing.T, session *mcp.ClientSession, toolName string, args any) T {
	t.Helper()
	ctx := context.Background()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", toolName, err)
	}
	if result.IsError {
		var texts []string
		for _, c := range result.Content {
			if tc, ok := c.(*mcp.TextContent); ok {
				texts = append(texts, tc.Text)
			}
		}
		t.Fatalf("CallTool(%s) IsError=true: %s", toolName, strings.Join(texts, "; "))
	}

	if len(result.Content) == 0 {
		t.Fatalf("CallTool(%s): empty content", toolName)
	}
	tc, ok := result.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("CallTool(%s): first content is not TextContent, got %T", toolName, result.Content[0])
	}

	var out T
	if err := json.Unmarshal([]byte(tc.Text), &out); err != nil {
		t.Fatalf("CallTool(%s): unmarshal result: %v\nraw: %s", toolName, err, tc.Text)
	}
	return out
}
