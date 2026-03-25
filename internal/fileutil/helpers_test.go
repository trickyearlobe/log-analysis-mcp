package fileutil

import "os"

// chmod0 removes all permissions from a file.
func chmod0(path string) error {
	return os.Chmod(path, 0000)
}

// chmod644 restores standard read/write permissions on a file.
func chmod644(path string) {
	os.Chmod(path, 0644)
}
