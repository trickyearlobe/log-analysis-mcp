package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func createTestZip(t *testing.T, entries map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, content := range entries {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func createTestTarGz(t *testing.T, entries map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)
	for name, content := range entries {
		hdr := &tar.Header{
			Name:    name,
			Mode:    0644,
			Size:    int64(len(content)),
			ModTime: time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunListArchive(t *testing.T) {
	zipEntries := map[string]string{
		"logs/app.log":   strings.Repeat("log line\n", 100),
		"logs/error.log": strings.Repeat("error line\n", 50),
		"data/config.yml": "key: value\n",
	}
	zipPath := createTestZip(t, zipEntries)

	tarEntries := map[string]string{
		"var/log/syslog":    strings.Repeat("syslog line\n", 200),
		"var/log/auth.log":  strings.Repeat("auth line\n", 100),
		"var/log/debug.log": strings.Repeat("debug line\n", 50),
	}
	tarGzPath := createTestTarGz(t, tarEntries)

	tests := []struct {
		name        string
		input       ListArchiveInput
		wantErr     bool
		errContains string
		checkOutput func(t *testing.T, out ListArchiveOutput)
	}{
		{
			name:  "zip list all entries",
			input: ListArchiveInput{Path: zipPath},
			checkOutput: func(t *testing.T, out ListArchiveOutput) {
				if out.TotalEntries != 3 {
					t.Errorf("TotalEntries = %d, want 3", out.TotalEntries)
				}
				if len(out.Entries) != 3 {
					t.Errorf("len(Entries) = %d, want 3", len(out.Entries))
				}
				if out.ArchiveType != "zip" {
					t.Errorf("ArchiveType = %q, want %q", out.ArchiveType, "zip")
				}
				if out.HasMore {
					t.Error("HasMore should be false")
				}
			},
		},
		{
			name:  "zip with glob pattern",
			input: ListArchiveInput{Path: zipPath, Pattern: "*.log"},
			checkOutput: func(t *testing.T, out ListArchiveOutput) {
				if out.TotalEntries != 2 {
					t.Errorf("TotalEntries = %d, want 2", out.TotalEntries)
				}
				for _, e := range out.Entries {
					if !strings.HasSuffix(e.Name, ".log") {
						t.Errorf("entry %q does not match *.log", e.Name)
					}
				}
			},
		},
		{
			name:  "zip max_entries caps results",
			input: ListArchiveInput{Path: zipPath, MaxEntries: 1},
			checkOutput: func(t *testing.T, out ListArchiveOutput) {
				if len(out.Entries) != 1 {
					t.Errorf("len(Entries) = %d, want 1", len(out.Entries))
				}
				if out.TotalEntries != 3 {
					t.Errorf("TotalEntries = %d, want 3", out.TotalEntries)
				}
				if !out.HasMore {
					t.Error("HasMore should be true")
				}
			},
		},
		{
			name:  "tar.gz list all entries",
			input: ListArchiveInput{Path: tarGzPath},
			checkOutput: func(t *testing.T, out ListArchiveOutput) {
				if out.TotalEntries != 3 {
					t.Errorf("TotalEntries = %d, want 3", out.TotalEntries)
				}
				if out.ArchiveType != "tar.gz" {
					t.Errorf("ArchiveType = %q, want %q", out.ArchiveType, "tar.gz")
				}
				if out.HasMore {
					t.Error("HasMore should be false")
				}
			},
		},
		{
			name:  "tar.gz with glob pattern",
			input: ListArchiveInput{Path: tarGzPath, Pattern: "*.log"},
			checkOutput: func(t *testing.T, out ListArchiveOutput) {
				if out.TotalEntries != 2 {
					t.Errorf("TotalEntries = %d, want 2", out.TotalEntries)
				}
			},
		},
		{
			name:  "tar.gz entry sizes correct",
			input: ListArchiveInput{Path: tarGzPath, Pattern: "syslog"},
			checkOutput: func(t *testing.T, out ListArchiveOutput) {
				if out.TotalEntries != 1 {
					t.Fatalf("TotalEntries = %d, want 1", out.TotalEntries)
				}
				expected := int64(len(strings.Repeat("syslog line\n", 200)))
				if out.Entries[0].Size != expected {
					t.Errorf("Size = %d, want %d", out.Entries[0].Size, expected)
				}
			},
		},
		{
			name:        "not an archive",
			input:       ListArchiveInput{Path: writeTempLog(t, "plain.log", "hello\n")},
			wantErr:     true,
			errContains: "INVALID_INPUT",
		},
		{
			name:        "file not found",
			input:       ListArchiveInput{Path: "/nonexistent/archive.zip"},
			wantErr:     true,
			errContains: "FILE_NOT_FOUND",
		},
		{
			name:        "directory not archive",
			input:       ListArchiveInput{Path: t.TempDir()},
			wantErr:     true,
			errContains: "INVALID_INPUT",
		},
		{
			name:        "invalid glob pattern",
			input:       ListArchiveInput{Path: zipPath, Pattern: "[invalid"},
			wantErr:     true,
			errContains: "VALIDATION_ERROR",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := RunListArchive(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.checkOutput != nil {
				tt.checkOutput(t, out)
			}
		})
	}
}
