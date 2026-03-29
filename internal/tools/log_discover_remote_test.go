package tools

import (
	"strings"
	"testing"
)

func TestRunDiscoverRemoteLogs_EmptyHosts(t *testing.T) {
	_, err := RunDiscoverRemoteLogs(DiscoverRemoteLogsInput{})
	if err == nil {
		t.Fatal("expected error for empty hosts, got nil")
	}
	if !strings.Contains(err.Error(), "hosts") {
		t.Errorf("error %q should contain %q", err.Error(), "hosts")
	}
}

func TestParseDiscoveryLine(t *testing.T) {
	tests := []struct {
		name        string
		line        string
		wantPath    string
		wantSize    int64
		wantModTime string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid line with fractional epoch",
			line:        "/var/log/app.log\t1234\t1710000000.000",
			wantPath:    "/var/log/app.log",
			wantSize:    1234,
			wantModTime: "2024-03-09T16:00:00Z",
		},
		{
			name:        "valid line with integer epoch",
			line:        "/var/log/syslog\t56789\t1700000000",
			wantPath:    "/var/log/syslog",
			wantSize:    56789,
			wantModTime: "2023-11-14T22:13:20Z",
		},
		{
			name:        "valid line with zero size",
			line:        "/var/log/empty.log\t0\t1710000000.500",
			wantPath:    "/var/log/empty.log",
			wantSize:    0,
			wantModTime: "2024-03-09T16:00:00Z",
		},
		{
			name:        "valid line with large file",
			line:        "/var/log/huge.log\t1073741824\t1710000000.000",
			wantPath:    "/var/log/huge.log",
			wantSize:    1073741824,
			wantModTime: "2024-03-09T16:00:00Z",
		},
		{
			name:        "too few fields",
			line:        "/var/log/app.log\t1234",
			wantErr:     true,
			errContains: "3 tab-separated fields",
		},
		{
			name:        "no tabs at all",
			line:        "/var/log/app.log",
			wantErr:     true,
			errContains: "3 tab-separated fields",
		},
		{
			name:        "invalid size",
			line:        "/var/log/app.log\tnotanumber\t1710000000.000",
			wantErr:     true,
			errContains: "parse size",
		},
		{
			name:        "invalid epoch returns empty modtime",
			line:        "/var/log/app.log\t100\tnotanumber",
			wantPath:    "/var/log/app.log",
			wantSize:    100,
			wantModTime: "",
		},
		{
			name:    "empty line",
			line:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, err := parseDiscoveryLine(tt.line)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if entry.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", entry.Path, tt.wantPath)
			}
			if entry.SizeBytes != tt.wantSize {
				t.Errorf("SizeBytes = %d, want %d", entry.SizeBytes, tt.wantSize)
			}
			if entry.ModifiedTime != tt.wantModTime {
				t.Errorf("ModifiedTime = %q, want %q", entry.ModifiedTime, tt.wantModTime)
			}
		})
	}
}

