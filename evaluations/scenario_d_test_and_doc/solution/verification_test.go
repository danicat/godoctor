package main

import (
	"bytes"
	"os"
	"testing"
)

func TestDocsExist(t *testing.T) {
	// Check if risk.go has doc comments
	content, _ := os.ReadFile("risk.go")
	if !bytes.Contains(content, []byte("// CalculateRiskScore")) {
		t.Error("Documentation missing for CalculateRiskScore")
	}
	
	// Check if risk_test.go exists
	if _, err := os.Stat("risk_test.go"); os.IsNotExist(err) {
		t.Error("risk_test.go not created")
	}
}