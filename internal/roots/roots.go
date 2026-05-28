package roots

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// State manages the registered project roots on a per-session basis.
type State struct {
	mu    sync.RWMutex
	roots map[*mcp.ServerSession][]string
}

// Global is the singleton instance for the entire application.
var Global = &State{
	roots: make(map[*mcp.ServerSession][]string),
}

// Add adds a new project root for the given session after normalizing it to an absolute path.
func (s *State) Add(session *mcp.ServerSession, path string) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.roots == nil {
		s.roots = make(map[*mcp.ServerSession][]string)
	}

	rts := s.roots[session]
	for _, r := range rts {
		if r == abs {
			return
		}
	}
	s.roots[session] = append(rts, abs)
}

// Get returns a copy of the registered roots for the given session.
func (s *State) Get(session *mcp.ServerSession) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.roots == nil {
		return nil
	}

	rts, exists := s.roots[session]
	if !exists {
		return nil
	}

	rootsCopy := make([]string, len(rts))
	copy(rootsCopy, rts)
	return rootsCopy
}

// Delete removes all registered roots for the given session.
func (s *State) Delete(session *mcp.ServerSession) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.roots != nil {
		delete(s.roots, session)
	}
}

// Sync synchronizes roots from the MCP client for the given session.
func (s *State) Sync(ctx context.Context, session *mcp.ServerSession) {
	if session == nil {
		s.Add(nil, ".")
		return
	}

	initParams := session.InitializeParams()
	if initParams == nil || initParams.Capabilities == nil || initParams.Capabilities.RootsV2 == nil {
		s.Add(session, ".")
		return
	}

	res, err := session.ListRoots(ctx, &mcp.ListRootsParams{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to list roots from client: %v. Falling back to CWD.\n", err)
		s.Add(session, ".")
		return
	}

	var rts []string
	for _, r := range res.Roots {
		u, err := url.Parse(r.URI)
		if err != nil || u.Scheme != "file" {
			continue
		}

		path := u.Path
		if u.Host != "" && u.Host != "localhost" {
			if filepath.Separator == '\\' {
				// Windows UNC path: \\host\path
				path = `\\` + u.Host + filepath.FromSlash(u.Path)
			} else {
				// Treat host as part of the path if u.Path is empty/relative
				if path == "" {
					path = u.Host
				} else {
					path = u.Host + path
				}
			}
		}

		// Normalize Windows-style absolute paths from URIs (e.g. /C:/path -> C:/path)
		if filepath.Separator == '\\' && len(path) > 2 && path[0] == '/' && path[2] == ':' {
			path = path[1:]
		}

		abs, err := filepath.Abs(path)
		if err == nil {
			rts = append(rts, abs)
		}
	}

	if len(rts) == 0 {
		abs, _ := filepath.Abs(".")
		rts = append(rts, abs)
	}

	s.mu.Lock()
	if s.roots == nil {
		s.roots = make(map[*mcp.ServerSession][]string)
	}
	s.roots[session] = rts
	s.mu.Unlock()
}

// Validate checks if the given path is within any of the registered roots for the session.
// It returns the absolute path if valid, or an error if not.
func (s *State) Validate(session *mcp.ServerSession, path string) (string, error) {
	if path == "" || path == "." {
		roots := s.Get(session)
		cwd, err := filepath.Abs(".")
		if err == nil && cwd != "/" && cwd != filepath.VolumeName(cwd)+string(filepath.Separator) {
			for _, root := range roots {
				if cwd == root || strings.HasPrefix(cwd, root+string(filepath.Separator)) {
					return cwd, nil
				}
			}
		}
		if len(roots) > 0 {
			return roots[0], nil
		}
		if err != nil {
			return "", fmt.Errorf("invalid path: %w", err)
		}
		if cwd == "/" || cwd == filepath.VolumeName(cwd)+string(filepath.Separator) {
			return "", fmt.Errorf("access denied: current working directory is the system root")
		}
		return cwd, nil
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	roots := s.Get(session)

	// Allow access to system temporary directory
	rawTemp := os.TempDir()
	if strings.HasPrefix(absPath, rawTemp) {
		return absPath, nil
	}
	if tempDir, err := filepath.EvalSymlinks(rawTemp); err == nil {
		// handle /var/folders/ vs /tmp mismatch on macOS
		if strings.HasPrefix(absPath, tempDir) || strings.HasPrefix(absPath, "/tmp") {
			return absPath, nil
		}
	}

	// If no roots are registered, default to CWD (unless it is the system root)
	if len(roots) == 0 {
		cwd, _ := filepath.Abs(".")
		if cwd == "/" || cwd == filepath.VolumeName(cwd)+string(filepath.Separator) {
			return "", fmt.Errorf("access denied: current working directory is the system root")
		}
		if absPath == cwd || strings.HasPrefix(absPath, cwd+string(filepath.Separator)) {
			return absPath, nil
		}
		return "", fmt.Errorf("access denied: path %s is outside the current working directory", path)
	}

	for _, root := range roots {
		if absPath == root || strings.HasPrefix(absPath, root+string(filepath.Separator)) {
			return absPath, nil
		}
	}

	return "", fmt.Errorf("access denied: path %s is outside of registered workspace roots", path)
}
