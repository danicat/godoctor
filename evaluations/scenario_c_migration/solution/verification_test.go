package main

import (
	"bytes"
	"os"
	"testing"
)

func TestMigration(t *testing.T) {
	// 1. Check if imports contain "io/ioutil"
	content, _ := os.ReadFile("main.go")
	if bytes.Contains(content, []byte("\"io/ioutil\"")) {
		t.Error("main.go still imports io/ioutil")
	}
	
	contentUtils, _ := os.ReadFile("utils.go")
	if bytes.Contains(contentUtils, []byte("\"io/ioutil\"")) {
		t.Error("utils.go still imports io/ioutil")
	}
	
	// 2. Functional check
	// We can verify if the new code compiles (go test does that)
}

