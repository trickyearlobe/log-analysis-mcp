package fileutil

import (
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// CompressedTailUnavailable is returned when TailLines cannot use backward
// seeking on a compressed file and must fall back to streaming.
const compressedTailUnavailable = "compressed files do not support seekable tail; using streaming fallback"

// IsCompressed reports whether the file path has a recognised compressed extension.
func IsCompressed(path string) bool {
	switch compressionType(path) {
	case compGzip, compBzip2, compZip:
		return true
	}
	return false
}

// OpenReader opens a file for reading, transparently decompressing if the file
// extension indicates a supported compressed format (.gz, .bz2, .zip).
// The returned io.ReadCloser yields decompressed bytes. The caller must close it.
// The returned int64 is the compressed file size on disk.
func OpenReader(path string) (io.ReadCloser, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open %s: %w", path, err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, fmt.Errorf("stat %s: %w", path, err)
	}
	size := info.Size()

	switch compressionType(path) {
	case compGzip:
		gr, err := gzip.NewReader(f)
		if err != nil {
			f.Close()
			return nil, 0, fmt.Errorf("gzip open %s: %w", path, err)
		}
		return &gzipReadCloser{gz: gr, file: f}, size, nil

	case compBzip2:
		br := bzip2.NewReader(f)
		return &bzip2ReadCloser{reader: br, file: f}, size, nil

	case compZip:
		// zip.OpenReader needs the path, not an io.Reader, because zip
		// requires random access to read the central directory.
		f.Close() // close our initial handle; zip.OpenReader opens its own
		zr, err := zip.OpenReader(path)
		if err != nil {
			return nil, 0, fmt.Errorf("zip open %s: %w", path, err)
		}
		if len(zr.File) == 0 {
			zr.Close()
			return nil, 0, fmt.Errorf("zip %s: archive contains no entries", path)
		}
		entry, err := zr.File[0].Open()
		if err != nil {
			zr.Close()
			return nil, 0, fmt.Errorf("zip entry open %s: %w", path, err)
		}
		return &zipReadCloser{entry: entry, archive: zr}, size, nil

	default:
		// Plain file — return as-is.
		return f, size, nil
	}
}

// compression type enum
type compType int

const (
	compNone  compType = iota
	compGzip           // .gz
	compBzip2          // .bz2
	compZip            // .zip
)

// compressionType returns the compression type based on file extension.
// Matching is case-insensitive.
func compressionType(path string) compType {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".gz":
		return compGzip
	case ".bz2":
		return compBzip2
	case ".zip":
		return compZip
	default:
		return compNone
	}
}

// CompressionName returns a human-readable compression type name based on file extension.
// Returns "gzip", "bzip2", "zip", or "none".
func CompressionName(path string) string {
	switch compressionType(path) {
	case compGzip:
		return "gzip"
	case compBzip2:
		return "bzip2"
	case compZip:
		return "zip"
	default:
		return "none"
	}
}

// gzipReadCloser wraps a gzip.Reader and the underlying file so that
// Close releases both resources.
type gzipReadCloser struct {
	gz   *gzip.Reader
	file *os.File
}

func (r *gzipReadCloser) Read(p []byte) (int, error) {
	return r.gz.Read(p)
}

func (r *gzipReadCloser) Close() error {
	gzErr := r.gz.Close()
	fErr := r.file.Close()
	if gzErr != nil {
		return gzErr
	}
	return fErr
}

// bzip2ReadCloser wraps a bzip2 io.Reader and the underlying file.
// compress/bzip2 returns an io.Reader (not io.ReadCloser), so we must
// track the file separately.
type bzip2ReadCloser struct {
	reader io.Reader
	file   *os.File
}

func (r *bzip2ReadCloser) Read(p []byte) (int, error) {
	return r.reader.Read(p)
}

func (r *bzip2ReadCloser) Close() error {
	return r.file.Close()
}

// zipReadCloser wraps a zip entry reader and the archive so that
// Close releases both resources.
type zipReadCloser struct {
	entry   io.ReadCloser
	archive *zip.ReadCloser
}

func (r *zipReadCloser) Read(p []byte) (int, error) {
	return r.entry.Read(p)
}

func (r *zipReadCloser) Close() error {
	eErr := r.entry.Close()
	aErr := r.archive.Close()
	if eErr != nil {
		return eErr
	}
	return aErr
}
