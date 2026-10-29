package dotenv

import (
	"strings"
	"testing"
)

func TestParseBasic(t *testing.T) {
	input := `
# A comment
PORT=8080
export HOST="localhost"
MESSAGE='Hello World'
EMPTY=
INLINE=value # inline comment
WIN_PATH="C:\\new"
`
	vars, err := ParseReader(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]string{
		"PORT":     "8080",
		"HOST":     "localhost",
		"MESSAGE":  "Hello World",
		"EMPTY":    "",
		"INLINE":   "value",
		"WIN_PATH": "C:\\new",
	}

	for k, want := range expected {
		got, ok := vars[k]
		if !ok {
			t.Errorf("missing key %q", k)
		} else if got != want {
			t.Errorf("key %q: got %q, want %q", k, got, want)
		}
	}
}

func TestUnescape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`"hello\nworld"`, "hello\nworld"},
		{`"tab\there"`, "tab\there"},
		{`"C:\\new"`, "C:\\new"},
		{`"quote\"inside"`, "quote\"inside"},
	}

	for _, tt := range tests {
		line := "KEY=" + tt.input
		vars, err := ParseReader(strings.NewReader(line))
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}
		if got := vars["KEY"]; got != tt.want {
			t.Errorf("for %s: got %q, want %q", tt.input, got, tt.want)
		}
	}
}
