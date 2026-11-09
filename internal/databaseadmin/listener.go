// Package databaseadmin contains the isolated local database administration runtime.
package databaseadmin

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	listenerReadHeaderTimeout = 5 * time.Second
	listenerReadTimeout       = 30 * time.Second
	listenerWriteTimeout      = 2 * time.Minute
	listenerIdleTimeout       = 60 * time.Second
	listenerMaxHeaderBytes    = 1 << 20
)

// Runtime owns the dedicated loopback-only HTTP listener used by database administration.
// It is intentionally separate from Flock's public site proxy and sharing listeners.
type Runtime struct {
	listener net.Listener
	server   *http.Server
	serveErr chan error
	closeOnce sync.Once
}

// StartRuntime binds an ephemeral TCP port on IPv4 loopback only. The returned host is the
// exact Host value handlers must validate; callers must not publish it through public proxies.
func StartRuntime(handler http.Handler) (*Runtime, error) {
	if handler == nil {
		return nil, errors.New("database admin handler is required")
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("listen for database admin: %w", err)
	}
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: listenerReadHeaderTimeout,
		ReadTimeout:       listenerReadTimeout,
		WriteTimeout:      listenerWriteTimeout,
		IdleTimeout:       listenerIdleTimeout,
		MaxHeaderBytes:    listenerMaxHeaderBytes,
	}
	runtime := &Runtime{
		listener: listener,
		server:   server,
		serveErr: make(chan error, 1),
	}
	go func() {
		err := server.Serve(listener)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		runtime.serveErr <- err
		close(runtime.serveErr)
	}()
	return runtime, nil
}

// Addr returns the bound loopback host:port. Use this exact value for Host and Origin checks.
func (r *Runtime) Addr() string {
	return r.listener.Addr().String()
}

// URL returns the local browser URL. It contains no credentials, tickets, or database identity.
func (r *Runtime) URL() string {
	return "http://" + r.Addr()
}

// Done reports the listener's termination. A nil value means it was shut down cleanly.
func (r *Runtime) Done() <-chan error {
	return r.serveErr
}

// Shutdown gracefully stops the listener, forcing connections closed if the context expires.
func (r *Runtime) Shutdown(ctx context.Context) error {
	var shutdownErr error
	r.closeOnce.Do(func() {
		shutdownErr = r.server.Shutdown(ctx)
		if shutdownErr != nil {
			_ = r.server.Close()
		}
	})
	return shutdownErr
}
