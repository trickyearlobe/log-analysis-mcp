package fileutil

import (
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// createGzipFile creates a gzip-compressed file in dir with the given name and content.
// Returns the full path to the created file.
func createGzipFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create gzip file: %v", err)
	}
	gw := gzip.NewWriter(f)
	if _, err := gw.Write([]byte(content)); err != nil {
		f.Close()
		t.Fatalf("write gzip data: %v", err)
	}
	if err := gw.Close(); err != nil {
		f.Close()
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close gzip file: %v", err)
	}
	return path
}

// createZipFile creates a zip archive in dir with the given name and entries (filename → content).
// Returns the full path to the created file.
func createZipFile(t *testing.T, dir, name string, entries map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip file: %v", err)
	}
	zw := zip.NewWriter(f)
	for eName, eContent := range entries {
		w, err := zw.Create(eName)
		if err != nil {
			zw.Close()
			f.Close()
			t.Fatalf("create zip entry %q: %v", eName, err)
		}
		if _, err := w.Write([]byte(eContent)); err != nil {
			zw.Close()
			f.Close()
			t.Fatalf("write zip entry %q: %v", eName, err)
		}
	}
	if err := zw.Close(); err != nil {
		f.Close()
		t.Fatalf("close zip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close zip file: %v", err)
	}
	return path
}

