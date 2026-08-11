package server_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/danicat/godoctor/internal/server"
)

func TestServer_RegisterHandlers(t *testing.T) {
	s := server.New("1.0.0-test")
	if s == nil {
		t.Fatal("New returned nil server")
	}

	err := s.RegisterHandlers()
	if err != nil {
		t.Fatalf("RegisterHandlers() unexpected error = %v", err)
	}
}

func TestServer_ServeHTTP_CORSAndShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on free port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	srv := server.New("test")
	errCh := make(chan error, 1)

	go func() {
		errCh <- srv.ServeHTTP(ctx, addr)
	}()

	// Wait for server to bind
	time.Sleep(100 * time.Millisecond)

	// Test OPTIONS CORS preflight
	req, err := http.NewRequestWithContext(ctx, http.MethodOptions, "http://"+addr+"/", nil)
	if err != nil {
		t.Fatalf("failed to create CORS request: %v", err)
	}
	req.Header.Set("Origin", "http://localhost:3000")

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("CORS request failed: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK for OPTIONS CORS request, got %d", resp.StatusCode)
	}

	allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	if allowOrigin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin to be 'http://localhost:3000', got %q", allowOrigin)
	}

	// Trigger shutdown via context cancellation
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("ServeHTTP returned unexpected error on shutdown: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for HTTP server to shut down")
	}
}