func TestParseEpochToRFC3339(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "integer epoch", in: "1710000000", want: "2024-03-09T16:00:00Z"},
		{name: "fractional epoch", in: "1710000000.000", want: "2024-03-09T16:00:00Z"},
		{name: "fractional with sub-second", in: "1710000000.500", want: "2024-03-09T16:00:00Z"},
		{name: "zero epoch", in: "0", want: "1970-01-01T00:00:00Z"},
		{name: "empty string", in: "", want: ""},
		{name: "whitespace only", in: "   ", want: ""},
		{name: "non-numeric", in: "abc", want: ""},
		{name: "with leading whitespace", in: " 1710000000.000 ", want: "2024-03-09T16:00:00Z"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseEpochToRFC3339(tt.in)
			if got != tt.want {
				t.Errorf("parseEpochToRFC3339(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestGroupRotatedFiles(t *testing.T) {
	tests := []struct {
		name          string
		entries       []discoveryEntry
		wantBasePaths []string
		wantVariants  map[string][]string
	}{
		{
			name: "groups rotated files under base",
			entries: []discoveryEntry{
				{Path: "/var/log/app.log", SizeBytes: 1000},
				{Path: "/var/log/app.log.1", SizeBytes: 900},
				{Path: "/var/log/app.log.2.gz", SizeBytes: 300},
				{Path: "/var/log/other.log", SizeBytes: 500},
			},
			wantBasePaths: []string{"/var/log/app.log", "/var/log/other.log"},
			wantVariants: map[string][]string{
				"/var/log/app.log": {"/var/log/app.log.1", "/var/log/app.log.2.gz"},
			},
		},
		{
			name: "natural sort order for variants",
			entries: []discoveryEntry{
				{Path: "/var/log/syslog", SizeBytes: 2000},
				{Path: "/var/log/syslog.10.gz", SizeBytes: 50},
				{Path: "/var/log/syslog.2.gz", SizeBytes: 100},
				{Path: "/var/log/syslog.1", SizeBytes: 500},
				{Path: "/var/log/syslog.3.gz", SizeBytes: 80},
			},
			wantBasePaths: []string{"/var/log/syslog"},
			wantVariants: map[string][]string{
				"/var/log/syslog": {
					"/var/log/syslog.1",
					"/var/log/syslog.2.gz",
					"/var/log/syslog.3.gz",
					"/var/log/syslog.10.gz",
				},
			},
		},
		{
			name: "no variants when base not present",
			entries: []discoveryEntry{
				{Path: "/var/log/app.log.1", SizeBytes: 100},
				{Path: "/var/log/app.log.2.gz", SizeBytes: 50},
			},
			wantBasePaths: []string{"/var/log/app.log.1", "/var/log/app.log.2.gz"},
			wantVariants:  map[string][]string{},
		},
		{
			name: "bz2 variants grouped correctly",
			entries: []discoveryEntry{
				{Path: "/var/log/messages", SizeBytes: 3000},
				{Path: "/var/log/messages.1", SizeBytes: 2000},
				{Path: "/var/log/messages.2.bz2", SizeBytes: 400},
			},
			wantBasePaths: []string{"/var/log/messages"},
			wantVariants: map[string][]string{
				"/var/log/messages": {"/var/log/messages.1", "/var/log/messages.2.bz2"},
			},
		},
		{
			name:          "empty input returns empty slice",
			entries:       []discoveryEntry{},
			wantBasePaths: []string{},
			wantVariants:  map[string][]string{},
		},
		{
			name: "single file with no variants",
			entries: []discoveryEntry{
				{Path: "/var/log/boot.log", SizeBytes: 100},
			},
			wantBasePaths: []string{"/var/log/boot.log"},
			wantVariants:  map[string][]string{},
		},
		{
			name: "multiple independent files with no rotation",
			entries: []discoveryEntry{
				{Path: "/var/log/auth.log", SizeBytes: 100},
				{Path: "/var/log/kern.log", SizeBytes: 200},
				{Path: "/var/log/daemon.log", SizeBytes: 300},
			},
			wantBasePaths: []string{"/var/log/auth.log", "/var/log/daemon.log", "/var/log/kern.log"},
			wantVariants:  map[string][]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := groupRotatedFiles(tt.entries)

			// Verify logs is never nil.
			if logs == nil {
				t.Fatal("groupRotatedFiles returned nil, want non-nil slice")
			}

			// Collect base paths.
			gotBasePaths := make([]string, len(logs))
			for i, l := range logs {
				gotBasePaths[i] = l.Path
				if l.Type != "file" {
					t.Errorf("log %q Type = %q, want %q", l.Path, l.Type, "file")
				}
			}

			// Sort for stable comparison (groupRotatedFiles preserves input order,
			// but the caller sorts later; we sort here for test determinism).
			sortStrings(gotBasePaths)
			sortStrings(tt.wantBasePaths)

			if len(gotBasePaths) != len(tt.wantBasePaths) {
				t.Fatalf("base paths = %v, want %v", gotBasePaths, tt.wantBasePaths)
			}
			for i := range gotBasePaths {
				if gotBasePaths[i] != tt.wantBasePaths[i] {
					t.Errorf("base path[%d] = %q, want %q", i, gotBasePaths[i], tt.wantBasePaths[i])
				}
			}

			// Verify variants.
			gotVariants := make(map[string][]string)
			for _, l := range logs {
				if len(l.Variants) > 0 {
					gotVariants[l.Path] = l.Variants
				}
			}

			if len(gotVariants) != len(tt.wantVariants) {
				t.Fatalf("variant groups count = %d, want %d\ngot:  %v\nwant: %v",
					len(gotVariants), len(tt.wantVariants), gotVariants, tt.wantVariants)
			}

			for base, wantVars := range tt.wantVariants {
				gotVars, ok := gotVariants[base]
				if !ok {
					t.Errorf("missing variant group for base %q", base)
					continue
				}
				if len(gotVars) != len(wantVars) {
					t.Errorf("variants for %q: got %v, want %v", base, gotVars, wantVars)
					continue
				}
				for vi := range gotVars {
					if gotVars[vi] != wantVars[vi] {
						t.Errorf("variant[%d] for %q = %q, want %q", vi, base, gotVars[vi], wantVars[vi])
					}
				}
			}
		})
	}
}

func TestDiscoverFormatSizeHuman(t *testing.T) {
	tests := []struct {
		name  string
		bytes int64
		want  string
	}{
		{name: "zero bytes", bytes: 0, want: "0 B"},
		{name: "small bytes", bytes: 512, want: "512 B"},
		{name: "one byte", bytes: 1, want: "1 B"},
		{name: "just under 1KB", bytes: 1023, want: "1023 B"},
		{name: "exactly 1KB", bytes: 1024, want: "1.0 KB"},
		{name: "kilobytes", bytes: 2560, want: "2.5 KB"},
		{name: "just under 1MB", bytes: 1048575, want: "1024.0 KB"},
		{name: "exactly 1MB", bytes: 1048576, want: "1.0 MB"},
		{name: "megabytes", bytes: 4718592, want: "4.5 MB"},
		{name: "just under 1GB", bytes: 1073741823, want: "1024.0 MB"},
		{name: "exactly 1GB", bytes: 1073741824, want: "1.0 GB"},
		{name: "gigabytes", bytes: 2684354560, want: "2.5 GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := discoverFormatSizeHuman(tt.bytes)
			if got != tt.want {
				t.Errorf("discoverFormatSizeHuman(%d) = %q, want %q", tt.bytes, got, tt.want)
			}
		})
	}
}

