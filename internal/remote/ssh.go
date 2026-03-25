package remote

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// BuildAuthMethods returns SSH auth methods in priority order (agent, then key files).
func BuildAuthMethods() ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	// Try SSH agent
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock != "" {
		conn, err := net.Dial("unix", sock)
		if err != nil {
			slog.Warn("remote: auth: SSH_AUTH_SOCK set but agent dial failed", "error", err)
		} else {
			agentClient := agent.NewClient(conn)
			methods = append(methods, ssh.PublicKeysCallback(agentClient.Signers))
			slog.Info("remote: auth: ssh-agent available")
		}
	} else {
		slog.Info("remote: auth: SSH_AUTH_SOCK not set, skipping agent")
	}

	// Try key files
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("remote: auth: cannot determine home directory: %w", err)
	}

	keyFiles := []string{
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
		filepath.Join(home, ".ssh", "id_rsa"),
	}

	for _, keyPath := range keyFiles {
		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			slog.Warn("remote: auth: failed to read key file", "path", keyPath, "error", err)
			continue
		}

		signer, err := ssh.ParsePrivateKey(keyBytes)
		if err != nil {
			var ppErr *ssh.PassphraseMissingError
			if errors.As(err, &ppErr) {
				slog.Warn("remote: auth: key is encrypted, skipping", "path", keyPath)
				continue
			}
			slog.Warn("remote: auth: failed to parse key file", "path", keyPath, "error", err)
			continue
		}

		methods = append(methods, ssh.PublicKeys(signer))
		slog.Info("remote: auth: loaded key file", "path", keyPath)
	}

	if len(methods) == 0 {
		return nil, fmt.Errorf("remote: auth: no methods available — SSH_AUTH_SOCK not set, no key files found in %s",
			filepath.Join(home, ".ssh")+"/")
	}

	return methods, nil
}

// BuildHostKeyCallback returns an ssh.HostKeyCallback backed by ~/.ssh/known_hosts.
func BuildHostKeyCallback() (ssh.HostKeyCallback, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("remote: host key: cannot determine home directory: %w", err)
	}

	path := filepath.Join(home, ".ssh", "known_hosts")

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("remote: no known_hosts file found at %s — run 'ssh-keyscan <host> >> %s' to add host keys", path, path)
	}

	cb, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("remote: host key: failed to parse %s: %w", path, err)
	}

	return cb, nil
}

// ClientPool manages reusable SSH client connections.
type ClientPool struct {
	mu          sync.Mutex
	clients     map[string]*ssh.Client
	dialTimeout time.Duration
}

// NewClientPool creates a pool with the given dial timeout.
func NewClientPool(dialTimeout time.Duration) *ClientPool {
	return &ClientPool{
		clients:     make(map[string]*ssh.Client),
		dialTimeout: dialTimeout,
	}
}

// Get returns an existing client or dials a new connection for the given target.
func (p *ClientPool) Get(target Target) (*ssh.Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	key := target.String()

	if client, ok := p.clients[key]; ok {
		// Keepalive check to see if connection is still alive
		_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
		if err == nil {
			return client, nil
		}
		slog.Info("remote: pool: stale connection, redialing", "target", key, "error", err)
		client.Close()
		delete(p.clients, key)
	}

	authMethods, err := BuildAuthMethods()
	if err != nil {
		return nil, fmt.Errorf("remote: pool: dial %s: %w", key, err)
	}

	hostKeyCB, err := BuildHostKeyCallback()
	if err != nil {
		return nil, fmt.Errorf("remote: pool: dial %s: %w", key, err)
	}

	config := &ssh.ClientConfig{
		User:            target.User,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCB,
		Timeout:         p.dialTimeout,
	}

	addr := fmt.Sprintf("%s:%d", target.Host, target.Port)
	conn, err := dialTCP(target.Host, target.Port, p.dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("remote: pool: dial %s: %w", key, err)
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, config)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("remote: pool: dial %s: %w", key, err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)
	slog.Info("remote: pool: new connection", "target", key)
	p.clients[key] = client
	return client, nil
}

