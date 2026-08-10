package roots

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
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
	if slices.Contains(rts, abs) {
		return
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

// parseRootURI parses a single MCP root URI and normalizes it to a local absolute path.
func parseRootURI(uri string) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return ""
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
		return abs
	}
	return ""
}

// parseRoots takes MCP ListRootsResult and extracts valid absolute paths.
func parseRoots(res *mcp.ListRootsResult) []string {
	var rts []string
	for _, r := range res.Roots {
		if abs := parseRootURI(r.URI); abs != "" {
			rts = append(rts, abs)
		}
	}
	if len(rts) == 0 {
		abs, _ := filepath.Abs(".")
		rts = append(rts, abs)
	}
	return rts
}

// Sync synchronizes roots from the MCP client for the given session.
func (s *State) Sync(ctx context.Context, session *mcp.ServerSession) {
	if session == nil {
		s.Add(nil, ".")
		return
	}

	initParams := session.InitializeParams()
	if initParams == nil || initParams.Capabilities == nil {
		s.Add(session, ".")
		return
	}

	res, err := session.ListRoots(ctx, &mcp.ListRootsParams{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to list roots from client: %v. Falling back to CWD.\n", err)
		s.Add(session, ".")
		return
	}

	rts := parseRoots(res)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.roots == nil {
		s.roots = make(map[*mcp.ServerSession][]string)
	}
	s.roots[session] = rts
}

// Validate checks if the given path is within any of the registered roots for the session.
// It returns the absolute path if valid, or an error if not.
// validateEmptyPath handles the case where path is empty or ".".
func (s *State) validateEmptyPath(session *mcp.ServerSession) (string, error) {
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

// isTempDir checks if absPath is inside the system temporary directory.
func isTempDir(absPath string) bool {
	rawTemp := os.TempDir()
	if strings.HasPrefix(absPath, rawTemp) {
		return true
	}
	if tempDir, err := filepath.EvalSymlinks(rawTemp); err == nil {
		if strings.HasPrefix(absPath, tempDir) || strings.HasPrefix(absPath, "/tmp") {
			return true
		}
	}
	return false
}

// validateCWD checks if the given path is allowed when no roots are registered.
func validateCWD(absPath string, originalPath string) (string, error) {
	cwd, _ := filepath.Abs(".")
	if cwd == "/" || cwd == filepath.VolumeName(cwd)+string(filepath.Separator) {
		return "", fmt.Errorf("access denied: current working directory is the system root")
	}
	if absPath == cwd || strings.HasPrefix(absPath, cwd+string(filepath.Separator)) {
		return absPath, nil
	}
	return "", fmt.Errorf("access denied: path %s is outside the current working directory", originalPath)
}

// Validate checks if the given path is within any of the registered roots for the session.
// It returns the absolute path if valid, or an error if not.
func (s *State) Validate(session *mcp.ServerSession, projectDir string) (string, error) {
	if projectDir == "" || projectDir == "." {
		workspaceDir, err := s.validateEmptyPath(session)
		if err == nil {
			s.Add(session, workspaceDir)
		}
		return workspaceDir, err
	}

	workspaceDir, err := filepath.Abs(projectDir)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	if isTempDir(workspaceDir) {
		return workspaceDir, nil
	}

	roots := s.Get(session)
	if len(roots) == 0 {
		validatedDir, err := validateCWD(workspaceDir, projectDir)
		if err == nil {
			s.Add(session, validatedDir)
		}
		return validatedDir, err
	}

	for _, root := range roots {
		if workspaceDir == root || strings.HasPrefix(workspaceDir, root+string(filepath.Separator)) {
			// Ensure the validated projectDir is registered as an MCP root
			s.Add(session, workspaceDir)
			return workspaceDir, nil
		}
	}

	return "", fmt.Errorf("access denied: path %s is outside of registered workspace roots", projectDir)
}
