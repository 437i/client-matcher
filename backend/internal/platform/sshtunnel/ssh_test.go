package sshtunnel

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/437i/client-matcher/backend/internal/platform/config"
)

func keys(t *testing.T) (ed25519.PrivateKey, ssh.Signer) {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return key, signer
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
}

func listen(t *testing.T) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func serverConfig(host, userKey ssh.Signer) *ssh.ServerConfig {
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			if meta.User() != "site" || string(password) != "synthetic" {
				return nil, errors.New("denied")
			}
			return nil, nil
		},
		PublicKeyCallback: func(meta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if meta.User() != "od" || !bytes.Equal(key.Marshal(), userKey.PublicKey().Marshal()) {
				return nil, errors.New("denied")
			}
			return nil, nil
		},
	}
	cfg.AddHostKey(host)
	return cfg
}

func TestSSHAuthenticationAndTrust(t *testing.T) {
	_, siteHost := keys(t)
	_, odHost := keys(t)
	userKey, odSigner := keys(t)
	_, other := keys(t)
	site, od := listen(t), listen(t)
	for listener, host := range map[net.Listener]ssh.Signer{site: siteHost, od: odHost} {
		go func() {
			for {
				raw, err := listener.Accept()
				if err != nil {
					return
				}
				go func() {
					defer raw.Close()
					conn, channels, requests, err := ssh.NewServerConn(raw, serverConfig(host, odSigner))
					if err != nil {
						return
					}
					defer conn.Close()
					go ssh.DiscardRequests(requests)
					for channel := range channels {
						channel.Reject(ssh.Prohibited, "synthetic")
					}
				}()
			}
		}()
	}
	dir := t.TempDir()
	hosts := filepath.Join(dir, "known_hosts")
	keyPath := filepath.Join(dir, "id")
	block, err := ssh.MarshalPrivateKey(userKey, "synthetic")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, keyPath, string(pem.EncodeToMemory(block)))
	siteLine := knownhosts.Line([]string{site.Addr().String()}, siteHost.PublicKey())
	odLine := knownhosts.Line([]string{od.Addr().String()}, odHost.PublicKey())
	siteConfig := config.SSHConfig{Address: site.Addr().String(), User: "site", Password: "synthetic"}
	odConfig := config.SSHConfig{Address: od.Addr().String(), User: "od", KeyPath: keyPath}
	check := func(t *testing.T, cfg config.SSHConfig, path string, success bool) {
		t.Helper()
		auth, err := authentication(cfg)
		if err != nil {
			t.Fatal(err)
		}
		client, raw, err := connect(t.Context(), cfg, auth, path)
		if err == nil {
			raw.Close()
			client.Wait()
		}
		if (err == nil) != success {
			t.Fatalf("connect success=%v: %v", success, err)
		}
	}
	both := siteLine + "\n" + odLine + "\n"
	writeFile(t, hosts, both)
	check(t, siteConfig, hosts, true)
	check(t, odConfig, hosts, true)
	badAuth := siteConfig
	badAuth.Password = "incorrect"
	check(t, badAuth, hosts, false)
	badAuth = odConfig
	badAuth.User = "incorrect"
	check(t, badAuth, hosts, false)
	check(t, siteConfig, filepath.Join(dir, "missing"), false)
	check(t, siteConfig, dir, false)
	if err := os.Chmod(hosts, 0); err != nil {
		t.Fatal(err)
	}
	check(t, siteConfig, hosts, false)
	if err := os.Chmod(hosts, 0600); err != nil {
		t.Fatal(err)
	}
	for name, contents := range map[string]string{
		"empty": "", "comments": "# comment\n  ", "malformed": "garbage\n",
		"malformed after valid": both + "garbage\n",
		"label after valid":     both + strings.Replace(siteLine, "ssh-ed25519", "ssh-rsa", 1) + "\n",
		"unknown":               odLine + "\n",
		"replacement":           knownhosts.Line([]string{site.Addr().String()}, other.PublicKey()) + "\n",
		"revoked":               both + "@revoked " + siteLine + "\n",
	} {
		t.Run(name, func(t *testing.T) { writeFile(t, hosts, contents); check(t, siteConfig, hosts, false) })
	}
}

func TestEncryptedODKeyRejectedBeforeConnection(t *testing.T) {
	key, signer := keys(t)
	listener := listen(t).(*net.TCPListener)
	dir := t.TempDir()
	keyPath, hosts := filepath.Join(dir, "key"), filepath.Join(dir, "known_hosts")
	const passphrase = "synthetic-passphrase"
	block, err := ssh.MarshalPrivateKeyWithPassphrase(key, "synthetic", []byte(passphrase))
	if err != nil {
		t.Fatal(err)
	}
	encoded := pem.EncodeToMemory(block)
	if _, err := ssh.ParsePrivateKeyWithPassphrase(encoded, []byte(passphrase)); err != nil {
		t.Fatal(err)
	}
	writeFile(t, keyPath, string(encoded))
	writeFile(t, hosts, knownhosts.Line([]string{listener.Addr().String()}, signer.PublicKey())+"\n")
	tunnel, err := New(config.SSHConfig{Address: listener.Addr().String(), User: "od", KeyPath: keyPath}, "localhost:5432", hosts)
	if err == nil {
		tunnel.Close()
		t.Fatal("encrypted key accepted")
	}
	if !strings.Contains(err.Error(), "unencrypted") || strings.Contains(err.Error(), passphrase) {
		t.Fatal(err)
	}
	if err := listener.SetDeadline(time.Now().Add(50 * time.Millisecond)); err != nil {
		t.Fatal(err)
	}
	conn, err := listener.Accept()
	if conn != nil {
		conn.Close()
		t.Fatal("dialed before rejecting key")
	}
	var timeout net.Error
	if !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatalf("unexpected accept error: %v", err)
	}
}

func TestHandshakeTimeoutAndCancellation(t *testing.T) {
	for _, cancelEarly := range []bool{false, true} {
		listener := listen(t)
		_, host := keys(t)
		hosts := filepath.Join(t.TempDir(), "known_hosts")
		writeFile(t, hosts, knownhosts.Line([]string{listener.Addr().String()}, host.PublicKey())+"\n")
		closed := make(chan struct{})
		go func() {
			defer close(closed)
			raw, err := listener.Accept()
			if err != nil {
				return
			}
			defer raw.Close()
			if cancelEarly {
				_, _ = raw.Write([]byte("SSH-2.0-stalled\r\n"))
			}
			_, _ = io.Copy(io.Discard, raw)
		}()
		ctx, cancel := context.WithCancel(t.Context())
		var timer *time.Timer
		if cancelEarly {
			timer = time.AfterFunc(250*time.Millisecond, cancel)
		}
		started := time.Now()
		cfg := config.SSHConfig{Address: listener.Addr().String(), User: "site"}
		_, _, err := connect(ctx, cfg, ssh.Password("synthetic"), hosts)
		cancel()
		if timer != nil {
			timer.Stop()
		}
		elapsed := time.Since(started)
		if err == nil || elapsed > handshakeTimeout+2*time.Second || (!cancelEarly && elapsed < handshakeTimeout) || (cancelEarly && elapsed > 2*time.Second) {
			t.Fatalf("unexpected handshake result after %s: %v", elapsed, err)
		}
		select {
		case <-closed:
		case <-time.After(time.Second):
			t.Fatal("stalled handshake retained raw connection")
		}
	}
}
