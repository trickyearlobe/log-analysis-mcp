package tools

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ListArchiveInput defines the parameters for the log_list_archive tool.
type ListArchiveInput struct {
	Path       string `json:"path"                 jsonschema:"Path to the archive file (.zip, .tar.gz, .tar.bz2, .tgz, .tar)"`
	MaxEntries int    `json:"max_entries,omitempty" jsonschema:"Maximum entries to return (max 1000)"`
	Pattern    string `json:"pattern,omitempty"     jsonschema:"Glob pattern to filter entry names (e.g. *.log)"`
}

// ArchiveEntry describes a single file or directory inside an archive.
type ArchiveEntry struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"mod_time"`
	IsDir   bool   `json:"is_dir"`
}

// ListArchiveOutput is the structured result of the log_list_archive tool.
type ListArchiveOutput struct {
	Entries      []ArchiveEntry `json:"entries"`
	TotalEntries int            `json:"total_entries"`
	ArchiveType  string         `json:"archive_type"`
	HasMore      bool           `json:"has_more"`
}

// archiveType classifies the archive format from the file path.
type archiveType int

const (
	archNone archiveType = iota
	archZip
	archTarGz
	archTarBz2
	archTar
)

// detectArchiveType returns the archive type from file extension (case-insensitive).
func detectArchiveType(path string) (archiveType, string) {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return archTarGz, "tar.gz"
	case strings.HasSuffix(lower, ".tar.bz2"):
		return archTarBz2, "tar.bz2"
	case strings.HasSuffix(lower, ".tar"):
		return archTar, "tar"
	case strings.HasSuffix(lower, ".zip"):
		return archZip, "zip"
	default:
		return archNone, ""
	}
}

// RunListArchive lists entries in an archive file.
func RunListArchive(input ListArchiveInput) (ListArchiveOutput, error) {
	input.MaxEntries = DefaultInt(input.MaxEntries, 200)
	input.MaxEntries = ClampInt(input.MaxEntries, 1, 1000)

	// Validate glob pattern if provided.
	if input.Pattern != "" {
		if _, err := filepath.Match(input.Pattern, "test"); err != nil {
			return ListArchiveOutput{}, fmt.Errorf("log_list_archive: VALIDATION_ERROR: invalid glob pattern %q: %w", input.Pattern, err)
		}
	}

	// Validate file exists.
	info, err := os.Stat(input.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return ListArchiveOutput{}, fmt.Errorf("log_list_archive: FILE_NOT_FOUND: %s", input.Path)
		}
		return ListArchiveOutput{}, fmt.Errorf("log_list_archive: %w", err)
	}
	if info.IsDir() {
		return ListArchiveOutput{}, fmt.Errorf("log_list_archive: INVALID_INPUT: path is a directory, not an archive: %s", input.Path)
	}

	archType, archName := detectArchiveType(input.Path)
	if archType == archNone {
		return ListArchiveOutput{}, fmt.Errorf("log_list_archive: INVALID_INPUT: file does not have a recognised archive extension (.zip, .tar.gz, .tgz, .tar.bz2, .tar): %s", input.Path)
	}

	var entries []ArchiveEntry
	var totalEntries int

	switch archType {
	case archZip:
		entries, totalEntries, err = listZipEntries(input.Path, input.Pattern, input.MaxEntries)
	case archTarGz, archTarBz2, archTar:
		entries, totalEntries, err = listTarEntries(input.Path, archType, input.Pattern, input.MaxEntries)
	}
	if err != nil {
		return ListArchiveOutput{}, fmt.Errorf("log_list_archive: %w", err)
	}

	if entries == nil {
		entries = []ArchiveEntry{}
	}

	return ListArchiveOutput{
		Entries:      entries,
		TotalEntries: totalEntries,
		ArchiveType:  archName,
		HasMore:      totalEntries > len(entries),
	}, nil
}

// listZipEntries reads the zip central directory and returns matching entries.
func listZipEntries(path, pattern string, maxEntries int) ([]ArchiveEntry, int, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, 0, fmt.Errorf("zip open: %w", err)
	}
	defer zr.Close()

	var entries []ArchiveEntry
	totalEntries := 0

	for _, f := range zr.File {
		if !matchesPattern(f.Name, pattern) {
			continue
		}
		totalEntries++
		if len(entries) < maxEntries {
			entries = append(entries, ArchiveEntry{
				Name:    f.Name,
				Size:    int64(f.UncompressedSize64),
				ModTime: f.Modified.UTC().Format(time.RFC3339),
				IsDir:   f.FileInfo().IsDir(),
			})
		}
	}
	return entries, totalEntries, nil
}

// listTarEntries streams a tar archive (optionally decompressed) and returns entries.
func listTarEntries(path string, aType archiveType, pattern string, maxEntries int) ([]ArchiveEntry, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open: %w", err)
	}
	defer f.Close()

	var reader io.Reader = f
	switch aType {
	case archTarGz:
		gr, gzErr := gzip.NewReader(f)
		if gzErr != nil {
			return nil, 0, fmt.Errorf("gzip: %w", gzErr)
		}
		defer gr.Close()
		reader = gr
	case archTarBz2:
		reader = bzip2.NewReader(f)
	}

	tr := tar.NewReader(reader)
	var entries []ArchiveEntry
	totalEntries := 0

	for {
		hdr, readErr := tr.Next()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return nil, 0, fmt.Errorf("tar read: %w", readErr)
		}

		if !matchesPattern(hdr.Name, pattern) {
			continue
		}
		totalEntries++
		if len(entries) < maxEntries {
			entries = append(entries, ArchiveEntry{
				Name:    hdr.Name,
				Size:    hdr.Size,
				ModTime: hdr.ModTime.UTC().Format(time.RFC3339),
				IsDir:   hdr.Typeflag == tar.TypeDir,
			})
		}
	}
	return entries, totalEntries, nil
}

// matchesPattern applies a glob filter to an entry name. Returns true if no
// pattern is set or if the base name matches.
func matchesPattern(name, pattern string) bool {
	if pattern == "" {
		return true
	}
	// Match against the base name (last path component).
	base := filepath.Base(name)
	matched, _ := filepath.Match(pattern, base)
	return matched
}
