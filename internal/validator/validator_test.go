package validator

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateBooleans(t *testing.T) {
	tmpDir := t.TempDir()
	schemaFile := filepath.Join(tmpDir, "envguard.yaml")
	schemaContent := `
version: "1"
vars:
  IS_ENABLED:
    type: boolean
    required: true
`
	if err := os.WriteFile(schemaFile, []byte(schemaContent), 0644); err != nil {
		t.Fatal(err)
	}

	validValues := []string{"true", "false", "1", "0", "yes", "no", "YES", "NO", "t", "f"}
	for _, val := range validValues {
		res, err := Validate(Options{
			SchemaPath: schemaFile,
			EnvVars: map[string]string{
				"IS_ENABLED": val,
			},
		})
		if err != nil {
			t.Fatalf("unexpected validation failure on %q: %v", val, err)
		}
		if res.HasErrors() {
			t.Errorf("expected boolean %q to be valid, got errors: %v", val, res.Errors)
		}
	}

	invalidValues := []string{"not_bool", "maybe", "2", "-1"}
	for _, val := range invalidValues {
		res, err := Validate(Options{
			SchemaPath: schemaFile,
			EnvVars: map[string]string{
				"IS_ENABLED": val,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !res.HasErrors() {
			t.Errorf("expected boolean %q to fail, passed without errors", val)
		}
	}
}

func TestValidateMinMaxConstraints(t *testing.T) {
	tmpDir := t.TempDir()
	schemaFile := filepath.Join(tmpDir, "envguard.yaml")
	schemaContent := `
version: "1"
vars:
  PORT:
    type: integer
    required: true
    min: 1
    max: 65535
  SECRET:
    type: string
    required: true
    min_length: 8
    max_length: 16
`
	if err := os.WriteFile(schemaFile, []byte(schemaContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Test out of bounds
	res, err := Validate(Options{
		SchemaPath: schemaFile,
		EnvVars: map[string]string{
			"PORT":   "99999",
			"SECRET": "short",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Errors) != 2 {
		t.Errorf("expected 2 errors for min/max out of bounds, got %d: %v", len(res.Errors), res.Errors)
	}

	// Test valid bounds
	res, err = Validate(Options{
		SchemaPath: schemaFile,
		EnvVars: map[string]string{
			"PORT":   "8080",
			"SECRET": "valid_secret_12",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Errorf("expected valid bounds to pass, got: %v", res.Errors)
	}
}

func TestValidateRequiredEmptyString(t *testing.T) {
	tmpDir := t.TempDir()
	schemaFile := filepath.Join(tmpDir, "envguard.yaml")
	schemaContent := `
version: "1"
vars:
  MUST_HAVE:
    type: string
    required: true
`
	if err := os.WriteFile(schemaFile, []byte(schemaContent), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := Validate(Options{
		SchemaPath: schemaFile,
		EnvVars: map[string]string{
			"MUST_HAVE": "",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasErrors() {
		t.Error("expected required variable with empty string to produce error")
	}
}

func TestValidateStrictModePromotesUndocumented(t *testing.T) {
	tmpDir := t.TempDir()
	schemaFile := filepath.Join(tmpDir, "envguard.yaml")
	schemaContent := `
version: "1"
vars:
  KNOWN:
    type: string
`
	if err := os.WriteFile(schemaFile, []byte(schemaContent), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := Validate(Options{
		SchemaPath: schemaFile,
		StrictMode: true,
		EnvVars: map[string]string{
			"KNOWN":   "val",
			"UNKNOWN": "unexpected",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasErrors() {
		t.Error("expected strict mode to promote undocumented variable warning to error")
	}
}
