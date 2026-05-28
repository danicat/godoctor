package roots

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestState_Add_And_Get(t *testing.T) {
	state := &State{
		roots: make(map[*mcp.ServerSession][]string),
	}

	session := &mcp.ServerSession{}

	// Test adding a root
	state.Add(session, "test_root")
	roots := state.Get(session)
	if len(roots) != 1 {
		t.Fatalf("expected 1 root, got %d", len(roots))
	}

	absExpected, _ := filepath.Abs("test_root")
	if roots[0] != absExpected {
		t.Errorf("expected root %q, got %q", absExpected, roots[0])
	}

	// Test duplicates are ignored
	state.Add(session, "test_root")
	roots = state.Get(session)
	if len(roots) != 1 {
		t.Errorf("expected duplicates to be ignored, got %d roots", len(roots))
	}
}

func TestState_Validate(t *testing.T) {
	state := &State{
		roots: make(map[*mcp.ServerSession][]string),
	}

	session := &mcp.ServerSession{}

	// Fix macOS TempDir bug by using safe dummy absolute paths outside of os.TempDir()
	absRoot1 := "/test_workspace/project1"
	if filepath.Separator == '\\' {
		absRoot1 = "C:\\test_workspace\\project1"
	}

	// Add registered root
	state.Add(session, absRoot1)

	// Test 1: Valid subpath of a registered root
	validPath := filepath.Join(absRoot1, "src", "main.go")
	resolved, err := state.Validate(session, validPath)
	if err != nil {
		t.Errorf("expected path %q to be valid: %v", validPath, err)
	}
	absValid, _ := filepath.Abs(validPath)
	if resolved != absValid {
		t.Errorf("expected resolved path %q, got %q", absValid, resolved)
	}

	// Test 2: Invalid path outside registered roots
	invalidPath := "/test_workspace/project2/main.go"
	if filepath.Separator == '\\' {
		invalidPath = "C:\\test_workspace\\project2\\main.go"
	}
	_, err = state.Validate(session, invalidPath)
	if err == nil {
		t.Errorf("expected path %q to be rejected as outside registered roots", invalidPath)
	} else if !strings.Contains(err.Error(), "outside of registered workspace roots") {
		t.Errorf("unexpected error message: %v", err)
	}

	// Test 3: Temporary directory access
	tempFile := filepath.Join(os.TempDir(), "some_temp_file.go")
	resolvedTemp, err := state.Validate(session, tempFile)
	if err != nil {
		t.Errorf("expected temporary path %q to be allowed: %v", tempFile, err)
	}
	absTemp, _ := filepath.Abs(tempFile)
	if resolvedTemp != absTemp {
		t.Errorf("expected resolved temp path %q, got %q", absTemp, resolvedTemp)
	}
}

func TestState_Validate_NoRoots(t *testing.T) {
	// If no roots are registered, it should fallback to CWD (unless CWD is "/")
	state := &State{
		roots: make(map[*mcp.ServerSession][]string),
	}

	session := &mcp.ServerSession{}

	cwd, _ := filepath.Abs(".")
	if cwd == "/" {
		// Can't easily test the fallback inside a system root if we are already there,
		// but we can test that it denies access to paths outside CWD.
		t.Skip("skipping CWD fallback test because CWD is root")
	}

	// Default/empty path should resolve to CWD
	resolved, err := state.Validate(session, ".")
	if err != nil {
		t.Fatalf("expected '.' to be valid: %v", err)
	}
	if resolved != cwd {
		t.Errorf("expected '.' to resolve to CWD %q, got %q", cwd, resolved)
	}

	// Subpath of CWD should be allowed
	subPath := filepath.Join(cwd, "main.go")
	resolvedSub, err := state.Validate(session, subPath)
	if err != nil {
		t.Errorf("expected subpath %q to be allowed: %v", subPath, err)
	}
	absSub, _ := filepath.Abs(subPath)
	if resolvedSub != absSub {
		t.Errorf("expected resolved subpath %q, got %q", absSub, resolvedSub)
	}

	// Path outside CWD should be denied
	parentOfCwd := filepath.Dir(cwd)
	if parentOfCwd != cwd { // Ensure we are not at root
		outsidePath := filepath.Join(parentOfCwd, "outside_godoctor_cwd.go")
		_, err = state.Validate(session, outsidePath)
		if err == nil {
			t.Errorf("expected path %q outside CWD to be denied", outsidePath)
		}
	}
}

