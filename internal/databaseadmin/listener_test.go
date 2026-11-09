package databaseadmin

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestStartRuntimeBindsLoopbackAndServesHandler(t *testing.T) {
	runtime, err := StartRuntime(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || net.ParseIP(peer) == nil || !net.ParseIP(peer).IsLoopback() {
			t.Errorf("request peer is not loopback: %q", r.RemoteAddr)
		}
		_, _ = io.WriteString(w, "ready")
	}))
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Shutdown(context.Background())

	host, _, err := net.SplitHostPort(runtime.Addr())
	if err != nil {
		t.Fatal(err)
	}
	if ip := net.ParseIP(host); ip == nil || !ip.IsLoopback() {
		t.Fatalf("listener is not loopback-only: %q", runtime.Addr())
	}
	if !strings.HasPrefix(runtime.URL(), "http://127.0.0.1:") {
		t.Fatalf("unexpected runtime URL: %q", runtime.URL())
	}

	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(runtime.URL())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || string(body) != "ready" {
		t.Fatalf("got status %d and body %q", response.StatusCode, body)
	}
}

func TestStartRuntimeRejectsNilHandler(t *testing.T) {
	if _, err := StartRuntime(nil); err == nil {
		t.Fatal("expected nil handler to be rejected")
	}
}

func TestRuntimeShutdownClosesListener(t *testing.T) {
	runtime, err := StartRuntime(http.NotFoundHandler())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := runtime.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-runtime.Done():
		if err != nil {
			t.Fatalf("serve returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("runtime did not stop")
	}
}