// CloseAll closes all pooled connections and clears the pool.
func (p *ClientPool) CloseAll() {
	p.mu.Lock()
	defer p.mu.Unlock()

	for key, client := range p.clients {
		if err := client.Close(); err != nil {
			slog.Warn("remote: pool: error closing connection", "target", key, "error", err)
		}
	}

	p.clients = make(map[string]*ssh.Client)
	slog.Info("remote: pool: all connections closed")
}

var (
	defaultPool     *ClientPool
	defaultPoolOnce sync.Once
)

// DefaultPool returns the shared connection pool, creating it on first use.
func DefaultPool() *ClientPool {
	defaultPoolOnce.Do(func() {
		defaultPool = NewClientPool(30 * time.Second)
	})
	return defaultPool
}

// CloseDefaultPool closes all connections in the default pool.
func CloseDefaultPool() {
	if defaultPool != nil {
		defaultPool.CloseAll()
	}
}

// ExecResult holds the output of a remote command.
type ExecResult struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// Exec runs a command on the remote host with the given timeout and output cap.
func Exec(client *ssh.Client, cmd string, timeout time.Duration, maxOutputBytes int) (ExecResult, error) {
	session, err := client.NewSession()
	if err != nil {
		return ExecResult{}, fmt.Errorf("remote: exec: new session: %w", err)
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	if err := session.Start(cmd); err != nil {
		return ExecResult{}, fmt.Errorf("remote: exec: start %q: %w", cmd, err)
	}

	// Wait for completion in a goroutine so we can enforce timeout
	done := make(chan error, 1)
	go func() {
		done <- session.Wait()
	}()

	timer := time.AfterFunc(timeout, func() {
		session.Close()
	})

	waitErr := <-done
	timer.Stop()

	result := ExecResult{}

	// Extract exit code
	if waitErr != nil {
		var exitErr *ssh.ExitError
		var exitMissing *ssh.ExitMissingError
		switch {
		case errors.As(waitErr, &exitErr):
			result.ExitCode = exitErr.ExitStatus()
		case errors.As(waitErr, &exitMissing):
			result.ExitCode = -1
		default:
			return ExecResult{}, fmt.Errorf("remote: exec: wait %q: %w", cmd, waitErr)
		}
	}

	// Truncate stdout if needed
	stdoutStr := stdoutBuf.String()
	if len(stdoutStr) > maxOutputBytes {
		stdoutStr = stdoutStr[:maxOutputBytes] + fmt.Sprintf("\n[truncated: output exceeded %d bytes]", maxOutputBytes)
	}
	result.Stdout = stdoutStr

	// Truncate stderr if needed
	stderrStr := stderrBuf.String()
	if len(stderrStr) > maxOutputBytes {
		stderrStr = stderrStr[:maxOutputBytes] + fmt.Sprintf("\n[truncated: output exceeded %d bytes]", maxOutputBytes)
	}
	result.Stderr = stderrStr

	return result, nil
}

// DownloadFile copies a remote file to a local path using cat over SSH.
func DownloadFile(client *ssh.Client, remotePath, localPath string, maxBytes int64, timeout time.Duration) (int64, error) {
	session, err := client.NewSession()
	if err != nil {
		return 0, fmt.Errorf("remote: download: new session: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return 0, fmt.Errorf("remote: download: stdout pipe: %w", err)
	}

	cmd := "cat " + ShellEscape(remotePath)
	if err := session.Start(cmd); err != nil {
		return 0, fmt.Errorf("remote: download: start %q: %w", cmd, err)
	}

	// Enforce timeout via context-style cancellation
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	go func() {
		<-ctx.Done()
		if ctx.Err() == context.DeadlineExceeded {
			session.Close()
		}
	}()

	localFile, err := os.Create(localPath)
	if err != nil {
		return 0, fmt.Errorf("remote: download: create local file %s: %w", localPath, err)
	}

	writer := bufio.NewWriter(localFile)
	// Read up to maxBytes+1 so we can detect overflow
	limitedReader := &io.LimitedReader{R: stdout, N: maxBytes + 1}

	written, copyErr := io.Copy(writer, limitedReader)

	// Flush and close the local file regardless of copy outcome
	flushErr := writer.Flush()
	closeErr := localFile.Close()

	if written > maxBytes {
		// Exceeded limit — clean up the local file
		os.Remove(localPath)
		session.Close()
		return 0, fmt.Errorf("remote: download: file %s exceeds maximum size of %d bytes", remotePath, maxBytes)
	}

	if copyErr != nil {
		os.Remove(localPath)
		return 0, fmt.Errorf("remote: download: copy: %w", copyErr)
	}
	if flushErr != nil {
		os.Remove(localPath)
		return 0, fmt.Errorf("remote: download: flush: %w", flushErr)
	}
	if closeErr != nil {
		return 0, fmt.Errorf("remote: download: close local file: %w", closeErr)
	}

	// Wait for session to complete
	if err := session.Wait(); err != nil {
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitStatus() != 0 {
			os.Remove(localPath)
			return 0, fmt.Errorf("remote: download: cat exited with status %d", exitErr.ExitStatus())
		}
	}

	return written, nil
}

// ExportJournal runs journalctl on the remote host and writes output to a local file.
func ExportJournal(client *ssh.Client, unit, since, until, localPath string, maxBytes int64, timeout time.Duration) (int64, error) {
	cmd := "journalctl -u " + ShellEscape(unit) + " --no-pager -o short-iso"
	if since != "" {
		cmd += " --since " + ShellEscape(since)
	}
	if until != "" {
		cmd += " --until " + ShellEscape(until)
	}

	session, err := client.NewSession()
	if err != nil {
		return 0, fmt.Errorf("remote: journal: new session: %w", err)
	}
	defer session.Close()

	stdout, err := session.StdoutPipe()
	if err != nil {
		return 0, fmt.Errorf("remote: journal: stdout pipe: %w", err)
	}

	var stderrBuf bytes.Buffer
	session.Stderr = &stderrBuf

	if err := session.Start(cmd); err != nil {
		return 0, fmt.Errorf("remote: journal: start %q: %w", cmd, err)
	}

	// Enforce timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	go func() {
		<-ctx.Done()
		if ctx.Err() == context.DeadlineExceeded {
			session.Close()
		}
	}()

	localFile, err := os.Create(localPath)
	if err != nil {
		return 0, fmt.Errorf("remote: journal: create local file %s: %w", localPath, err)
	}

	writer := bufio.NewWriter(localFile)
	limitedReader := &io.LimitedReader{R: stdout, N: maxBytes + 1}

	written, copyErr := io.Copy(writer, limitedReader)

	flushErr := writer.Flush()
	closeErr := localFile.Close()

	if written > maxBytes {
		os.Remove(localPath)
		session.Close()
		return 0, fmt.Errorf("remote: journal: output exceeds maximum size of %d bytes", maxBytes)
	}

	if copyErr != nil {
		os.Remove(localPath)
		return 0, fmt.Errorf("remote: journal: copy: %w", copyErr)
	}
	if flushErr != nil {
		os.Remove(localPath)
		return 0, fmt.Errorf("remote: journal: flush: %w", flushErr)
	}
	if closeErr != nil {
		return 0, fmt.Errorf("remote: journal: close local file: %w", closeErr)
	}

	// Wait for session to complete and check for journalctl availability
	waitErr := session.Wait()
	stderrStr := stderrBuf.String()

	if waitErr != nil {
		var exitErr *ssh.ExitError
		if errors.As(waitErr, &exitErr) {
			exitCode := exitErr.ExitStatus()
			if exitCode == 127 || bytes.Contains(stderrBuf.Bytes(), []byte("command not found")) {
				os.Remove(localPath)
				return 0, fmt.Errorf("remote: journalctl not available on host")
			}
			// Non-zero exit but not "not found" — could be partial output, return what we have
			slog.Warn("remote: journal: journalctl exited with non-zero status",
				"exit_code", exitCode, "stderr", stderrStr)
		} else {
			os.Remove(localPath)
			return 0, fmt.Errorf("remote: journal: wait: %w", waitErr)
		}
	} else if bytes.Contains(stderrBuf.Bytes(), []byte("command not found")) {
		// Some shells report "command not found" on stderr but exit 0 via wrapper
		os.Remove(localPath)
		return 0, fmt.Errorf("remote: journalctl not available on host")
	}

	return written, nil
}
