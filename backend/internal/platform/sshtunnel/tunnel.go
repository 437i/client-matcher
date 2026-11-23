package sshtunnel

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/437i/client-matcher/backend/internal/platform/config"
)

const (
	keepaliveInterval = 15 * time.Second
	operationTimeout  = 5 * time.Second
	maxForwards       = 32
)

var ErrUnavailable = errors.New("SSH tunnel unavailable")

type Tunnel struct {
	listener           net.Listener
	cfg                config.SSHConfig
	target, knownHosts string
	auth               ssh.AuthMethod
	ctx                context.Context
	cancel             context.CancelFunc
	workers            sync.WaitGroup

	mu                 sync.Mutex
	current            *session
	generation         uint64
	recover            bool
	reset, startupLost func()

	// Set before Start. Tests use shorter intervals with real SSH peers.
	keepaliveEvery, operationLimit, initialBackoff time.Duration
}

type session struct {
	client     *ssh.Client
	raw        net.Conn
	lost       chan struct{}
	generation uint64
	sockets    map[net.Conn]struct{}
	workers    sync.WaitGroup
}

type generationConn struct {
	net.Conn
	generation uint64
}

func New(cfg config.SSHConfig, target, knownHosts string) (*Tunnel, error) {
	auth, err := authentication(cfg)
	if err != nil {
		return nil, err
	}
	if _, err := hostKeyCallback(knownHosts); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	t := &Tunnel{
		listener: listener, cfg: cfg, target: target, knownHosts: knownHosts, auth: auth,
		ctx: ctx, cancel: cancel, keepaliveEvery: keepaliveInterval,
		operationLimit: operationTimeout, initialBackoff: time.Second,
	}
	t.workers.Go(t.accept)
	return t, nil
}

func (t *Tunnel) Address() string { return t.listener.Addr().String() }

// Start binds the pool before publishing the initial session. Call once, before Close.
func (t *Tunnel) Start(ctx context.Context, reset, startupLost func()) error {
	s, err := t.connect(ctx)
	if err != nil {
		return err
	}
	t.reset, t.startupLost = reset, startupLost
	t.publish(s)
	t.workers.Go(func() { t.supervise(s) })
	return nil
}

// EnableRecovery switches all startup tunnels together after their final database checks.
// The composition root calls it once, with distinct tunnels in startup order.
func EnableRecovery(tunnels ...*Tunnel) error {
	for _, tunnel := range tunnels {
		tunnel.mu.Lock()
		defer tunnel.mu.Unlock()
	}
	for _, tunnel := range tunnels {
		if tunnel.ctx.Err() != nil || !available(tunnel.current) {
			return ErrUnavailable
		}
	}
	for _, tunnel := range tunnels {
		tunnel.recover = true
	}
	return nil
}

func (t *Tunnel) Close() {
	t.cancel()
	t.listener.Close()
	t.workers.Wait()
}

func (t *Tunnel) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	t.mu.Lock()
	if !available(t.current) {
		t.mu.Unlock()
		return nil, ErrUnavailable
	}
	generation := t.current.generation
	t.mu.Unlock()
	conn, err := (&net.Dialer{}).DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return &generationConn{Conn: conn, generation: generation}, nil
}

func (t *Tunnel) CheckConn(conn net.Conn) (bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !available(t.current) {
		return false, ErrUnavailable
	}
	tagged, ok := conn.(*generationConn)
	return ok && tagged.generation == t.current.generation, nil
}

func available(s *session) bool {
	if s == nil {
		return false
	}
	select {
	case <-s.lost:
		return false
	default:
		return true
	}
}

func (t *Tunnel) connect(ctx context.Context) (*session, error) {
	client, raw, err := connect(ctx, t.cfg, t.auth, t.knownHosts)
	if err != nil {
		return nil, err
	}
	return &session{client: client, raw: raw, lost: make(chan struct{}), sockets: make(map[net.Conn]struct{})}, nil
}

func (t *Tunnel) publish(s *session) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.generation++
	s.generation = t.generation
	t.current = s
}

