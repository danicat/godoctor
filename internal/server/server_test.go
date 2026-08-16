package server_test

import (
	"context"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/danicat/godoctor/internal/server"
)

func TestServer_RegisterHandlers_Idempotent(t *testing.T) {
	s := server.New("1.0.0-test")
	if s == nil {
		t.Fatal("New returned nil server")
	}

	// Sequential idempotency check
	for i := 0; i < 3; i++ {
		if err := s.RegisterHandlers(); err != nil {
			t.Fatalf("RegisterHandlers() iteration %d unexpected error = %v", i, err)
		}
	}

	// Concurrent idempotency check
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.RegisterHandlers(); err != nil {
				t.Errorf("concurrent RegisterHandlers() error = %v", err)
			}
		}()
	}
	wg.Wait()
}

func TestServer_Options(t *testing.T) {
	customInstr := "Custom instructions for testing"
	customOrigins := []string{"https://app.godoctor.dev"}

	s := server.New("2.0.0",
		server.WithInstructions(customInstr),
		server.WithAllowedOrigins(customOrigins),
	)
	if s == nil {
		t.Fatal("New with options returned nil server")
	}

	if err := s.RegisterHandlers(); err != nil {
		t.Fatalf("RegisterHandlers() unexpected error = %v", err)
	}
}

func TestServer_ServeHTTP_CORS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on free port: %v", err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()

	srv := server.New("test", server.WithAllowedOrigins([]string{"https://custom.allowed.corp"}))
	errCh := make(chan error, 1)

	go func() {
		errCh <- srv.ServeHTTP(ctx, addr)
	}()

	// Wait for server to bind with retry polling
	client := &http.Client{Timeout: 2 * time.Second}
	var ready bool
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		req, _ := http.NewRequestWithContext(ctx, http.MethodOptions, "http://"+addr+"/", nil)
		if resp, err := client.Do(req); err == nil {
			_ = resp.Body.Close()
			ready = true
			break
		}
	}
	if !ready {
		t.Fatal("timed out waiting for HTTP server to become ready")
	}

	tests := []struct {
		name           string
		origin         string
		expectedStatus int
		expectAllow    bool
	}{
		{
			name:           "localhost port 3000 allowed",
			origin:         "http://localhost:3000",
			expectedStatus: http.StatusOK,
			expectAllow:    true,
		},
		{
			name:           "127.0.0.1 port 8080 allowed",
			origin:         "http://127.0.0.1:8080",
			expectedStatus: http.StatusOK,
			expectAllow:    true,
		},
		{
			name:           "https localhost allowed",
			origin:         "https://localhost:5173",
			expectedStatus: http.StatusOK,
			expectAllow:    true,
		},
		{
			name:           "configured custom origin allowed",
			origin:         "https://custom.allowed.corp",
			expectedStatus: http.StatusOK,
			expectAllow:    true,
		},
		{
			name:           "attacker prefix subdomain rejected",
			origin:         "http://localhost.attacker.com",
			expectedStatus: http.StatusForbidden,
			expectAllow:    false,
		},
		{
			name:           "arbitrary untrusted https origin rejected",
			origin:         "https://evil-site.com",
			expectedStatus: http.StatusForbidden,
			expectAllow:    false,
		},
		{
			name:           "arbitrary untrusted http origin rejected",
			origin:         "http://attacker.com",
			expectedStatus: http.StatusForbidden,
			expectAllow:    false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(ctx, http.MethodOptions, "http://"+addr+"/", nil)
			if err != nil {
				t.Fatalf("failed to create OPTIONS request: %v", err)
			}
			req.Header.Set("Origin", tc.origin)

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.expectedStatus {
				t.Errorf("expected status %d, got %d", tc.expectedStatus, resp.StatusCode)
			}

			allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
			if tc.expectAllow {
				if allowOrigin != tc.origin {
					t.Errorf("expected Access-Control-Allow-Origin %q, got %q", tc.origin, allowOrigin)
				}
				if resp.Header.Get("Access-Control-Allow-Credentials") != "true" {
					t.Error("expected Access-Control-Allow-Credentials to be true for allowed origin")
				}
			} else {
				if allowOrigin != "" {
					t.Errorf("expected no Access-Control-Allow-Origin header for rejected origin, got %q", allowOrigin)
				}
			}
		})
	}

	// Trigger graceful shutdown
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

func TestServer_ServeHTTP_BindFailure(t *testing.T) {
	// Create a listener to hold a port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to create test listener: %v", err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	srv := server.New("test")
	ctx := context.Background()

	// ServeHTTP on the already bound address should fail immediately and not hang
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ServeHTTP(ctx, addr)
	}()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected ServeHTTP to return error when binding to used port, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ServeHTTP hung instead of returning error on bind failure")
	}
}