func TestState_Validate_MultiSession(t *testing.T) {
	state := &State{
		roots: make(map[*mcp.ServerSession][]string),
	}

	session1 := &mcp.ServerSession{}
	session2 := &mcp.ServerSession{}

	root1 := "/test_workspace/project1"
	root2 := "/test_workspace/project2"
	if filepath.Separator == '\\' {
		root1 = "C:\\test_workspace\\project1"
		root2 = "C:\\test_workspace\\project2"
	}

	// Register distinct roots to distinct sessions
	state.Add(session1, root1)
	state.Add(session2, root2)

	// Session 1 should access project 1 but not project 2
	path1 := filepath.Join(root1, "main.go")
	if _, err := state.Validate(session1, path1); err != nil {
		t.Errorf("session1 should access path inside project1: %v", err)
	}
	path2 := filepath.Join(root2, "main.go")
	if _, err := state.Validate(session1, path2); err == nil {
		t.Errorf("session1 should NOT access path inside project2")
	}

	// Session 2 should access project 2 but not project 1
	if _, err := state.Validate(session2, path2); err != nil {
		t.Errorf("session2 should access path inside project2: %v", err)
	}
	if _, err := state.Validate(session2, path1); err == nil {
		t.Errorf("session2 should NOT access path inside project1")
	}
}

func TestState_Validate_MultiRootCwd(t *testing.T) {
	state := &State{
		roots: make(map[*mcp.ServerSession][]string),
	}

	session := &mcp.ServerSession{}

	cwd, _ := filepath.Abs(".")
	dummyRoot := "/test_workspace/project1"
	if filepath.Separator == '\\' {
		dummyRoot = "C:\\test_workspace\\project1"
	}

	// Register dummy root first, and the actual CWD second
	state.Add(session, dummyRoot)
	state.Add(session, cwd)

	// Validate "." should resolve to CWD instead of defaulting to the first root (dummyRoot)
	resolved, err := state.Validate(session, ".")
	if err != nil {
		t.Fatalf("expected '.' to be valid: %v", err)
	}

	if resolved != cwd {
		t.Errorf("expected '.' to resolve to CWD %q, got %q", cwd, resolved)
	}
}

func TestState_Sync_NoCapabilities(t *testing.T) {
	state := &State{
		roots: make(map[*mcp.ServerSession][]string),
	}
	session := &mcp.ServerSession{} // Nil InitializeParams by default

	state.Sync(context.Background(), session)
	rts := state.Get(session)
	if len(rts) != 1 {
		t.Fatalf("expected 1 root, got %d", len(rts))
	}
	cwd, _ := filepath.Abs(".")
	if rts[0] != cwd {
		t.Errorf("expected root to be CWD %q, got %q", cwd, rts[0])
	}
}

func TestState_Sync_WithCapabilitiesAndPercentDecoding(t *testing.T) {
	ctx := context.Background()

	// 1. Create client that advertises roots capability
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "1.0.0"}, nil)

	var uri string
	var expectedPath string
	if filepath.Separator == '\\' {
		uri = "file:///C:/path/to/my%20workspace"
		expectedPath = "C:\\path\\to\\my workspace"
	} else {
		uri = "file:///path/to/my%20workspace"
		expectedPath = "/path/to/my workspace"
	}

	client.AddRoots(&mcp.Root{URI: uri})

	// 2. Create server
	server := mcp.NewServer(&mcp.Implementation{Name: "test-server", Version: "1.0.0"}, nil)

	t1, t2 := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, t1, nil)
	if err != nil {
		t.Fatalf("failed to connect server: %v", err)
	}
	defer func() { _ = serverSession.Close() }()

	clientSession, err := client.Connect(ctx, t2, nil)
	if err != nil {
		t.Fatalf("failed to connect client: %v", err)
	}
	defer func() { _ = clientSession.Close() }()

	// 3. Call roots state Sync manually
	state := &State{
		roots: make(map[*mcp.ServerSession][]string),
	}
	state.Sync(ctx, serverSession)

	rts := state.Get(serverSession)
	if len(rts) != 1 {
		t.Fatalf("expected 1 root, got %d", len(rts))
	}

	absExpected, _ := filepath.Abs(expectedPath)
	if rts[0] != absExpected {
		t.Errorf("expected resolved and normalized root %q, got %q", absExpected, rts[0])
	}
}

func TestState_Delete(t *testing.T) {
	state := &State{
		roots: make(map[*mcp.ServerSession][]string),
	}
	session := &mcp.ServerSession{}

	state.Add(session, "test_root")
	rts := state.Get(session)
	if len(rts) != 1 {
		t.Fatalf("expected 1 root, got %d", len(rts))
	}

	state.Delete(session)
	rts = state.Get(session)
	if len(rts) != 0 {
		t.Errorf("expected 0 roots after Delete, got %d", len(rts))
	}
}
