//nolint:testpackage // Testing internal functions requires same package
package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/sentiolabs/envctl/internal/env"
)

func TestWriteEnvFormat(t *testing.T) {
	entries := []env.Entry{
		{Key: "SIMPLE", Value: "value", Source: "test"},
		{Key: "WITH_SPACES", Value: "hello world", Source: "test"},
		{Key: "WITH_QUOTES", Value: `say "hello"`, Source: "test"},
		{Key: "WITH_NEWLINE", Value: "line1\nline2", Source: "test"},
		{Key: "EMPTY", Value: "", Source: "test"},
	}

	var buf bytes.Buffer
	if err := Write(&buf, entries, FormatEnv); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	output := buf.String()

	// Check output contains expected values
	expectedLines := []string{
		"SIMPLE=value",
		`WITH_SPACES="hello world"`,
		`WITH_QUOTES="say \"hello\""`,
		`WITH_NEWLINE="line1\nline2"`,
		`EMPTY=""`,
	}

	for _, expected := range expectedLines {
		if !strings.Contains(output, expected) {
			t.Errorf("output missing %q\ngot:\n%s", expected, output)
		}
	}
}

func TestWriteShellFormat(t *testing.T) {
	entries := []env.Entry{
		{Key: "SIMPLE", Value: "value", Source: "test"},
		{Key: "WITH_DOLLAR", Value: "$HOME/path", Source: "test"},
	}

	var buf bytes.Buffer
	if err := Write(&buf, entries, FormatShell); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	output := buf.String()

	// Check shell format
	if !strings.Contains(output, `export SIMPLE="value"`) {
		t.Errorf("output missing export SIMPLE\ngot:\n%s", output)
	}

	if !strings.Contains(output, `export WITH_DOLLAR="\$HOME/path"`) {
		t.Errorf("output missing escaped dollar sign\ngot:\n%s", output)
	}
}

func TestWriteJSONFormat(t *testing.T) {
	entries := []env.Entry{
		{Key: "KEY1", Value: "value1", Source: "test"},
		{Key: "KEY2", Value: "value2", Source: "test"},
	}

	var buf bytes.Buffer
	if err := Write(&buf, entries, FormatJSON); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Verify valid JSON
	var result map[string]string
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\ngot:\n%s", err, buf.String())
	}

	if result["KEY1"] != "value1" {
		t.Errorf("JSON KEY1 = %q, want %q", result["KEY1"], "value1")
	}
	if result["KEY2"] != "value2" {
		t.Errorf("JSON KEY2 = %q, want %q", result["KEY2"], "value2")
	}
}

func TestWriteList(t *testing.T) {
	entries := []env.Entry{
		{Key: "DATABASE_URL", Value: "secret", Source: "myapp/dev"},
		{Key: "API_KEY", Value: "secret", Source: "shared/keys"},
	}

	// Test verbose output
	var buf bytes.Buffer
	if err := WriteList(&buf, entries, false); err != nil {
		t.Fatalf("WriteList() error: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "DATABASE_URL") || !strings.Contains(output, "myapp/dev") {
		t.Errorf("verbose output missing key/source info\ngot:\n%s", output)
	}

	// Test quiet output
	buf.Reset()
	if err := WriteList(&buf, entries, true); err != nil {
		t.Fatalf("WriteList() quiet error: %v", err)
	}

	output = buf.String()
	if !strings.Contains(output, "DATABASE_URL") {
		t.Errorf("quiet output missing key\ngot:\n%s", output)
	}
	if strings.Contains(output, "myapp/dev") {
		t.Errorf("quiet output should not contain source\ngot:\n%s", output)
	}
}

func TestParseFormat(t *testing.T) {
	tests := []struct {
		input   string
		want    Format
		wantErr bool
	}{
		{"env", FormatEnv, false},
		{"ENV", FormatEnv, false},
		{"", FormatEnv, false},
		{"shell", FormatShell, false},
		{"SHELL", FormatShell, false},
		{"json", FormatJSON, false},
		{"JSON", FormatJSON, false},
		{"invalid", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseFormat(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("ParseFormat() expected error")
				}
				return
			}
			if err != nil {
				t.Errorf("ParseFormat() error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("ParseFormat() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSortEntries(t *testing.T) {
	entries := []env.Entry{
		{Key: "ZEBRA", Value: "z", Source: "test"},
		{Key: "APPLE", Value: "a", Source: "test"},
		{Key: "MANGO", Value: "m", Source: "test"},
		{Key: "BANANA", Value: "b", Source: "test"},
	}

	sorted := sortEntries(entries)

	// Verify sorted order
	expectedOrder := []string{"APPLE", "BANANA", "MANGO", "ZEBRA"}
	for i, key := range expectedOrder {
		if sorted[i].Key != key {
			t.Errorf("sortEntries()[%d].Key = %q, want %q", i, sorted[i].Key, key)
		}
	}

	// Verify original slice is not modified
	if entries[0].Key != "ZEBRA" {
		t.Error("sortEntries() modified the original slice")
	}
}

func TestSortEntriesEmpty(t *testing.T) {
	sorted := sortEntries(nil)
	if len(sorted) != 0 {
		t.Errorf("sortEntries(nil) returned %d entries, want 0", len(sorted))
	}

	sorted = sortEntries([]env.Entry{})
	if len(sorted) != 0 {
		t.Errorf("sortEntries([]) returned %d entries, want 0", len(sorted))
	}
}

func TestQuoteValue(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"with space", `"with space"`},
		{`with "quotes"`, `"with \"quotes\""`},
		{"with\nnewline", `"with\nnewline"`},
		{"", `""`},
		{"with=equals", `"with=equals"`},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := quoteValue(tt.input); got != tt.want {
				t.Errorf("quoteValue(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
