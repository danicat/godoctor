package hooks

import (
	"testing"
)

func makePayload(toolName string, input map[string]interface{}) HookPayload {
	return HookPayload{ToolName: toolName, ToolInput: input}
}

func pathInput(path string) map[string]interface{} {
	return map[string]interface{}{"path": path}
}

func cmdInput(cmd string) map[string]interface{} {
	return map[string]interface{}{"command": cmd}
}

// assertDeny checks the response is a deny and fails the test with a helpful message if not.
func assertDeny(t *testing.T, resp HookResponse, label string) {
	t.Helper()
	if resp.Decision != "deny" {
		t.Errorf("%s: expected deny, got %q (reason: %s)", label, resp.Decision, resp.Reason)
	}
}

// assertAllow checks the response is an allow and fails the test with a helpful message if not.
func assertAllow(t *testing.T, resp HookResponse, label string) {
	t.Helper()
	if resp.Decision != "allow" {
		t.Errorf("%s: expected allow, got %q (reason: %s)", label, resp.Decision, resp.Reason)
	}
}

// ---------------------------------------------------------------------------
// replace tool
// ---------------------------------------------------------------------------

func TestReplace_GoFile_Deny(t *testing.T) {
	assertDeny(t, evaluate(makePayload("replace", pathInput("main.go"))), "replace main.go")
	assertDeny(t, evaluate(makePayload("replace", pathInput("internal/server/handler.go"))), "replace subdir .go")
}

func TestReplace_NonGoFile_Allow(t *testing.T) {
	assertAllow(t, evaluate(makePayload("replace", pathInput("script.py"))), "replace .py")
	assertAllow(t, evaluate(makePayload("replace", pathInput("README.md"))), "replace .md")
	assertAllow(t, evaluate(makePayload("replace", pathInput("config.yaml"))), "replace .yaml")
	assertAllow(t, evaluate(makePayload("replace", pathInput("Dockerfile"))), "replace Dockerfile")
}

func TestReplace_NoPath_Allow(t *testing.T) {
	assertAllow(t, evaluate(makePayload("replace", map[string]interface{}{})), "replace no path")
}

// ---------------------------------------------------------------------------
// read_file tool
// ---------------------------------------------------------------------------

func TestReadFile_GoFile_Deny(t *testing.T) {
	assertDeny(t, evaluate(makePayload("read_file", pathInput("cmd/godoctor/main.go"))), "read_file main.go")
	assertDeny(t, evaluate(makePayload("read_file", pathInput("types.go"))), "read_file types.go")
}

func TestReadFile_NonGoFile_Allow(t *testing.T) {
	assertAllow(t, evaluate(makePayload("read_file", pathInput("README.md"))), "read_file .md")
	assertAllow(t, evaluate(makePayload("read_file", pathInput("package.json"))), "read_file .json")
	assertAllow(t, evaluate(makePayload("read_file", pathInput("index.ts"))), "read_file .ts")
	assertAllow(t, evaluate(makePayload("read_file", pathInput(".env"))), "read_file .env")
}

func TestReadFile_NoPath_Allow(t *testing.T) {
	assertAllow(t, evaluate(makePayload("read_file", map[string]interface{}{})), "read_file no path")
}

// ---------------------------------------------------------------------------
// write_file tool
// ---------------------------------------------------------------------------

func TestWriteFile_GoFile_Deny(t *testing.T) {
	assertDeny(t, evaluate(makePayload("write_file", pathInput("new_service.go"))), "write_file .go")
	assertDeny(t, evaluate(makePayload("write_file", pathInput("pkg/util/helper.go"))), "write_file subdir .go")
}

func TestWriteFile_NonGoFile_Allow(t *testing.T) {
	assertAllow(t, evaluate(makePayload("write_file", pathInput("index.html"))), "write_file .html")
	assertAllow(t, evaluate(makePayload("write_file", pathInput("styles.css"))), "write_file .css")
	assertAllow(t, evaluate(makePayload("write_file", pathInput("main.py"))), "write_file .py")
}

func TestWriteFile_NoPath_Allow(t *testing.T) {
	assertAllow(t, evaluate(makePayload("write_file", map[string]interface{}{})), "write_file no path")
}

// ---------------------------------------------------------------------------
// run_shell_command — build/test (always blocked)
// ---------------------------------------------------------------------------

func TestShell_GoBuild_Deny(t *testing.T) {
	assertDeny(t, evaluate(makePayload("run_shell_command", cmdInput("go build ./..."))), "go build ./...")
	assertDeny(t, evaluate(makePayload("run_shell_command", cmdInput("go build -o bin/app ."))), "go build -o")
}

func TestShell_GoTest_Deny(t *testing.T) {
	assertDeny(t, evaluate(makePayload("run_shell_command", cmdInput("go test ./..."))), "go test ./...")
	assertDeny(t, evaluate(makePayload("run_shell_command", cmdInput("go test -v -run TestFoo ./pkg/..."))), "go test -v")
}

func TestShell_GoVet_Deny(t *testing.T) {
	assertDeny(t, evaluate(makePayload("run_shell_command", cmdInput("go vet ./..."))), "go vet")
}

func TestShell_GolangciLint_Deny(t *testing.T) {
	assertDeny(t, evaluate(makePayload("run_shell_command", cmdInput("golangci-lint run"))), "golangci-lint run")
}

