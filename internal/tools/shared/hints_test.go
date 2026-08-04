package shared_test

import (
	"strings"
	"testing"

	"github.com/danicat/godoctor/internal/tools/shared"
)

func TestGetDocHintFromOutput(t *testing.T) {
	output := "package github.com/test/pkg is not in GOROOT"
	hint := shared.GetDocHintFromOutput(output)
	if !strings.Contains(hint, "github.com/test/pkg") {
		t.Errorf("GetDocHintFromOutput() expected hint, got %q", hint)
	}

	undefinedOut := "undefined: http.Get"
	undefinedHint := shared.GetDocHintFromOutput(undefinedOut)
	if !strings.Contains(undefinedHint, "http") {
		t.Errorf("GetDocHintFromOutput() expected http hint, got %q", undefinedHint)
	}
}
