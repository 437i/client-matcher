package sshtunnel

import (
	"context"
	"errors"
	"io"
	"net"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/437i/client-matcher/backend/internal/platform/config"
)

func TestIndependentTunnelRecovery(t *testing.T) {
	if keepaliveInterval != 15*time.Second || operationTimeout != 5*time.Second {
		t.Fatal("unexpected production deadlines")
	}
	_, firstKey := keys(t)
	_, secondKey := keys(t)
	_, wrongKey := keys(t)
	first, second := listen(t), listen(t)
	hosts := filepath.Join(t.TempDir(), "known_hosts")
	writeFile(t, hosts, knownhosts.Line([]string{first.Addr().String()}, firstKey.PublicKey())+"\n"+knownhosts.Line([]string{second.Addr().String()}, secondKey.PublicKey())+"\n")
	var mode atomic.Int32
	type peer struct {
		conn net.Conn
		done chan struct{}
	}
	requests := make(chan bool, 128)
	channels := make(chan struct{}, 128)
	firstPeers, secondPeers := make(chan peer, 32), make(chan peer, 32)
	serve := func(l net.Listener, key ssh.Signer, peers chan peer, faultable bool) {
		for {
			raw, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				defer raw.Close()
				host := key
				if faultable && mode.Load() == 3 {
					host = wrongKey
				}
				server, incoming, global, err := ssh.NewServerConn(raw, serverConfig(host, host))
				if err != nil {
					return
				}
				defer server.Close()
				done := make(chan struct{})
				defer close(done)
				peers <- peer{raw, done}
				var workers sync.WaitGroup
				workers.Go(func() {
					for req := range global {
						if req.Type == "keepalive@openssh.com" {
							select {
							case requests <- req.WantReply:
							default:
							}
							if !faultable || mode.Load() != 1 {
								_ = req.Reply(false, nil)
							}
						}
					}
				})
				for ch := range incoming {
					if faultable && mode.Load() == 2 {
						select {
						case channels <- struct{}{}:
						default:
						}
						continue
					}
					if faultable && mode.Load() == 4 {
						_ = ch.Reject(ssh.ConnectionFailed, "synthetic refusal")
						continue
					}
					c, reqs, err := ch.Accept()
					if err != nil {
						continue
					}
					workers.Go(func() { ssh.DiscardRequests(reqs) })
					workers.Go(func() { defer c.Close(); _, _ = io.Copy(c, c); _ = c.CloseWrite() })
				}
				workers.Wait()
			}()
		}
	}
	go serve(first, firstKey, firstPeers, true)
	go serve(second, secondKey, secondPeers, false)
	var firstResets, secondResets atomic.Int32
	makeTunnel := func(l net.Listener, resets *atomic.Int32) *Tunnel {
		t.Helper()
		tunnel, err := New(config.SSHConfig{Address: l.Addr().String(), User: "site", Password: "synthetic"}, "localhost:5432", hosts)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(tunnel.Close)
		tunnel.keepaliveEvery = 30 * time.Millisecond
		tunnel.operationLimit = 90 * time.Millisecond
		tunnel.initialBackoff = 130 * time.Millisecond
		if err := tunnel.Start(t.Context(), func() { resets.Add(1) }, func() {}); err != nil {
			t.Fatal(err)
		}
		if err := EnableRecovery(tunnel); err != nil {
			t.Fatal(err)
		}
		return tunnel
	}
	a, b := makeTunnel(first, &firstResets), makeTunnel(second, &secondResets)
	getPeer := func(peers chan peer) peer {
		t.Helper()
		select {
		case p := <-peers:
			return p
		case <-time.After(3 * time.Second):
			t.Fatal("missing SSH session")
			return peer{}
		}
	}
	old := getPeer(firstPeers)
	_ = getPeer(secondPeers)
	newSocket := func(tunnel *Tunnel) net.Conn {
		t.Helper()
		c, err := tunnel.DialContext(t.Context(), "tcp", tunnel.Address())
		if err != nil {
			t.Fatal(err)
		}
		_ = c.SetDeadline(time.Now().Add(2 * time.Second))
		t.Cleanup(func() { c.Close() })
		return c
	}
	echo := func(c net.Conn) {
		t.Helper()
		if _, err := c.Write([]byte("echo")); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, 4)
		if _, err := io.ReadFull(c, buf); err != nil || string(buf) != "echo" {
			t.Fatalf("probe got %q: %v", buf, err)
		}
	}
	waitDone := func(p peer) {
		t.Helper()
		select {
		case <-p.done:
		case <-time.After(2 * time.Second):
			t.Fatal("old SSH operations did not drain")
		}
	}
	echo(newSocket(a))
	echo(newSocket(b))
	select {
	case want := <-requests:
		if !want {
			t.Fatal("keepalive did not request reply")
		}
	case <-time.After(time.Second):
		t.Fatal("missing keepalive")
	}
	previous := newSocket(a)
	echo(previous)
	mode.Store(1)
	waitDone(old)
	if _, err := previous.Read(make([]byte, 1)); err == nil {
		t.Fatal("old local socket survived")
	}
	mode.Store(3)
	select {
	case <-firstPeers:
		t.Fatal("unexpected session with changed key")
	case <-time.After(420 * time.Millisecond):
	}
	echo(newSocket(b))
	if secondResets.Load() != 0 {
		t.Fatal("unaffected pool reset")
	}
	mode.Store(0)
	old = getPeer(firstPeers)
	waitUntil(t, func() bool { a.mu.Lock(); defer a.mu.Unlock(); return available(a.current) })
	if valid, err := a.CheckConn(previous); err != nil || valid {
		t.Fatalf("stale connection accepted: %v", err)
	}
	echo(newSocket(a))

	mode.Store(4)
	refused := newSocket(a)
	if _, err := refused.Read(make([]byte, 1)); err == nil {
		t.Fatal("refused forward open")
	}
	if firstResets.Load() != 1 {
		t.Fatal("channel rejection reset session")
	}
	mode.Store(2)
	stalled := newSocket(a)
	select {
	case <-channels:
	case <-time.After(time.Second):
		t.Fatal("missing stalled open")
	}
	waitDone(old)
	if _, err := stalled.Read(make([]byte, 1)); err == nil {
		t.Fatal("stalled channel retained socket")
	}
	mode.Store(0)
	old = getPeer(firstPeers)
	waitUntil(t, func() bool { a.mu.Lock(); defer a.mu.Unlock(); return available(a.current) })
	echo(newSocket(a))
	echo(newSocket(b))

	// Race parallel accepts against loss of the recovered transport.
	var dialing sync.WaitGroup
	for range 40 {
		dialing.Go(func() {
			c, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", a.Address())
			if err == nil {
				c.Close()
			}
		})
	}
	old.conn.Close()
	dialing.Wait()
	waitDone(old)
	waitUntil(t, func() bool { return firstResets.Load() == 3 })
	a.Close()
	if _, err := a.DialContext(t.Context(), "tcp", a.Address()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("dial after close: %v", err)
	}
	if c, err := net.DialTimeout("tcp", a.Address(), time.Second); err == nil {
		c.Close()
		t.Fatal("listener did not close")
	}
	select {
	case <-firstPeers:
		t.Fatal("reconnect after close")
	case <-time.After(200 * time.Millisecond):
	}
	echo(newSocket(b))
}

func waitUntil(t *testing.T, condition func() bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	for !condition() {
		select {
		case <-ctx.Done():
			t.Fatal("condition did not become true")
		case <-time.After(time.Millisecond):
		}
	}
}

func TestBackoffCap(t *testing.T) {
	got := time.Second
	for _, want := range []time.Duration{2, 4, 8, 16, 30, 30} {
		got = nextBackoff(got)
		if got != want*time.Second {
			t.Fatalf("backoff=%s, want %ds", got, want)
		}
	}
}
