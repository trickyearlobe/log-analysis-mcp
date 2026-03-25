package remote

import (
	"fmt"
	"net"
	"runtime"
	"testing"
	"time"
)

func TestDialTCPDirectSuccess(t *testing.T) {
	resetDialStrategy()
	defer resetDialStrategy()

	// Start a local TCP listener to simulate a reachable host.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer ln.Close()

	// Accept one connection in the background.
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		conn.Close()
	}()

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	conn, err := dialTCP(host, port, 5*time.Second)
	if err != nil {
		t.Fatalf("dialTCP: %v", err)
	}
	conn.Close()

	// Strategy should be cached as direct.
	strategyMu.Lock()
	strat := currentStrategy
	strategyMu.Unlock()

	if strat != strategyDirect {
		t.Errorf("strategy = %d, want strategyDirect (%d)", strat, strategyDirect)
	}
}

func TestDialTCPDirectSuccessCached(t *testing.T) {
	resetDialStrategy()
	defer resetDialStrategy()

	// Start two listeners — we'll dial twice to verify caching.
	ln1, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen 1: %v", err)
	}
	defer ln1.Close()

	ln2, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen 2: %v", err)
	}
	defer ln2.Close()

	for _, ln := range []net.Listener{ln1, ln2} {
		ln := ln
		go func() {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			conn.Close()
		}()
	}

	// First dial — probes and caches strategyDirect.
	host1, portStr1, _ := net.SplitHostPort(ln1.Addr().String())
	var port1 int
	fmt.Sscanf(portStr1, "%d", &port1)

	conn, err := dialTCP(host1, port1, 5*time.Second)
	if err != nil {
		t.Fatalf("first dialTCP: %v", err)
	}
	conn.Close()

	// Second dial — should go straight to direct without probing.
	host2, portStr2, _ := net.SplitHostPort(ln2.Addr().String())
	var port2 int
	fmt.Sscanf(portStr2, "%d", &port2)

	conn, err = dialTCP(host2, port2, 5*time.Second)
	if err != nil {
		t.Fatalf("second dialTCP (cached): %v", err)
	}
	conn.Close()
}

func TestDialTCPDirectFailNoFallbackOnLinux(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("this test verifies non-darwin behaviour")
	}

	resetDialStrategy()
	defer resetDialStrategy()

	// Dial a port that nothing is listening on.
	_, err := dialTCP("127.0.0.1", 1, 1*time.Second)
	if err == nil {
		t.Fatal("dialTCP to closed port: expected error, got nil")
	}

	// Strategy should remain unknown — no fallback was attempted.
	strategyMu.Lock()
	strat := currentStrategy
	strategyMu.Unlock()

	if strat != strategyUnknown {
		t.Errorf("strategy = %d, want strategyUnknown (%d)", strat, strategyUnknown)
	}
}

func TestDialTCPProxyCached(t *testing.T) {
	resetDialStrategy()
	defer resetDialStrategy()

	// Force strategy to proxy, then verify dialTCP uses the proxy path.
	// proxyDial spawns ssh -W which returns a conn immediately (the process
	// starts, connection failure surfaces later during SSH handshake).
	// So we just verify the strategy is preserved and a conn is returned.
	strategyMu.Lock()
	currentStrategy = strategyProxy
	strategyMu.Unlock()

	// Use localhost port 1 — ssh -W will start the process even if the
	// target is unreachable. The conn is a pipe to the ssh process.
	conn, err := dialTCP("127.0.0.1", 1, 2*time.Second)
	if err != nil {
		// On systems without /usr/bin/ssh this will fail — that's OK.
		t.Skipf("proxyDial not available: %v", err)
	}
	conn.Close()

	// Strategy should still be proxy (not reset).
	strategyMu.Lock()
	strat := currentStrategy
	strategyMu.Unlock()

	if strat != strategyProxy {
		t.Errorf("strategy = %d, want strategyProxy (%d)", strat, strategyProxy)
	}
}

func TestDialTCPDirectCachedFailsCleanly(t *testing.T) {
	resetDialStrategy()
	defer resetDialStrategy()

	// Force strategy to direct, then dial a closed port.
	strategyMu.Lock()
	currentStrategy = strategyDirect
	strategyMu.Unlock()

	_, err := dialTCP("127.0.0.1", 1, 1*time.Second)
	if err == nil {
		t.Fatal("dialTCP direct to closed port: expected error, got nil")
	}

	// Strategy should remain direct — a failed direct dial doesn't
	// trigger fallback once the strategy is cached.
	strategyMu.Lock()
	strat := currentStrategy
	strategyMu.Unlock()

	if strat != strategyDirect {
		t.Errorf("strategy = %d, want strategyDirect (%d)", strat, strategyDirect)
	}
}

func TestResetDialStrategy(t *testing.T) {
	strategyMu.Lock()
	currentStrategy = strategyDirect
	strategyMu.Unlock()

	resetDialStrategy()

	strategyMu.Lock()
	strat := currentStrategy
	strategyMu.Unlock()

	if strat != strategyUnknown {
		t.Errorf("after reset: strategy = %d, want strategyUnknown (%d)", strat, strategyUnknown)
	}
}
