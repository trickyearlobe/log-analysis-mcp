package tools

import (
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeGzipFile creates a gzip-compressed file containing content.
func writeGzipFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create gzip file %s: %v", path, err)
	}
	gw := gzip.NewWriter(f)
	if _, err := gw.Write([]byte(content)); err != nil {
		f.Close()
		t.Fatalf("write gzip data %s: %v", path, err)
	}
	if err := gw.Close(); err != nil {
		f.Close()
		t.Fatalf("close gzip writer %s: %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close gzip file %s: %v", path, err)
	}
	return path
}

// writeZipFile creates a zip archive containing a single entry with content.
func writeZipFile(t *testing.T, dir, name, entryName, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip file %s: %v", path, err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create(entryName)
	if err != nil {
		f.Close()
		t.Fatalf("create zip entry %s: %v", path, err)
	}
	if _, err := w.Write([]byte(content)); err != nil {
		f.Close()
		t.Fatalf("write zip data %s: %v", path, err)
	}
	if err := zw.Close(); err != nil {
		f.Close()
		t.Fatalf("close zip writer %s: %v", path, err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close zip file %s: %v", path, err)
	}
	return path
}

func TestRunDecompressFile(t *testing.T) {
	dir := t.TempDir()

	content := "line1\nline2\nline3\n"

	gzPath := writeGzipFile(t, dir, "test.log.gz", content)
	zipPath := writeZipFile(t, dir, "test.log.zip", "test.log", content)
	plainPath := writeTestFile(t, dir, "plain.txt", []string{"hello", "world"})

	tests := []struct {
		name        string
		input       DecompressFileInput
		wantErr     bool
		errContains string
		check       func(t *testing.T, out DecompressFileOutput)
	}{
		{
			name:  "decompress gzip",
			input: DecompressFileInput{Path: gzPath},
			check: func(t *testing.T, out DecompressFileOutput) {
				if out.TempPath == "" {
					t.Fatal("expected non-empty temp_path")
				}
				data, err := os.ReadFile(out.TempPath)
				if err != nil {
					t.Fatalf("read temp file: %v", err)
				}
				if string(data) != content {
					t.Errorf("decompressed content = %q, want %q", string(data), content)
				}

				gzInfo, err := os.Stat(gzPath)
				if err != nil {
					t.Fatalf("stat gz file: %v", err)
				}
				if out.CompressedSize != gzInfo.Size() {
					t.Errorf("compressed_size = %d, want %d", out.CompressedSize, gzInfo.Size())
				}
				if out.DecompressedSize != int64(len(content)) {
					t.Errorf("decompressed_size = %d, want %d", out.DecompressedSize, len(content))
				}
				if out.OriginalPath != gzPath {
					t.Errorf("original_path = %q, want %q", out.OriginalPath, gzPath)
				}
				if out.Note == "" {
					t.Error("expected non-empty note")
				}
			},
		},
		{
			name:  "decompress zip",
			input: DecompressFileInput{Path: zipPath},
			check: func(t *testing.T, out DecompressFileOutput) {
				if out.TempPath == "" {
					t.Fatal("expected non-empty temp_path")
				}
				data, err := os.ReadFile(out.TempPath)
				if err != nil {
					t.Fatalf("read temp file: %v", err)
				}
				if string(data) != content {
					t.Errorf("decompressed content = %q, want %q", string(data), content)
				}

				zipInfo, err := os.Stat(zipPath)
				if err != nil {
					t.Fatalf("stat zip file: %v", err)
				}
				if out.CompressedSize != zipInfo.Size() {
					t.Errorf("compressed_size = %d, want %d", out.CompressedSize, zipInfo.Size())
				}
				if out.DecompressedSize != int64(len(content)) {
					t.Errorf("decompressed_size = %d, want %d", out.DecompressedSize, len(content))
				}
				if out.OriginalPath != zipPath {
					t.Errorf("original_path = %q, want %q", out.OriginalPath, zipPath)
				}
				if out.Note == "" {
					t.Error("expected non-empty note")
				}
			},
		},
		{
			name:        "not compressed",
			input:       DecompressFileInput{Path: plainPath},
			wantErr:     true,
			errContains: "compressed extension",
		},
		{
			name:        "file not found",
			input:       DecompressFileInput{Path: filepath.Join(dir, "nonexistent.gz")},
			wantErr:     true,
			errContains: "FILE_NOT_FOUND",
		},
		{
			name:        "directory path rejected",
			input:       DecompressFileInput{Path: dir},
			wantErr:     true,
			errContains: "INVALID_INPUT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Cleanup(CleanupTempFiles)

			out, err := RunDecompressFile(tt.input)
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
			if tt.check != nil {
				tt.check(t, out)
			}
		})
	}
}

func TestCleanupTempFiles(t *testing.T) {
	dir := t.TempDir()
	content := "cleanup test data\n"
	gzPath := writeGzipFile(t, dir, "cleanup.log.gz", content)

	out, err := RunDecompressFile(DecompressFileInput{Path: gzPath})
	if err != nil {
		t.Fatalf("decompress failed: %v", err)
	}

	// Verify the temp file exists.
	if _, err := os.Stat(out.TempPath); err != nil {
		t.Fatalf("temp file should exist before cleanup: %v", err)
	}

	CleanupTempFiles()

	// Verify the temp file is gone.
	if _, err := os.Stat(out.TempPath); !os.IsNotExist(err) {
		t.Errorf("temp file should not exist after cleanup, got err: %v", err)
	}
}
