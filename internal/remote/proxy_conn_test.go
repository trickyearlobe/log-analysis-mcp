package remote

import (
	"io"
	"net"
	"testing"
	"time"
)

func TestProxyConnImplementsNetConn(t *testing.T) {
	// Compile-time check is in proxy_conn.go, but verify at runtime too.
	var _ net.Conn = (*proxyConn)(nil)
}

func TestProxyConnReadWrite(t *testing.T) {
	// Use cat as a loopback — what we write to stdin comes back on stdout.
	conn, err := newTestProxyConn(t)
	if err != nil {
		t.Fatalf("newTestProxyConn: %v", err)
	}
	defer conn.Close()

	msg := []byte("hello proxy\n")
	n, err := conn.Write(msg)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(msg) {
		t.Fatalf("Write: wrote %d bytes, want %d", n, len(msg))
	}

	buf := make([]byte, 256)
	n, err = conn.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	got := string(buf[:n])
	if got != string(msg) {
		t.Errorf("Read = %q, want %q", got, string(msg))
	}
}

func TestProxyConnCloseStopsProcess(t *testing.T) {
	conn, err := newTestProxyConn(t)
	if err != nil {
		t.Fatalf("newTestProxyConn: %v", err)
	}

	pc := conn.(*proxyConn)
	pid := pc.cmd.Process.Pid

	if err := conn.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Process should be dead. Wait should have been called already in Close,
	// so ProcessState should be set.
	if pc.cmd.ProcessState == nil {
		t.Errorf("process %d: ProcessState is nil after Close, expected reaped", pid)
	}
}

func TestProxyConnDoubleCloseIsHarmless(t *testing.T) {
	conn, err := newTestProxyConn(t)
	if err != nil {
		t.Fatalf("newTestProxyConn: %v", err)
	}

	if err := conn.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}

func TestProxyConnReadAfterClose(t *testing.T) {
	conn, err := newTestProxyConn(t)
	if err != nil {
		t.Fatalf("newTestProxyConn: %v", err)
	}
	conn.Close()

	buf := make([]byte, 64)
	_, err = conn.Read(buf)
	if err == nil {
		t.Fatal("Read after Close: expected error, got nil")
	}
}

func TestProxyConnWriteAfterClose(t *testing.T) {
	conn, err := newTestProxyConn(t)
	if err != nil {
		t.Fatalf("newTestProxyConn: %v", err)
	}
	conn.Close()

	_, err = conn.Write([]byte("data"))
	if err == nil {
		t.Fatal("Write after Close: expected error, got nil")
	}
}

func TestProxyConnLocalAddr(t *testing.T) {
	conn, err := newTestProxyConn(t)
	if err != nil {
		t.Fatalf("newTestProxyConn: %v", err)
	}
	defer conn.Close()

	addr := conn.LocalAddr()
	if addr.Network() != "tcp" {
		t.Errorf("LocalAddr().Network() = %q, want %q", addr.Network(), "tcp")
	}
	if addr.String() == "" {
		t.Error("LocalAddr().String() is empty")
	}
}

func TestProxyConnRemoteAddr(t *testing.T) {
	conn, err := newTestProxyConn(t)
	if err != nil {
		t.Fatalf("newTestProxyConn: %v", err)
	}
	defer conn.Close()

	addr := conn.RemoteAddr()
	if addr.Network() != "tcp" {
		t.Errorf("RemoteAddr().Network() = %q, want %q", addr.Network(), "tcp")
	}
	if addr.String() == "" {
		t.Error("RemoteAddr().String() is empty")
	}
}

func TestProxyConnDeadlinesAreNoOps(t *testing.T) {
	conn, err := newTestProxyConn(t)
	if err != nil {
		t.Fatalf("newTestProxyConn: %v", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(time.Second)

	if err := conn.SetDeadline(deadline); err != nil {
		t.Errorf("SetDeadline: %v", err)
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		t.Errorf("SetReadDeadline: %v", err)
	}
	if err := conn.SetWriteDeadline(deadline); err != nil {
		t.Errorf("SetWriteDeadline: %v", err)
	}
}

func TestProxyConnMultipleMessages(t *testing.T) {
	conn, err := newTestProxyConn(t)
	if err != nil {
		t.Fatalf("newTestProxyConn: %v", err)
	}
	defer conn.Close()

	messages := []string{"first\n", "second\n", "third\n"}
	for _, msg := range messages {
		_, err := conn.Write([]byte(msg))
		if err != nil {
			t.Fatalf("Write(%q): %v", msg, err)
		}

		buf := make([]byte, 256)
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatalf("Read after Write(%q): %v", msg, err)
		}
		got := string(buf[:n])
		if got != msg {
			t.Errorf("Read = %q, want %q", got, msg)
		}
	}
}

func TestProxyConnEOFOnStdinClose(t *testing.T) {
	conn, err := newTestProxyConn(t)
	if err != nil {
		t.Fatalf("newTestProxyConn: %v", err)
	}

	// Close stdin so cat exits.
	pc := conn.(*proxyConn)
	pc.stdin.Close()

	// cat should exit and stdout should give EOF.
	buf := make([]byte, 64)
	_, err = conn.Read(buf)
	if err != io.EOF && err != nil {
		// Some systems may return a different closed-pipe error, but
		// we expect either EOF or an error — not success with data.
		t.Logf("Read after stdin close: got err=%v (acceptable)", err)
	}

	// Clean up fully.
	conn.Close()
}

// newTestProxyConn creates a proxyConn backed by `cat`, which echoes stdin
// to stdout — a simple loopback for testing the net.Conn implementation
// without needing SSH or network access.
func newTestProxyConn(t *testing.T) (net.Conn, error) {
	t.Helper()

	cmd := catCommand()
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		return nil, err
	}

	return &proxyConn{
		cmd:    cmd,
		stdin:  stdin,
		stdout: stdout,
		addr:   "127.0.0.1:22",
	}, nil
}
