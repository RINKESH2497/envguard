package scanner

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScanFileKnownPatterns(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "secrets.txt")
	content := `
# Config file with sample leaked keys
AWS_KEY=AKIAIOSFODNN7EXAMPLE
GH_TOKEN=ghp_123456789012345678901234567890123456
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	findings, err := ScanFile(testFile, 4.5)
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(findings) < 2 {
		t.Fatalf("expected at least 2 findings, got %d", len(findings))
	}

	patterns := make(map[string]bool)
	for _, f := range findings {
		patterns[f.Pattern] = true
	}

	if !patterns["AWS Access Key"] {
		t.Error("expected AWS Access Key pattern detection")
	}
	if !patterns["GitHub Token"] {
		t.Error("expected GitHub Token pattern detection")
	}
}

func TestScanLongLineDoesNotCrash(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "bundle.min.js")
	// Create a line > 100KB (exceeds default bufio 64KB limit)
	longLine := "var bundle = \"" + strings.Repeat("A", 120*1024) + "\";\n"
	if err := os.WriteFile(testFile, []byte(longLine), 0644); err != nil {
		t.Fatal(err)
	}

	findings, err := ScanFile(testFile, 4.5)
	if err != nil {
		t.Fatalf("scanner crashed on long line: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestScanBinarySkipped(t *testing.T) {
	tmpDir := t.TempDir()
	binFile := filepath.Join(tmpDir, "testbinary")
	// Write file containing null bytes
	data := []byte{0x7f, 'E', 'L', 'F', 0x00, 0x01, 0x02}
	if err := os.WriteFile(binFile, data, 0755); err != nil {
		t.Fatal(err)
	}

	findings, err := ScanFile(binFile, 4.5)
	if err != nil {
		t.Fatalf("scanner error on binary: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for binary, got %d", len(findings))
	}
}

func TestScanExplicitDotEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	content := "AWS_KEY=AKIAIOSFODNN7EXAMPLE\n"
	if err := os.WriteFile(envFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Directly scanning .env explicitly should find the secret
	res, err := Scan(Options{
		Paths: []string{envFile},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.SecretsFound == 0 {
		t.Error("expected explicit .env scan to detect secrets, found 0")
	}

	// Scanning directory containing .env should also detect secrets
	resDir, err := Scan(Options{
		Paths:     []string{tmpDir},
		Recursive: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resDir.SecretsFound == 0 {
		t.Error("expected directory scan to include .env and detect secrets, found 0")
	}
}
