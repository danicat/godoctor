// Package lsp implements a stateful, persistent JSON-RPC Language Server Protocol (LSP) client.
package lsp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os/exec"
	"sync"

	"github.com/danicat/godoctor/internal/roots"
	"github.com/danicat/godoctor/internal/safeshell"
)

// Manager coordinates the lifecycle of the single, persistent gopls language server process.
type Manager struct {
	cmd    *exec.Cmd
	client *Client
	mu     sync.Mutex
	closed bool
}

// GlobalManager is the shared, package-level LSP lifecycle controller.
var GlobalManager = &Manager{}

// Client retrieves the active, initialized LSP client.
func (m *Manager) Client(ctx context.Context) (*Client, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, fmt.Errorf("lsp manager is closed")
	}

	if m.client != nil {
		return m.client, nil
	}

	// 1. Launch gopls as a daemon serving over stdin/stdout
	cmd, err := safeshell.CommandContext(ctx, "gopls", "serve")
	if err != nil {
		return nil, err
	}

	// Create pipe endpoints for reliable cross-talk
	srvConn, cliConn := net.Pipe()

	// Intercept standard streams and pipe them into the connection
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		_ = srvConn.Close()
		_ = cliConn.Close()
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		_ = srvConn.Close()
		_ = cliConn.Close()
		return nil, err
	}

	// Proxy loops
	go func() {
		_, _ = ioCopy(stdinPipe, srvConn)
		_ = stdinPipe.Close()
	}()
	go func() {
		_, _ = ioCopy(srvConn, stdoutPipe)
		_ = srvConn.Close()
	}()

	if err := cmd.Start(); err != nil {
		_ = cliConn.Close()
		return nil, err
	}

	m.cmd = cmd
	m.client = NewClientWithConn(cliConn)

	// Perform connection initialization handshake
	if err := m.client.Initialize(ctx, roots.Global.GetAllRoots()); err != nil {
		_ = m.client.Close(ctx)
		m.client = nil
		return nil, err
	}

	return m.client, nil
}

// Reset terminates the active connection and process (if any) and clears the client
// so that a fresh, healthy language server is spawned on the next query.
func (m *Manager) Reset(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}

	if m.client != nil {
		_ = m.client.Close(ctx)
	}
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
		_, _ = m.cmd.Process.Wait()
	}
	m.client = nil
	return nil
}

// Close terminates the gopls process and cleans up connection resources.
func (m *Manager) Close(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true
	if m.client == nil {
		return nil
	}

	_ = m.client.Close(ctx)
	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
		_, _ = m.cmd.Process.Wait()
	}
	m.client = nil
	return nil
}

// Manual mock wrapper to avoid importing heavy "io" dependency details unless needed
func ioCopy(dst writeCloser, src readCloser) (int64, error) {
	buf := make([]byte, 32*1024)
	var written int64
	var err error
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			nw, ew := dst.Write(buf[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = fmt.Errorf("invalid write result")
				}
			}
			written += int64(nw)
			if ew != nil {
				err = ew
				break
			}
			if nr != nw {
				err = fmt.Errorf("short write")
				break
			}
		}
		if er != nil {
			if !errors.Is(er, io.EOF) {
				err = er
			}
			break
		}
	}
	return written, err
}

type writeCloser interface {
	Write(p []byte) (n int, err error)
}

type readCloser interface {
	Read(p []byte) (n int, err error)
}
