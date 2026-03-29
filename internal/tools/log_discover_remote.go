package tools

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/trickyearlobe/log-analysis-mcp/internal/remote"
	"golang.org/x/crypto/ssh"
)

// maxDiscoveryOutputBytes caps the output from any single remote command.
const maxDiscoveryOutputBytes = 1048576

// DiscoverRemoteLogsInput defines the parameters for the log_discover_remote tool.
type DiscoverRemoteLogsInput struct {
	Hosts           []string `json:"hosts"                      jsonschema:"SSH targets in [user@]host[:port] format"`
	AdditionalPaths []string `json:"additional_paths,omitempty"  jsonschema:"Extra directories to scan for log files"`
	CustomCommand   string   `json:"custom_command,omitempty"    jsonschema:"Custom shell command for log discovery (output: one path per line)"`
	TimeoutSeconds  int      `json:"timeout_seconds,omitempty"   jsonschema:"Max seconds per host (default 30)"`
}

// DiscoveredLog represents a single discovered log file or journal unit.
type DiscoveredLog struct {
	Path         string   `json:"path"`
	Type         string   `json:"type"`
	SizeBytes    int64    `json:"size_bytes,omitempty"`
	SizeHuman    string   `json:"size_human,omitempty"`
	ModifiedTime string   `json:"modified_time,omitempty"`
	Variants     []string `json:"variants,omitempty"`
}

// HostDiscoveryResult holds the discovery output for a single host.
type HostDiscoveryResult struct {
	Host  string          `json:"host"`
	Logs  []DiscoveredLog `json:"logs"`
	Error string          `json:"error,omitempty"`
}

// DiscoverRemoteLogsOutput is the structured result of the log_discover_remote tool.
type DiscoverRemoteLogsOutput struct {
	Results []HostDiscoveryResult `json:"results"`
}

// discoveryEntry holds parsed file metadata before grouping.
type discoveryEntry struct {
	Path         string
	SizeBytes    int64
	SizeHuman    string
	ModifiedTime string
}

// rotationSuffixRe matches common log rotation suffixes like .1, .2.gz, .3.bz2.
var rotationSuffixRe = regexp.MustCompile(`\.\d+(?:\.gz|\.bz2)?$`)

// rotationIndexRe extracts the numeric index from a rotation suffix.
var rotationIndexRe = regexp.MustCompile(`\.(\d+)(?:\.gz|\.bz2)?$`)

// RunDiscoverRemoteLogs discovers log files and journal units on remote hosts via SSH.
func RunDiscoverRemoteLogs(input DiscoverRemoteLogsInput) (DiscoverRemoteLogsOutput, error) {
	if len(input.Hosts) == 0 {
		return DiscoverRemoteLogsOutput{}, fmt.Errorf("log_discover_remote: hosts is required and must contain at least one SSH target")
	}

	input.TimeoutSeconds = DefaultInt(input.TimeoutSeconds, 30)
	timeout := time.Duration(input.TimeoutSeconds) * time.Second
	pool := remote.DefaultPool()

	results := make([]HostDiscoveryResult, len(input.Hosts))
	for i, host := range input.Hosts {
		results[i].Host = host
		results[i].Logs = []DiscoveredLog{}

		target, err := remote.ParseTarget(host)
		if err != nil {
			results[i].Error = fmt.Sprintf("invalid host format: %v", err)
			continue
		}

		client, err := pool.Get(target)
		if err != nil {
			results[i].Error = fmt.Sprintf("connect: %v", err)
			continue
		}

		// Track all discovered paths for dedup.
		seen := make(map[string]bool)

		// Step 1: File discovery via find.
		fileEntries, err := discoverFiles(client, input.AdditionalPaths, timeout)
		if err != nil {
			results[i].Error = fmt.Sprintf("file discovery: %v", err)
			continue
		}
		for _, e := range fileEntries {
			seen[e.Path] = true
		}

		// Step 2: Journal discovery.
		journalLogs := discoverJournals(client, timeout)

		// Step 3: Custom command.
		if input.CustomCommand != "" {
			customEntries, customErr := discoverCustom(client, input.CustomCommand, timeout)
			if customErr != nil {
				results[i].Error = fmt.Sprintf("custom_command failed: %v", customErr)
				// Continue with what we have — don't abort other steps.
			} else {
				for _, e := range customEntries {
					if !seen[e.Path] {
						seen[e.Path] = true
						fileEntries = append(fileEntries, e)
					}
				}
			}
		}

		// Step 4: Group rotated files.
		fileLogs := groupRotatedFiles(fileEntries)

		// Combine file and journal logs.
		allLogs := append(fileLogs, journalLogs...)

		// Step 5: Sort by path.
		sort.Slice(allLogs, func(a, b int) bool {
			return allLogs[a].Path < allLogs[b].Path
		})

		if len(allLogs) == 0 {
			results[i].Logs = []DiscoveredLog{}
		} else {
			results[i].Logs = allLogs
		}
	}

	return DiscoverRemoteLogsOutput{Results: results}, nil
}