func (t *Tunnel) fail(s *session) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.current != s || !available(s) {
		return
	}
	close(s.lost)
	if !t.recover {
		t.startupLost()
	}
}

func (t *Tunnel) supervise(s *session) {
	backoff := t.initialBackoff
	for {
		s.workers.Go(func() {
			_ = s.client.Wait()
			t.fail(s)
		})
		if t.monitor(s) {
			backoff = t.initialBackoff
		}

		t.mu.Lock()
		t.current = nil
		sockets := s.sockets
		s.sockets = nil
		resume := t.recover
		t.mu.Unlock()

		s.raw.Close()
		for socket := range sockets {
			socket.Close()
		}
		t.reset()
		s.workers.Wait()
		if !resume || t.ctx.Err() != nil {
			return
		}

		for {
			select {
			case <-t.ctx.Done():
				return
			case <-time.After(backoff):
			}
			backoff = nextBackoff(backoff)
			replacement, err := t.connect(t.ctx)
			if err == nil {
				s = replacement
				t.publish(s)
				break
			}
		}
	}
}

func (t *Tunnel) monitor(s *session) (healthy bool) {
	ticker := time.NewTicker(t.keepaliveEvery)
	defer ticker.Stop()
	for {
		select {
		case <-t.ctx.Done():
			return healthy
		case <-s.lost:
			return healthy
		case <-ticker.C:
			reply := make(chan error, 1)
			s.workers.Go(func() {
				_, _, err := s.client.SendRequest("keepalive@openssh.com", true, nil)
				reply <- err
			})
			timer := time.NewTimer(t.operationLimit)
			select {
			case err := <-reply:
				if err != nil {
					t.fail(s)
				} else {
					healthy = true
				}
			case <-timer.C:
				t.fail(s)
			case <-s.lost:
			case <-t.ctx.Done():
			}
			timer.Stop()
		}
	}
}

func (t *Tunnel) accept() {
	for {
		local, err := t.listener.Accept()
		if err != nil {
			return
		}
		t.mu.Lock()
		s := t.current
		if !available(s) || t.ctx.Err() != nil || len(s.sockets) >= maxForwards {
			t.mu.Unlock()
			local.Close()
			continue
		}
		s.sockets[local] = struct{}{}
		s.workers.Go(func() { t.forward(s, local) })
		t.mu.Unlock()
	}
}

func (t *Tunnel) forward(s *session, local net.Conn) {
	defer func() {
		local.Close()
		t.mu.Lock()
		delete(s.sockets, local)
		t.mu.Unlock()
	}()
	type result struct {
		conn net.Conn
		err  error
	}
	resultCh := make(chan result)
	timer := time.NewTimer(t.operationLimit)
	defer timer.Stop()
	finished := make(chan struct{})
	defer close(finished)
	s.workers.Go(func() {
		conn, err := s.client.Dial("tcp", t.target)
		select {
		case resultCh <- result{conn, err}:
		case <-finished:
			if conn != nil {
				conn.Close()
			}
		}
	})
	select {
	case <-t.ctx.Done():
		return
	case <-s.lost:
		return
	case <-timer.C:
		t.fail(s)
	case opened := <-resultCh:
		if opened.err != nil {
			var rejected *ssh.OpenChannelError
			if !errors.As(opened.err, &rejected) {
				t.fail(s)
			}
			return
		}
		defer opened.conn.Close()
		if available(s) && t.ctx.Err() == nil {
			relay(local, opened.conn)
		}
	}
}

func relay(local, remote net.Conn) {
	// Both TCPConn and the direct-tcpip connection support half-close.
	copyHalf := func(dst, src net.Conn) {
		_, err := io.Copy(dst, src)
		if err == nil {
			err = dst.(interface{ CloseWrite() error }).CloseWrite()
		}
		if err != nil {
			local.Close()
			remote.Close()
		}
	}
	var copyWorkers sync.WaitGroup
	copyWorkers.Go(func() { copyHalf(local, remote) })
	copyHalf(remote, local)
	copyWorkers.Wait()
}

func nextBackoff(delay time.Duration) time.Duration {
	return min(2*delay, 30*time.Second)
}
