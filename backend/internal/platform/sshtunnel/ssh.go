package sshtunnel

import (
	"bufio"
	"context"
	"encoding/base64"
	"errors"
	"net"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"github.com/437i/client-matcher/backend/internal/platform/config"
)

const handshakeTimeout = 5 * time.Second

func hostKeyCallback(path string) (ssh.HostKeyCallback, error) {
	invalid := errors.New("invalid SSH known_hosts file")
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, invalid
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, invalid
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	trusted := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		revoked := fields[0] == "@revoked"
		if revoked || fields[0] == "@cert-authority" {
			fields = fields[1:]
		} else if strings.HasPrefix(fields[0], "@") {
			return nil, invalid
		}
		if len(fields) < 3 {
			return nil, invalid
		}
		encoded, err := base64.StdEncoding.DecodeString(fields[2])
		if err != nil {
			return nil, invalid
		}
		key, err := ssh.ParsePublicKey(encoded)
		if err != nil || key.Type() != fields[1] {
			return nil, invalid
		}
		trusted = trusted || !revoked
	}
	if scanner.Err() != nil || !trusted {
		return nil, invalid
	}
	callback, err := knownhosts.New(path)
	if err != nil {
		return nil, invalid
	}
	return callback, nil
}

func authentication(cfg config.SSHConfig) (ssh.AuthMethod, error) {
	if cfg.KeyPath == "" {
		return ssh.Password(cfg.Password), nil
	}
	info, err := os.Stat(cfg.KeyPath)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("invalid SSH private key file")
	}
	key, err := os.ReadFile(cfg.KeyPath)
	if err != nil {
		return nil, errors.New("read SSH private key file failed")
	}
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, errors.New("SSH private key must be valid and unencrypted")
	}
	return ssh.PublicKeys(signer), nil
}

func connect(ctx context.Context, cfg config.SSHConfig, auth ssh.AuthMethod, path string) (*ssh.Client, net.Conn, error) {
	callback, err := hostKeyCallback(path)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, handshakeTimeout)
	defer cancel()
	raw, err := (&net.Dialer{}).DialContext(ctx, "tcp", cfg.Address)
	if err != nil {
		return nil, nil, errors.New("SSH TCP connection failed")
	}
	deadline, _ := ctx.Deadline()
	if err := raw.SetDeadline(deadline); err != nil {
		raw.Close()
		return nil, nil, errors.New("set SSH handshake deadline failed")
	}
	stop := context.AfterFunc(ctx, func() { raw.Close() })
	conn, channels, requests, err := ssh.NewClientConn(raw, cfg.Address, &ssh.ClientConfig{
		User: cfg.User, Auth: []ssh.AuthMethod{auth}, HostKeyCallback: callback,
	})
	if !stop() || err != nil || ctx.Err() != nil {
		raw.Close()
		return nil, nil, errors.New("SSH handshake failed")
	}
	if err := raw.SetDeadline(time.Time{}); err != nil {
		raw.Close()
		return nil, nil, errors.New("clear SSH handshake deadline failed")
	}
	return ssh.NewClient(conn, channels, requests), raw, nil
}
