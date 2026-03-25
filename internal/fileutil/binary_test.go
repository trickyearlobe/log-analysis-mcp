package fileutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckBinary(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		wantErr bool
		errSub  string
	}{
		{
			name:    "plain text file",
			content: []byte("this is a normal log line\nand another line\n"),
			wantErr: false,
		},
		{
			name:    "file with null byte at start",
			content: []byte{0x00, 'h', 'e', 'l', 'l', 'o'},
			wantErr: true,
			errSub:  "binary file detected",
		},
		{
			name:    "file with null byte in middle",
			content: []byte("hello\x00world\n"),
			wantErr: true,
			errSub:  "binary file detected",
		},
		{
			name:    "file with null byte at end",
			content: append([]byte("some text"), 0x00),
			wantErr: true,
			errSub:  "binary file detected",
		},
		{
			name:    "empty file",
			content: []byte{},
			wantErr: false,
		},
		{
			name:    "file with high bytes but no null",
			content: []byte{0xFF, 0xFE, 0xAB, 0xCD, '\n'},
			wantErr: false,
		},
		{
			name:    "file with all printable ASCII",
			content: []byte("2025-01-15T10:30:00Z INFO server started on :8080\n"),
			wantErr: false,
		},
		{
			name:    "file with tabs and special whitespace",
			content: []byte("key\tvalue\nfoo\tbar\n"),
			wantErr: false,
		},
		{
			name:    "file with carriage returns",
			content: []byte("line1\r\nline2\r\n"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "test.log")
			if err := os.WriteFile(path, tt.content, 0644); err != nil {
				t.Fatalf("write temp file: %v", err)
			}

			err := CheckBinary(path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("CheckBinary() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && !strings.Contains(err.Error(), tt.errSub) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.errSub)
			}
		})
	}
}

func TestCheckBinaryFileNotFound(t *testing.T) {
	err := CheckBinary("/nonexistent/path/to/file.log")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestCheckBinaryUnreadableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "noperm.log")
	if err := os.WriteFile(path, []byte("content\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := chmod0(path); err != nil {
		t.Skip("cannot remove read permissions on this OS")
	}
	t.Cleanup(func() { chmod644(path) })

	err := CheckBinary(path)
	if err == nil {
		t.Fatal("expected error for unreadable file, got nil")
	}
}

func TestCheckBinaryNullAfterCheckSize(t *testing.T) {
	// Null byte beyond the 8192-byte check window should not be detected.
	// This is expected behaviour — we only check the first 8192 bytes.
	prefix := strings.Repeat("A", binaryCheckSize)
	content := append([]byte(prefix), 0x00)

	dir := t.TempDir()
	path := filepath.Join(dir, "late_null.log")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	err := CheckBinary(path)
	if err != nil {
		t.Errorf("CheckBinary() should not detect null byte beyond check window, got: %v", err)
	}
}

func TestCheckBinaryNullAtCheckBoundary(t *testing.T) {
	// Null byte at exactly the last position within the check window.
	content := make([]byte, binaryCheckSize)
	for i := range content {
		content[i] = 'A'
	}
	content[binaryCheckSize-1] = 0x00

	dir := t.TempDir()
	path := filepath.Join(dir, "boundary_null.log")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	err := CheckBinary(path)
	if err == nil {
		t.Fatal("expected binary detection for null byte at check boundary")
	}
}

func TestCheckBinarySmallFileWithNull(t *testing.T) {
	// File smaller than the check size but containing a null byte.
	content := []byte{'h', 'e', 'l', 0x00, 'o'}
	dir := t.TempDir()
	path := filepath.Join(dir, "small_binary.log")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	err := CheckBinary(path)
	if err == nil {
		t.Fatal("expected binary detection for small file with null byte")
	}
	if !strings.Contains(err.Error(), "binary file detected") {
		t.Errorf("error %q does not contain expected message", err.Error())
	}
}

func TestCheckBinaryPathInErrorMessage(t *testing.T) {
	// Verify the error message includes the file path.
	content := []byte{0x00}
	dir := t.TempDir()
	path := filepath.Join(dir, "named_binary.log")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	err := CheckBinary(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not contain path %q", err.Error(), path)
	}
}
