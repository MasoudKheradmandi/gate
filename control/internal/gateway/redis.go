package gateway

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// Redis supervises the embedded, Redis-compatible store (Valkey) that holds the access
// queue state when no external Redis is configured. It listens on loopback only, loads
// the generated ACL file (default user disabled, H12) and keeps nothing on disk: queue
// state is short-lived by design. If the process dies it is restarted.
type Redis struct {
	Bin       string // e.g. /usr/bin/valkey-server
	Port      string // loopback port, e.g. 6379
	ACLFile   string
	Dir       string // working directory (writable; nothing is persisted)
	MaxMemory string // e.g. 256mb

	mu      sync.Mutex
	cmd     *exec.Cmd
	stopped bool
	exited  chan struct{}
}

func (r *Redis) args() []string {
	return []string{
		"--bind", "127.0.0.1", "--port", r.Port, "--protected-mode", "yes",
		"--aclfile", r.ACLFile, "--dir", r.Dir,
		"--save", "", "--appendonly", "no",
		"--maxmemory", r.MaxMemory, "--maxmemory-policy", "noeviction",
		"--daemonize", "no", "--logfile", "", "--loglevel", "warning",
	}
}

// Start launches the server, waits until it accepts connections and keeps it running
// until Stop is called or ctx ends.
func (r *Redis) Start(ctx context.Context) error {
	if err := os.MkdirAll(r.Dir, 0o700); err != nil {
		return err
	}
	if err := r.spawn(); err != nil {
		return err
	}
	go r.supervise(ctx)
	return WaitTCP(ctx, net.JoinHostPort("127.0.0.1", r.Port), 10*time.Second)
}

func (r *Redis) spawn() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.stopped {
		return errors.New("redis stopped")
	}
	cmd := exec.Command(r.Bin, r.args()...)
	cmd.Dir = r.Dir
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	r.cmd = cmd
	r.exited = make(chan struct{})
	go func(c *exec.Cmd, done chan struct{}) { _ = c.Wait(); close(done) }(cmd, r.exited)
	slog.Info("embedded redis started", "pid", cmd.Process.Pid, "port", r.Port)
	return nil
}

func (r *Redis) supervise(ctx context.Context) {
	backoff := time.Second
	for {
		r.mu.Lock()
		exited := r.exited
		r.mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-exited:
		}
		r.mu.Lock()
		stopped := r.stopped
		r.mu.Unlock()
		if stopped {
			return
		}
		slog.Error("embedded redis exited; restarting", "in", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
		if err := r.spawn(); err != nil {
			slog.Error("restart embedded redis", "err", err)
		}
	}
}

// Stop terminates the server (SIGTERM, then SIGKILL after timeout).
func (r *Redis) Stop(timeout time.Duration) {
	r.mu.Lock()
	r.stopped = true
	cmd, exited := r.cmd, r.exited
	r.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-exited:
	case <-time.After(timeout):
		_ = cmd.Process.Kill()
	}
}

// WaitTCP waits until addr accepts TCP connections.
func WaitTCP(ctx context.Context, addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		c, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			c.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
}
