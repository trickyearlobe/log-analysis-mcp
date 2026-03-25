package remote

import (
	"fmt"
	"log/slog"
	"net"
	"runtime"
	"sync"
	"time"
)

// dialStrategy records whether direct TCP dial works on this host.
// Once we discover that direct dial fails (macOS firewall), we skip
// straight to the proxy for all subsequent connections.
type dialStrategy int

const (
	strategyUnknown dialStrategy = iota
	strategyDirect               // net.Dial works fine
	strategyProxy                // net.Dial blocked, use ssh -W
)

var (
	currentStrategy dialStrategy
	strategyMu      sync.Mutex
)

// dialTCP establishes a TCP connection to host:port. It tries net.Dial first.
// If that fails and we're on macOS (darwin), it falls back to spawning
// /usr/bin/ssh -W as a proxy — the system SSH binary is pre-authorized by
// macOS Application Firewall even when locally-built Go binaries are blocked.
//
// The chosen strategy is cached so subsequent calls skip the failing path.
func dialTCP(host string, port int, timeout time.Duration) (net.Conn, error) {
	addr := fmt.Sprintf("%s:%d", host, port)

	strategyMu.Lock()
	strat := currentStrategy
	strategyMu.Unlock()

	switch strat {
	case strategyDirect:
		return directDial(addr, timeout)

	case strategyProxy:
		return proxyDial(host, port, timeout)

	default:
		// Unknown — probe with direct dial first.
		conn, directErr := directDial(addr, timeout)
		if directErr == nil {
			strategyMu.Lock()
			currentStrategy = strategyDirect
			strategyMu.Unlock()
			slog.Info("remote: dialer: direct TCP works, using for all connections")
			return conn, nil
		}

		// Direct dial failed. Only fall back to proxy on macOS.
		if runtime.GOOS != "darwin" {
			return nil, fmt.Errorf("remote: dial %s: %w", addr, directErr)
		}

		slog.Warn("remote: dialer: direct TCP failed on macOS, trying ssh proxy fallback",
			"addr", addr, "direct_error", directErr)

		conn, proxyErr := proxyDial(host, port, timeout)
		if proxyErr != nil {
			return nil, fmt.Errorf("remote: dial %s: direct: %v; proxy: %w", addr, directErr, proxyErr)
		}

		strategyMu.Lock()
		currentStrategy = strategyProxy
		strategyMu.Unlock()
		slog.Warn("remote: dialer: ssh proxy fallback succeeded, using for all connections")
		return conn, nil
	}
}

// directDial does a plain net.DialTimeout TCP connection.
func directDial(addr string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("tcp", addr, timeout)
}

// resetDialStrategy resets the cached strategy to unknown. Used in tests.
func resetDialStrategy() {
	strategyMu.Lock()
	currentStrategy = strategyUnknown
	strategyMu.Unlock()
}