// discoverFiles runs find on the remote host and parses the output.
func discoverFiles(client *ssh.Client, additionalPaths []string, timeout time.Duration) ([]discoveryEntry, error) {
	pathArgs := "/var/log"
	for _, p := range additionalPaths {
		pathArgs += " " + remote.ShellEscape(p)
	}

	cmd := fmt.Sprintf(
		"find %s -maxdepth 3 \\( -name '*.log' -o -name '*.log.*' -o -name 'syslog*' -o -name 'messages*' -o -name '*.gz' -o -name '*.bz2' \\) -printf '%%p\\t%%s\\t%%T@\\n' 2>/dev/null",
		pathArgs,
	)

	result, err := remote.Exec(client, cmd, timeout, maxDiscoveryOutputBytes)
	if err != nil {
		return nil, fmt.Errorf("find command: %w", err)
	}

	var entries []discoveryEntry
	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		entry, parseErr := parseDiscoveryLine(line)
		if parseErr != nil {
			continue // skip unparseable lines
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// discoverJournals checks for journald availability and lists running service units.
func discoverJournals(client *ssh.Client, timeout time.Duration) []DiscoveredLog {
	// Check if journalctl is available via --list-boots.
	bootResult, err := remote.Exec(client, "journalctl --list-boots --no-pager 2>/dev/null", timeout, maxDiscoveryOutputBytes)
	if err != nil || bootResult.ExitCode != 0 {
		return nil
	}

	// List running service units.
	unitResult, err := remote.Exec(client,
		"systemctl list-units --type=service --state=running --no-pager --no-legend 2>/dev/null | awk '{print $1}'",
		timeout, maxDiscoveryOutputBytes)
	if err != nil || unitResult.ExitCode != 0 {
		return nil
	}

	var logs []DiscoveredLog
	lines := strings.Split(strings.TrimSpace(unitResult.Stdout), "\n")
	for _, line := range lines {
		unit := strings.TrimSpace(line)
		if unit == "" {
			continue
		}
		logs = append(logs, DiscoveredLog{
			Path: unit,
			Type: "journal",
		})
	}

	return logs
}

// discoverCustom runs a user-supplied command and parses stdout as one path per line.
func discoverCustom(client *ssh.Client, cmd string, timeout time.Duration) ([]discoveryEntry, error) {
	result, err := remote.Exec(client, cmd, timeout, maxDiscoveryOutputBytes)
	if err != nil {
		return nil, fmt.Errorf("exec: %w", err)
	}
	if result.ExitCode != 0 {
		stderr := result.Stderr
		if len(stderr) > 200 {
			stderr = stderr[:200]
		}
		return nil, fmt.Errorf("custom_command failed (exit %d): %s", result.ExitCode, stderr)
	}

	var entries []discoveryEntry
	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")
	for _, line := range lines {
		p := strings.TrimSpace(line)
		if p == "" {
			continue
		}
		entries = append(entries, discoveryEntry{Path: p})
	}

	return entries, nil
}

// parseDiscoveryLine parses a tab-separated line from find -printf '%p\t%s\t%T@\n'.
func parseDiscoveryLine(line string) (discoveryEntry, error) {
	parts := strings.SplitN(line, "\t", 3)
	if len(parts) != 3 {
		return discoveryEntry{}, fmt.Errorf("expected 3 tab-separated fields, got %d", len(parts))
	}

	path := parts[0]

	sizeBytes, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return discoveryEntry{}, fmt.Errorf("parse size %q: %w", parts[1], err)
	}

	modTime := parseEpochToRFC3339(parts[2])

	return discoveryEntry{
		Path:         path,
		SizeBytes:    sizeBytes,
		SizeHuman:    discoverFormatSizeHuman(sizeBytes),
		ModifiedTime: modTime,
	}, nil
}

// parseEpochToRFC3339 converts a Unix epoch float string to RFC 3339 format.
func parseEpochToRFC3339(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	dotIdx := strings.Index(s, ".")
	var sec int64
	var nsec int64

	if dotIdx >= 0 {
		var err error
		sec, err = strconv.ParseInt(s[:dotIdx], 10, 64)
		if err != nil {
			return ""
		}
		frac := s[dotIdx+1:]
		// Pad or truncate to 9 digits for nanoseconds.
		for len(frac) < 9 {
			frac += "0"
		}
		frac = frac[:9]
		nsec, _ = strconv.ParseInt(frac, 10, 64)
	} else {
		var err error
		sec, err = strconv.ParseInt(s, 10, 64)
		if err != nil {
			return ""
		}
	}

	t := time.Unix(sec, nsec).UTC()
	return t.Format(time.RFC3339)
}

// discoverFormatSizeHuman formats a byte count into a human-readable string.
func discoverFormatSizeHuman(b int64) string {
	const (
		kb = 1024
		mb = 1024 * 1024
		gb = 1024 * 1024 * 1024
	)
	switch {
	case b < kb:
		return fmt.Sprintf("%d B", b)
	case b < mb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	case b < gb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	default:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	}
}

// groupRotatedFiles groups rotated log files as variants of their base file.
// For example, app.log.1, app.log.2.gz become variants of app.log.
// Variants are removed from the top-level list and sorted naturally.
func groupRotatedFiles(entries []discoveryEntry) []DiscoveredLog {
	if len(entries) == 0 {
		return []DiscoveredLog{}
	}

	// Build a set of known paths for base-path lookup.
	pathSet := make(map[string]bool, len(entries))
	for _, e := range entries {
		pathSet[e.Path] = true
	}

	// Identify which paths are variants of another path.
	variantOf := make(map[string]string) // variant path → base path
	for _, e := range entries {
		base := findBasePath(e.Path, pathSet)
		if base != "" {
			variantOf[e.Path] = base
		}
	}

	// Group variants under their base.
	variantMap := make(map[string][]string)
	for variant, base := range variantOf {
		variantMap[base] = append(variantMap[base], variant)
	}

	// Sort variants naturally within each group.
	for base := range variantMap {
		sortVariantsNaturally(variantMap[base])
	}

	// Build output, skipping paths that are variants.
	var logs []DiscoveredLog
	for _, e := range entries {
		if _, isVariant := variantOf[e.Path]; isVariant {
			continue
		}

		log := DiscoveredLog{
			Path:         e.Path,
			Type:         "file",
			SizeBytes:    e.SizeBytes,
			SizeHuman:    e.SizeHuman,
			ModifiedTime: e.ModifiedTime,
		}

		if variants, ok := variantMap[e.Path]; ok {
			log.Variants = variants
		}

		logs = append(logs, log)
	}

	if logs == nil {
		return []DiscoveredLog{}
	}
	return logs
}

// findBasePath strips rotation suffixes from a path and checks if the base exists.
// Returns the base path if found, empty string otherwise.
func findBasePath(path string, pathSet map[string]bool) string {
	candidate := path
	for {
		loc := rotationSuffixRe.FindStringIndex(candidate)
		if loc == nil {
			break
		}
		candidate = candidate[:loc[0]]
		if pathSet[candidate] && candidate != path {
			return candidate
		}
	}
	return ""
}

// sortVariantsNaturally sorts variant paths by extracting the numeric rotation index.
// Produces 1, 2, 3, 10 instead of lexicographic 1, 10, 2, 3.
func sortVariantsNaturally(variants []string) {
	sort.Slice(variants, func(i, j int) bool {
		ni := extractRotationIndex(variants[i])
		nj := extractRotationIndex(variants[j])
		if ni != nj {
			return ni < nj
		}
		return variants[i] < variants[j]
	})
}

// extractRotationIndex extracts the numeric rotation index from a rotated log path.
// e.g., "/var/log/app.log.2.gz" → 2. Returns 0 if no index found.
func extractRotationIndex(path string) int {
	m := rotationIndexRe.FindStringSubmatch(path)
	if len(m) < 2 {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}
