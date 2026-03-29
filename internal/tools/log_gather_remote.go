package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/trickyearlobe/log-analysis-mcp/internal/remote"
)

// GatherRemoteLogsInput defines the parameters for the log_gather_remote tool.
type GatherRemoteLogsInput struct {
	Hosts          []string `json:"hosts"                      jsonschema:"SSH targets in [user@]host[:port] format"`
	Paths          []string `json:"paths,omitempty"             jsonschema:"Remote file paths to gather"`
	JournalUnits   []string `json:"journal_units,omitempty"     jsonschema:"Systemd journal units to export"`
	JournalSince   string   `json:"journal_since,omitempty"     jsonschema:"ISO 8601 start time for journal export"`
	JournalUntil   string   `json:"journal_until,omitempty"     jsonschema:"ISO 8601 end time for journal export"`
	MaxFileBytes   int      `json:"max_file_bytes,omitempty"    jsonschema:"Max bytes per file (default 100MB)"`
	TimeoutSeconds int      `json:"timeout_seconds,omitempty"   jsonschema:"Max seconds per file transfer (default 300)"`
}

// GatheredFile describes a single file retrieved from a remote host.
type GatheredFile struct {
	Host       string `json:"host"`
	RemotePath string `json:"remote_path"`
	LocalPath  string `json:"local_path"`
	SizeBytes  int64  `json:"size_bytes"`
	Type       string `json:"type"` // "file" or "journal"
	Error      string `json:"error,omitempty"`
}

// GatherRemoteLogsOutput is the structured result of the log_gather_remote tool.
type GatherRemoteLogsOutput struct {
	Files   []GatheredFile `json:"files"`
	TempDir string         `json:"temp_dir"`
}

// flattenPath replaces path separators with dashes and strips leading dashes.
func flattenPath(p string) string {
	// Normalize by cleaning the path first to collapse repeated slashes
	cleaned := filepath.Clean(p)
	flat := strings.ReplaceAll(cleaned, "/", "-")
	flat = strings.TrimLeft(flat, "-")
	return flat
}

// RunGatherRemoteLogs gathers log files and journal exports from remote hosts over SSH.
func RunGatherRemoteLogs(input GatherRemoteLogsInput) (GatherRemoteLogsOutput, error) {
	if len(input.Hosts) == 0 {
		return GatherRemoteLogsOutput{}, fmt.Errorf("log_gather_remote: hosts must not be empty")
	}
	if len(input.Paths) == 0 && len(input.JournalUnits) == 0 {
		return GatherRemoteLogsOutput{}, fmt.Errorf("log_gather_remote: at least one of paths or journal_units must be provided")
	}

	maxBytes := int64(DefaultInt(input.MaxFileBytes, 104857600))
	timeout := time.Duration(DefaultInt(input.TimeoutSeconds, 300)) * time.Second

	tmpDir, err := os.MkdirTemp("", "log-analysis-mcp-gather-")
	if err != nil {
		return GatherRemoteLogsOutput{}, fmt.Errorf("log_gather_remote: create temp dir: %w", err)
	}
	registerTempFile(tmpDir)

	pool := remote.DefaultPool()
	var files []GatheredFile

	for _, host := range input.Hosts {
		target, err := remote.ParseTarget(host)
		if err != nil {
			// Record one error entry per host when parsing fails
			entry := GatheredFile{
				Host:  host,
				Type:  "file",
				Error: fmt.Sprintf("parse target: %v", err),
			}
			files = append(files, entry)
			continue
		}

		client, err := pool.Get(target)
		if err != nil {
			entry := GatheredFile{
				Host:  host,
				Type:  "file",
				Error: fmt.Sprintf("connect: %v", err),
			}
			files = append(files, entry)
			continue
		}

		hostDir := filepath.Join(tmpDir, target.Host)
		if err := os.MkdirAll(hostDir, 0o700); err != nil {
			entry := GatheredFile{
				Host:  host,
				Type:  "file",
				Error: fmt.Sprintf("create host dir: %v", err),
			}
			files = append(files, entry)
			continue
		}

		// Download each requested file path
		for _, remotePath := range input.Paths {
			localName := flattenPath(remotePath)
			localPath := filepath.Join(hostDir, localName)

			entry := GatheredFile{
				Host:       host,
				RemotePath: remotePath,
				LocalPath:  localPath,
				Type:       "file",
			}

			n, err := remote.DownloadFile(client, remotePath, localPath, maxBytes, timeout)
			if err != nil {
				entry.Error = fmt.Sprintf("download: %v", err)
			} else {
				entry.SizeBytes = n
			}
			files = append(files, entry)
		}

		// Export each requested journal unit
		for _, unit := range input.JournalUnits {
			localName := "journal-" + unit + ".log"
			localPath := filepath.Join(hostDir, localName)

			entry := GatheredFile{
				Host:       host,
				RemotePath: "journalctl -u " + unit,
				LocalPath:  localPath,
				Type:       "journal",
			}

			n, err := remote.ExportJournal(client, unit, input.JournalSince, input.JournalUntil, localPath, maxBytes, timeout)
			if err != nil {
				entry.Error = fmt.Sprintf("journal export: %v", err)
			} else {
				entry.SizeBytes = n
			}
			files = append(files, entry)
		}
	}

	// Guarantee non-nil slice in output
	if files == nil {
		files = []GatheredFile{}
	}

	return GatherRemoteLogsOutput{
		Files:   files,
		TempDir: tmpDir,
	}, nil
}