func TestShell_GoGet_Deny(t *testing.T) {
	assertDeny(t, evaluate(makePayload("run_shell_command", cmdInput("go get github.com/pkg/errors"))), "go get")
	assertDeny(t, evaluate(makePayload("run_shell_command", cmdInput("cd myapp && go get ./..."))), "inline go get")
}

// ---------------------------------------------------------------------------
// run_shell_command — file writers on .go files (blocked)
// ---------------------------------------------------------------------------

func TestShell_SedOnGoFile_Deny(t *testing.T) {
	assertDeny(t, evaluate(makePayload("run_shell_command", cmdInput("sed -i 's/foo/bar/g' main.go"))), "sed -i .go")
}

func TestShell_TeeOnGoFile_Deny(t *testing.T) {
	assertDeny(t, evaluate(makePayload("run_shell_command", cmdInput("echo 'package main' | tee main.go"))), "tee .go")
}

func TestShell_EchoRedirectToGoFile_Deny(t *testing.T) {
	assertDeny(t, evaluate(makePayload("run_shell_command", cmdInput("echo 'package main' > main.go"))), "echo > .go")
	assertDeny(t, evaluate(makePayload("run_shell_command", cmdInput("echo 'package main' >> main.go"))), "echo >> .go")
}

// ---------------------------------------------------------------------------
// run_shell_command — file writers on non-.go files (allowed)
// ---------------------------------------------------------------------------

func TestShell_SedOnNonGoFile_Allow(t *testing.T) {
	assertAllow(t, evaluate(makePayload("run_shell_command", cmdInput("sed -i 's/foo/bar/g' config.yaml"))), "sed -i .yaml")
	assertAllow(t, evaluate(makePayload("run_shell_command", cmdInput("sed -i 's/v1/v2/' Dockerfile"))), "sed -i Dockerfile")
}

func TestShell_EchoRedirectToNonGoFile_Allow(t *testing.T) {
	assertAllow(t, evaluate(makePayload("run_shell_command", cmdInput("echo 'hello' > output.txt"))), "echo > .txt")
}

func TestShell_EchoToDevNull_Allow(t *testing.T) {
	assertAllow(t, evaluate(makePayload("run_shell_command", cmdInput("echo 'test' > /dev/null"))), "echo > /dev/null")
}

// ---------------------------------------------------------------------------
// run_shell_command — file readers on .go files (blocked)
// ---------------------------------------------------------------------------

func TestShell_CatGoFile_Deny(t *testing.T) {
	assertDeny(t, evaluate(makePayload("run_shell_command", cmdInput("cat main.go"))), "cat main.go")
	assertDeny(t, evaluate(makePayload("run_shell_command", cmdInput("cat internal/hooks/intercept.go"))), "cat subdir .go")
}

func TestShell_GrepGoFile_Deny(t *testing.T) {
	assertDeny(t, evaluate(makePayload("run_shell_command", cmdInput("grep 'func ' main.go"))), "grep .go")
}

// ---------------------------------------------------------------------------
// run_shell_command — file readers on .go files with pipes (allowed)
// ---------------------------------------------------------------------------

func TestShell_GrepGoFileWithPipe_Allow(t *testing.T) {
	assertAllow(t, evaluate(makePayload("run_shell_command", cmdInput("grep 'TODO' main.go | wc -l"))), "grep .go | pipe")
}

// ---------------------------------------------------------------------------
// run_shell_command — file readers on non-.go files (allowed)
// ---------------------------------------------------------------------------

func TestShell_CatNonGoFile_Allow(t *testing.T) {
	assertAllow(t, evaluate(makePayload("run_shell_command", cmdInput("cat README.md"))), "cat .md")
	assertAllow(t, evaluate(makePayload("run_shell_command", cmdInput("cat package.json"))), "cat .json")
}

func TestShell_GrepNonGoFile_Allow(t *testing.T) {
	assertAllow(t, evaluate(makePayload("run_shell_command", cmdInput("grep 'error' server.log"))), "grep .log")
}

// ---------------------------------------------------------------------------
// run_shell_command — safe commands (always allowed)
// ---------------------------------------------------------------------------

func TestShell_SafeCommands_Allow(t *testing.T) {
	cases := []struct {
		label string
		cmd   string
	}{
		{"npm install", "npm install"},
		{"pip install", "pip install requests"},
		{"ls", "ls -la"},
		{"mkdir", "mkdir -p dist"},
		{"git status", "git status"},
		{"curl", "curl https://example.com"},
		{"docker build", "docker build -t myapp ."},
		{"make", "make build"},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			assertAllow(t, evaluate(makePayload("run_shell_command", cmdInput(tc.cmd))), tc.label)
		})
	}
}

// ---------------------------------------------------------------------------
// Unknown tools (always allowed)
// ---------------------------------------------------------------------------

func TestUnknownTool_Allow(t *testing.T) {
	assertAllow(t, evaluate(makePayload("search_web", map[string]interface{}{"query": "golang"})), "search_web")
	assertAllow(t, evaluate(makePayload("list_dir", map[string]interface{}{"path": "/tmp"})), "list_dir")
	assertAllow(t, evaluate(makePayload("", map[string]interface{}{})), "empty tool name")
}
