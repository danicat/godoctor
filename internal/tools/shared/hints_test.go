package shared_test

import (
	"strings"
	"testing"

	"github.com/danicat/godoctor/internal/tools/shared"
	"golang.org/x/tools/go/packages"
)

func TestGetDocHint(t *testing.T) {
	errs := []packages.Error{
		{Msg: "undefined: http.Get"},
	}
	hint := shared.GetDocHint(errs)
	if !strings.Contains(hint, "http") {
		t.Errorf("GetDocHint() expected package hint, got %q", hint)
	}

	importErrs := []packages.Error{
		{Msg: "could not import github.com/foo/bar"},
	}
	importHint := shared.GetDocHint(importErrs)
	if !strings.Contains(importHint, "github.com/foo/bar") {
		t.Errorf("GetDocHint() expected import hint, got %q", importHint)
	}
}

func TestGetDocHintFromOutput(t *testing.T) {
	output := "package github.com/test/pkg is not in GOROOT"
	hint := shared.GetDocHintFromOutput(output)
	if !strings.Contains(hint, "github.com/test/pkg") {
		t.Errorf("GetDocHintFromOutput() expected hint, got %q", hint)
	}
}

func TestCleanError(t *testing.T) {
	msg := `invalid package name: ""`
	cleaned := shared.CleanError(msg)
	if strings.Contains(cleaned, `""`) {
		t.Errorf("CleanError() failed to clean quotes, got %q", cleaned)
	}
}