func TestExtractRotationIndex(t *testing.T) {
	tests := []struct {
		name string
		path string
		want int
	}{
		{name: "simple .1", path: "/var/log/app.log.1", want: 1},
		{name: "double digit", path: "/var/log/app.log.10", want: 10},
		{name: "with .gz", path: "/var/log/app.log.2.gz", want: 2},
		{name: "with .bz2", path: "/var/log/app.log.3.bz2", want: 3},
		{name: "no rotation suffix", path: "/var/log/app.log", want: 0},
		{name: "bare .gz without index", path: "/var/log/app.log.gz", want: 0},
		{name: "high index with .gz", path: "/var/log/syslog.99.gz", want: 99},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRotationIndex(tt.path)
			if got != tt.want {
				t.Errorf("extractRotationIndex(%q) = %d, want %d", tt.path, got, tt.want)
			}
		})
	}
}

func TestFindBasePath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		pathSet map[string]bool
		want    string
	}{
		{
			name:    "finds base for .1 variant",
			path:    "/var/log/app.log.1",
			pathSet: map[string]bool{"/var/log/app.log": true, "/var/log/app.log.1": true},
			want:    "/var/log/app.log",
		},
		{
			name:    "finds base for .2.gz variant",
			path:    "/var/log/app.log.2.gz",
			pathSet: map[string]bool{"/var/log/app.log": true, "/var/log/app.log.2.gz": true},
			want:    "/var/log/app.log",
		},
		{
			name:    "no base when base not in set",
			path:    "/var/log/app.log.1",
			pathSet: map[string]bool{"/var/log/app.log.1": true},
			want:    "",
		},
		{
			name:    "no match for non-rotated path",
			path:    "/var/log/app.log",
			pathSet: map[string]bool{"/var/log/app.log": true},
			want:    "",
		},
		{
			name:    "finds base through double suffix strip",
			path:    "/var/log/syslog.3.bz2",
			pathSet: map[string]bool{"/var/log/syslog": true, "/var/log/syslog.3.bz2": true},
			want:    "/var/log/syslog",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findBasePath(tt.path, tt.pathSet)
			if got != tt.want {
				t.Errorf("findBasePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestSortVariantsNaturally(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			name: "numeric order not lexicographic",
			in:   []string{"app.log.10", "app.log.2", "app.log.1", "app.log.3"},
			want: []string{"app.log.1", "app.log.2", "app.log.3", "app.log.10"},
		},
		{
			name: "mixed compressed and uncompressed",
			in:   []string{"s.3.gz", "s.1", "s.2.bz2", "s.10.gz"},
			want: []string{"s.1", "s.2.bz2", "s.3.gz", "s.10.gz"},
		},
		{
			name: "already sorted",
			in:   []string{"a.log.1", "a.log.2", "a.log.3"},
			want: []string{"a.log.1", "a.log.2", "a.log.3"},
		},
		{
			name: "single element",
			in:   []string{"a.log.1"},
			want: []string{"a.log.1"},
		},
		{
			name: "empty slice",
			in:   []string{},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Copy to avoid mutating test data.
			got := make([]string, len(tt.in))
			copy(got, tt.in)
			sortVariantsNaturally(got)

			if len(got) != len(tt.want) {
				t.Fatalf("length = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("index %d = %q, want %q\nfull result: %v", i, got[i], tt.want[i], got)
					break
				}
			}
		})
	}
}

// sortStrings sorts a string slice in place for deterministic test comparisons.
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}
