package schema

import (
	"strings"
	"testing"
)

func TestValidateValidSchema(t *testing.T) {
	yml := `
version: "1"
vars:
  PORT:
    type: integer
    default: "3000"
    required: false
  DATABASE_URL:
    type: string
    required: true
    pattern: "^postgres(ql)?://"
    secret: true
`
	s, err := LoadFromReader(strings.NewReader(yml))
	if err != nil {
		t.Fatalf("expected valid schema, got: %v", err)
	}

	if len(s.Vars) != 2 {
		t.Errorf("expected 2 vars, got %d", len(s.Vars))
	}
}

func TestValidateInvalidType(t *testing.T) {
	yml := `
version: "1"
vars:
  CUSTOM:
    type: custom_type
`
	_, err := LoadFromReader(strings.NewReader(yml))
	if err == nil {
		t.Fatal("expected error for unsupported type, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported type") {
		t.Errorf("expected 'unsupported type' in error, got: %v", err)
	}
}

func TestValidateInvalidPattern(t *testing.T) {
	yml := `
version: "1"
vars:
  BROKEN:
    type: string
    pattern: "[invalid(regex"
`
	_, err := LoadFromReader(strings.NewReader(yml))
	if err == nil {
		t.Fatal("expected error for invalid regex, got nil")
	}
}

func TestValidateVariablesAlias(t *testing.T) {
	yml := `
version: "1"
variables:
  PORT:
    type: integer
`
	s, err := LoadFromReader(strings.NewReader(yml))
	if err != nil {
		t.Fatalf("expected variables alias to parse, got: %v", err)
	}
	if _, ok := s.Vars["PORT"]; !ok {
		t.Error("expected PORT in s.Vars")
	}
}