func TestIsCompressed(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: "gz extension", path: "access.log.gz", want: true},
		{name: "bz2 extension", path: "access.log.bz2", want: true},
		{name: "zip extension", path: "archive.zip", want: true},
		{name: "log extension", path: "app.log", want: false},
		{name: "txt extension", path: "notes.txt", want: false},
		{name: "uppercase GZ", path: "access.log.GZ", want: true},
		{name: "mixed case Bz2", path: "access.log.Bz2", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCompressed(tt.path)
			if got != tt.want {
				t.Errorf("IsCompressed(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestOpenReader_PlainText(t *testing.T) {
	dir := t.TempDir()
	content := "hello\nworld\n"
	path := filepath.Join(dir, "plain.log")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write plain file: %v", err)
	}

	rc, size, err := OpenReader(path)
	if err != nil {
		t.Fatalf("OpenReader(%q): %v", path, err)
	}
	defer rc.Close()

	info, _ := os.Stat(path)
	if size != info.Size() {
		t.Errorf("size = %d, want %d", size, info.Size())
	}

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != content {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestOpenReader_Gzip(t *testing.T) {
	dir := t.TempDir()
	content := "line1\nline2\nline3\n"
	path := createGzipFile(t, dir, "test.gz", content)

	rc, size, err := OpenReader(path)
	if err != nil {
		t.Fatalf("OpenReader(%q): %v", path, err)
	}
	defer rc.Close()

	// size should be the compressed size on disk
	info, _ := os.Stat(path)
	if size != info.Size() {
		t.Errorf("size = %d, want compressed size %d", size, info.Size())
	}

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != content {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestOpenReader_Bzip2(t *testing.T) {
	// stdlib has no bzip2 writer; use the bzip2 command if available.
	if _, err := exec.LookPath("bzip2"); err != nil {
		t.Skip("bzip2 command not available")
	}

	dir := t.TempDir()
	content := "line1\nline2\nline3\n"

	plainPath := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(plainPath, []byte(content), 0644); err != nil {
		t.Fatalf("write plain file: %v", err)
	}

	// bzip2 compresses in-place, producing test.txt.bz2
	cmd := exec.Command("bzip2", plainPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("bzip2 command failed: %v: %s", err, out)
	}

	bz2Path := plainPath + ".bz2"
	rc, size, err := OpenReader(bz2Path)
	if err != nil {
		t.Fatalf("OpenReader(%q): %v", bz2Path, err)
	}
	defer rc.Close()

	info, _ := os.Stat(bz2Path)
	if size != info.Size() {
		t.Errorf("size = %d, want compressed size %d", size, info.Size())
	}

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != content {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestOpenReader_ZipSingleEntry(t *testing.T) {
	dir := t.TempDir()
	content := "single entry content\n"
	entries := map[string]string{"log.txt": content}
	path := createZipFile(t, dir, "single.zip", entries)

	rc, _, err := OpenReader(path)
	if err != nil {
		t.Fatalf("OpenReader(%q): %v", path, err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != content {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestOpenReader_ZipMultipleEntries(t *testing.T) {
	dir := t.TempDir()
	// Use ordered creation: zip iterates entries in insertion order.
	// createZipFile uses a map so order is not guaranteed; build manually.
	path := filepath.Join(dir, "multi.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	zw := zip.NewWriter(f)

	firstContent := "first entry\n"
	w1, err := zw.Create("first.log")
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	w1.Write([]byte(firstContent))

	w2, err := zw.Create("second.log")
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	w2.Write([]byte("second entry\n"))

	zw.Close()
	f.Close()

	rc, _, err := OpenReader(path)
	if err != nil {
		t.Fatalf("OpenReader(%q): %v", path, err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	// OpenReader returns the first entry
	if string(got) != firstContent {
		t.Errorf("content = %q, want first entry %q", got, firstContent)
	}
}

func TestOpenReader_ZipZeroEntries(t *testing.T) {
	dir := t.TempDir()
	entries := map[string]string{} // empty
	path := createZipFile(t, dir, "empty.zip", entries)

	_, _, err := OpenReader(path)
	if err == nil {
		t.Fatal("OpenReader on empty zip: expected error, got nil")
	}
}

func TestOpenReader_CorruptGzip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.gz")
	// Write bytes that start with a valid gzip header but have corrupt payload.
	// Gzip magic: 0x1f 0x8b, method deflate: 0x08, flags: 0x00, then garbage.
	corruptData := []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff,
		0xde, 0xad, 0xbe, 0xef, 0xca, 0xfe}
	if err := os.WriteFile(path, corruptData, 0644); err != nil {
		t.Fatalf("write corrupt gz: %v", err)
	}

	rc, _, err := OpenReader(path)
	if err != nil {
		// gzip.NewReader may fail on the header — that's acceptable
		return
	}
	defer rc.Close()

	// If NewReader succeeded, reading should produce an error
	_, readErr := io.ReadAll(rc)
	if readErr == nil {
		t.Error("expected error reading corrupt gzip data, got nil")
	}
}

func TestOpenReader_NonExistentFile(t *testing.T) {
	_, _, err := OpenReader("/no/such/path/ever.log")
	if err == nil {
		t.Fatal("OpenReader on non-existent file: expected error, got nil")
	}
}

func TestOpenReader_CaseInsensitiveGZ(t *testing.T) {
	dir := t.TempDir()
	content := "uppercase gz\n"
	// Create a real gzip file first, then rename to .GZ
	tmpPath := createGzipFile(t, dir, "temp.gz", content)
	gzPath := filepath.Join(dir, "test.GZ")
	if err := os.Rename(tmpPath, gzPath); err != nil {
		t.Fatalf("rename to .GZ: %v", err)
	}

	rc, _, err := OpenReader(gzPath)
	if err != nil {
		t.Fatalf("OpenReader(%q): %v", gzPath, err)
	}
	defer rc.Close()

	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != content {
		t.Errorf("content = %q, want %q", got, content)
	}
}

func TestOpenReader_CloseReleasesResources(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, dir string) string
	}{
		{
			name: "plain file",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				p := filepath.Join(dir, "plain.log")
				os.WriteFile(p, []byte("data\n"), 0644)
				return p
			},
		},
		{
			name: "gzip file",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				return createGzipFile(t, dir, "close.gz", "data\n")
			},
		},
		{
			name: "zip file",
			setup: func(t *testing.T, dir string) string {
				t.Helper()
				return createZipFile(t, dir, "close.zip", map[string]string{"a.txt": "data\n"})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := tt.setup(t, dir)

			rc, _, err := OpenReader(path)
			if err != nil {
				t.Fatalf("OpenReader: %v", err)
			}
			if err := rc.Close(); err != nil {
				t.Errorf("Close returned error: %v", err)
			}
		})
	}
}
