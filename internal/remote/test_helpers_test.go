package remote

import "os/exec"

// catCommand returns an exec.Cmd for the platform's cat binary.
// Used by proxy_conn_test.go as a loopback subprocess for testing
// the net.Conn implementation without needing SSH or network access.
func catCommand() *exec.Cmd {
	return exec.Command("cat")
}
