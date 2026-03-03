package main

import (
	"os"
	"strings"
	"testing"
)

func TestGoModDeclaresGo126(t *testing.T) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("failed to read go.mod: %v", err)
	}

	lines := strings.Split(string(data), "\n")
	var goDirective string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "go ") && !strings.HasPrefix(trimmed, "go.") {
			goDirective = trimmed
			break
		}
	}

	if goDirective == "" {
		t.Fatal("no go directive found in go.mod")
	}

	expected := "go 1.26"
	if goDirective != expected {
		t.Errorf("go.mod go directive = %q, want %q", goDirective, expected)
	}
}
