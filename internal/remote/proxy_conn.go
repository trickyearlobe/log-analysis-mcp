package remote

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os/exec"
	"sync"
	"time"
)

// proxyConn wraps an ssh -W subprocess as a net.Conn. The SSH binary handles
// the TCP connection (authorized by macOS Application Firewall), while our
// code reads/writes the subprocess stdio to get a raw TCP stream to the
// remote host:port.
type proxyConn struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	addr   string // real "host:port" for knownhosts lookup

	mu     sync.Mutex
	closed bool
}

// proxyDial spawns /usr/bin/ssh -W host:port to establish a TCP connection
// through the system SSH binary. This bypasses macOS Application Firewall
// which blocks net.Dial from unsigned/locally-built binaries.
//
// The returned net.Conn reads from the subprocess stdout and writes to its
// stdin. Close kills the subprocess.
func proxyDial(host string, port int, timeout time.Duration) (net.Conn, error) {
	sshBin := "/usr/bin/ssh"

	target := fmt.Sprintf("%s:%d", host, port)
	connectTimeout := fmt.Sprintf("%d", max(1, int(timeout.Seconds())))

	// -W host:port  — stdio forwarding (raw TCP proxy)
	// -o BatchMode=yes — never prompt for passwords (would hang MCP)
	// -o ConnectTimeout=N — bound the connection time
	// -o StrictHostKeyChecking=yes — respect known_hosts (ssh default)
	// The final argument is the host to connect through (same host).
	cmd := exec.Command(sshBin,
		"-W", target,
		"-o", "BatchMode=yes",
		"-o", fmt.Sprintf("ConnectTimeout=%s", connectTimeout),
		"-o", "StrictHostKeyChecking=yes",
		"-p", fmt.Sprintf("%d", port),
		host,
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("remote: proxy: stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, fmt.Errorf("remote: proxy: stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, fmt.Errorf("remote: proxy: start ssh -W %s: %w", target, err)
	}

	slog.Info("remote: proxy: started ssh -W", "target", target, "pid", cmd.Process.Pid)

	return &proxyConn{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		addr:   target,
	}, nil
}

// Read reads from the SSH subprocess stdout (data arriving from the remote host).
func (c *proxyConn) Read(b []byte) (int, error) {
	return c.stdout.Read(b)
}

// Write writes to the SSH subprocess stdin (data sent to the remote host).
func (c *proxyConn) Write(b []byte) (int, error) {
	return c.stdin.Write(b)
}

// Close kills the SSH subprocess and releases resources. Safe to call multiple times.
func (c *proxyConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}
	c.closed = true

	// Close stdin first — signals the subprocess to exit gracefully.
	c.stdin.Close()

	// Kill the process if it hasn't exited.
	if c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}

	// Reap the zombie. Ignore the error — the process was killed.
	c.cmd.Wait()

	slog.Info("remote: proxy: closed", "pid", c.cmd.Process.Pid)
	return nil
}

// LocalAddr returns a dummy address. The real local addr is inside the ssh process.
func (c *proxyConn) LocalAddr() net.Addr {
	return proxyAddr{desc: "ssh-proxy-local"}
}

// RemoteAddr returns the real host:port so that knownhosts can look up the
// host key. The x/crypto/ssh knownhosts callback calls net.SplitHostPort on
// this value, so it must be a valid "host:port" string.
func (c *proxyConn) RemoteAddr() net.Addr {
	return proxyAddr{desc: c.addr}
}

// SetDeadline is a no-op. Timeouts are handled by SSH ConnectTimeout and
// the caller's context-based cancellation which calls Close.
func (c *proxyConn) SetDeadline(_ time.Time) error { return nil }

// SetReadDeadline is a no-op.
func (c *proxyConn) SetReadDeadline(_ time.Time) error { return nil }

// SetWriteDeadline is a no-op.
func (c *proxyConn) SetWriteDeadline(_ time.Time) error { return nil }

// proxyAddr is a placeholder net.Addr for proxy connections.
type proxyAddr struct {
	desc string
}

func (a proxyAddr) Network() string { return "tcp" }
func (a proxyAddr) String() string  { return a.desc }

// Compile-time check that proxyConn implements net.Conn.
var _ net.Conn = (*proxyConn)(nil)
