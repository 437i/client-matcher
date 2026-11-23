// Package sshtest provides a loopback SSH peer for synthetic integration tests.
package sshtest

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/437i/client-matcher/backend/internal/platform/config"
)

const Password = "synthetic-ssh"

type Peer struct {
	Config    config.SSHConfig
	KnownHost string
	listener  net.Listener
	mu        sync.Mutex
	clients   map[net.Conn]struct{}
	workers   sync.WaitGroup
}

func New(t testing.TB, address, target string, publicKey bool) *Peer {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	host, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp4", address)
	if err != nil {
		t.Fatal(err)
	}
	p := &Peer{listener: listener, clients: make(map[net.Conn]struct{}), Config: config.SSHConfig{Address: listener.Addr().String(), User: "synthetic"}}
	cfg := &ssh.ServerConfig{}
	cfg.AddHostKey(host)
	if publicKey {
		_, identity, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		signer, err := ssh.NewSignerFromKey(identity)
		if err != nil {
			t.Fatal(err)
		}
		block, err := ssh.MarshalPrivateKey(identity, "synthetic OD")
		if err != nil {
			t.Fatal(err)
		}
		p.Config.KeyPath = filepath.Join(t.TempDir(), "od-key")
		if err := os.WriteFile(p.Config.KeyPath, pem.EncodeToMemory(block), 0600); err != nil {
			t.Fatal(err)
		}
		cfg.PublicKeyCallback = func(m ssh.ConnMetadata, k ssh.PublicKey) (*ssh.Permissions, error) {
			if m.User() == p.Config.User && bytes.Equal(signer.PublicKey().Marshal(), k.Marshal()) {
				return nil, nil
			}
			return nil, errors.New("denied")
		}
	} else {
		p.Config.Password = Password
		cfg.PasswordCallback = func(m ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if m.User() == p.Config.User && string(password) == Password {
				return nil, nil
			}
			return nil, errors.New("denied")
		}
	}
	p.KnownHost = knownhosts.Line([]string{listener.Addr().String()}, host.PublicKey()) + "\n"
	p.workers.Go(func() {
		for {
			c, err := listener.Accept()
			if err != nil {
				return
			}
			p.mu.Lock()
			p.clients[c] = struct{}{}
			p.workers.Go(func() {
				defer func() { c.Close(); p.mu.Lock(); delete(p.clients, c); p.mu.Unlock() }()
				serve(c, cfg, target)
			})
			p.mu.Unlock()
		}
	})
	t.Cleanup(func() { listener.Close(); p.Drop(); p.workers.Wait() })
	return p
}

func (p *Peer) Drop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for c := range p.clients {
		c.Close()
	}
}

func (p *Peer) Connections() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.clients)
}

func serve(c net.Conn, cfg *ssh.ServerConfig, target string) {
	session, channels, requests, err := ssh.NewServerConn(c, cfg)
	if err != nil {
		return
	}
	defer session.Close()
	var workers sync.WaitGroup
	workers.Go(func() { ssh.DiscardRequests(requests) })
	for ch := range channels {
		if ch.ChannelType() != "direct-tcpip" {
			_ = ch.Reject(ssh.Prohibited, "invalid channel")
			continue
		}
		var requested struct {
			Host       string
			Port       uint32
			Origin     string
			OriginPort uint32
		}
		if ssh.Unmarshal(ch.ExtraData(), &requested) != nil || requested.Host != "localhost" || requested.Port != 5432 {
			_ = ch.Reject(ssh.Prohibited, "invalid forwarding address")
			continue
		}
		workers.Go(func() {
			remote, err := net.DialTimeout("tcp4", target, time.Second)
			if err != nil {
				_ = ch.Reject(ssh.ConnectionFailed, "unavailable")
				return
			}
			defer remote.Close()
			channel, reqs, err := ch.Accept()
			if err != nil {
				return
			}
			defer channel.Close()
			done := make(chan struct{})
			go func() { defer close(done); ssh.DiscardRequests(reqs) }()
			copied := make(chan struct{})
			go func() { defer close(copied); _, _ = io.Copy(channel, remote); _ = channel.CloseWrite() }()
			_, _ = io.Copy(remote, channel)
			remote.Close()
			channel.Close()
			<-copied
			<-done
		})
	}
	workers.Wait()
}
