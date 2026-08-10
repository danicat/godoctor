package server_test

import (
	"testing"

	"github.com/danicat/godoctor/internal/server"
)

func TestServer_RegisterHandlers(t *testing.T) {
	s := server.New("test")
	err := s.RegisterHandlers()
	if err != nil {
		t.Fatalf("RegisterHandlers() unexpected error = %v", err)
	}
}
