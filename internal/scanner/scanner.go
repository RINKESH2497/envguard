// Package scanner detects accidentally committed secrets and sensitive values
// in source files. It combines pattern-based matching (known secret formats)
// with Shannon entropy analysis to identify high-entropy strings that are
// likely credentials.
package scanner

import (
	"bufio"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Finding represents a single detected secret or sensitive value.
type Finding struct {
	// FilePath is the path of the file containing the finding.
	FilePath string

	// Line is the 1-based line number where the match was found.
	Line int

	// Match is the redacted form of the matched content. Only the first and
	// last few characters are preserved; the rest is replaced with asterisks.
	Match string

	// Pattern is a human-readable label for the detection rule that matched.
	Pattern string

	// Severity indicates the risk level: "high", "medium", or "low".
	Severity string
}

// Options configures a scan operation.
type Options struct {
	// Paths lists files and/or directories to scan.
	Paths []string

	// Recursive enables recursive scanning of directories.
	Recursive bool

	// ExcludePatterns contains glob patterns for files to skip (e.g. "*.log").
	ExcludePatterns []string

	// EntropyThreshold sets the minimum Shannon entropy required for a string
	// to be flagged by entropy-based detection. Defaults to 4.5.
	EntropyThreshold float64
}

// ScanResult summarises the output of a scan operation.
type ScanResult struct {
	// Findings is the list of detected secrets.
	Findings []Finding

	// FilesScanned is the total number of files that were analysed.
	FilesScanned int

	// SecretsFound is the total number of findings (len(Findings)).
	SecretsFound int
}

// secretPattern pairs a compiled regexp with a human-readable label and severity.
type secretPattern struct {
	re       *regexp.Regexp
	label    string
	severity string
}

// knownPatterns is the set of compiled secret-detection regexps.
var knownPatterns = []secretPattern{
	{
		re:       regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
		label:    "AWS Access Key",
		severity: "high",
	},
	{
		re:       regexp.MustCompile(`gh[ps]_[A-Za-z0-9_]{36,}`),
		label:    "GitHub Token",
		severity: "high",
	},
	{
		re:       regexp.MustCompile(`github_pat_[A-Za-z0-9_]{22,}`),
		label:    "GitHub Fine-grained Token",
		severity: "high",
	},
	{
		re:       regexp.MustCompile(`(?i)(api[_-]?key|apikey)\s*[:=]\s*['"]?[A-Za-z0-9]{20,}`),
		label:    "Generic API Key",
		severity: "medium",
	},
	{
		re:       regexp.MustCompile(`-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----`),
		label:    "Private Key",
		severity: "high",
	},
	{
		re:       regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`),
		label:    "JWT",
		severity: "medium",
	},
	{
		re:       regexp.MustCompile(`xox[bpors]-[0-9]{10,}-[0-9a-zA-Z]{10,}`),
		label:    "Slack Token",
		severity: "high",
	},
	{
		re:       regexp.MustCompile(`(?i)(secret|password|passwd|token)\s*[:=]\s*['"]?[^\s'"]{8,}`),
		label:    "Generic Secret",
		severity: "medium",
	},
	// AWS Secret Key is checked separately because it requires entropy validation.
}

// awsSecretKeyRe matches potential AWS secret keys (40-char base64-ish strings).
var awsSecretKeyRe = regexp.MustCompile(`[0-9a-zA-Z/+=]{40}`)

// binaryExtensions lists file extensions that should always be skipped.
var binaryExtensions = map[string]bool{
	".exe":   true,
	".dll":   true,
	".so":    true,
	".dylib": true,
	".bin":   true,
	".png":   true,
	".jpg":   true,
	".jpeg":  true,
	".gif":   true,
	".ico":   true,
	".woff":  true,
	".woff2": true,
	".ttf":   true,
	".eot":   true,
}

// skipDirs lists directory names that should always be skipped.
var skipDirs = map[string]bool{
	".git":        true,
	"node_modules": true,
	"vendor":       true,
	"__pycache__":  true,
}

// skipFiles lists specific filenames that should always be skipped.
var skipFiles = map[string]bool{
	".env": true,
}

// Scan walks the configured paths, scans each eligible file, and returns
// aggregated results.
func Scan(opts Options) (*ScanResult, error) {
	if opts.EntropyThreshold <= 0 {
		opts.EntropyThreshold = 4.5
	}

	result := &ScanResult{}
	seen := make(map[string]bool) // avoid scanning the same file twice

	for _, p := range opts.Paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("scanner: cannot stat %q: %w", p, err)
		}

		if !info.IsDir() {
			if seen[p] {
				continue
			}
			seen[p] = true
			if err := scanOneFile(p, opts, result); err != nil {
				return nil, err
			}
			continue
		}

		// Walk directory.
		walkFn := func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if d.IsDir() {
				base := d.Name()
				if skipDirs[base] {
					return filepath.SkipDir
				}
				if !opts.Recursive && path != p {
					return filepath.SkipDir
				}
				return nil
			}

			if seen[path] {
				return nil
			}
			seen[path] = true
			return scanOneFile(path, opts, result)
		}

		if err := filepath.WalkDir(p, walkFn); err != nil {
			return nil, fmt.Errorf("scanner: walking %q: %w", p, err)
		}
	}

	result.SecretsFound = len(result.Findings)
	return result, nil
}

// scanOneFile scans a single file and appends findings to result.
func scanOneFile(path string, opts Options, result *ScanResult) error {
	if shouldSkipFile(path, opts.ExcludePatterns) {
		return nil
	}

	findings, err := ScanFile(path, opts.EntropyThreshold)
	if err != nil {
		return err
	}

	result.FilesScanned++
	result.Findings = append(result.Findings, findings...)
	return nil
}

// shouldSkipFile returns true if the file should be excluded from scanning
// based on its extension, name, or the configured exclude patterns.
func shouldSkipFile(path string, excludePatterns []string) bool {
	base := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(path))

	if binaryExtensions[ext] {
		return true
	}

	if skipFiles[base] {
		return true
	}

	for _, pattern := range excludePatterns {
		if matched, _ := filepath.Match(pattern, base); matched {
			return true
		}
	}

	return false
}

// ScanFile scans a single file for secrets and returns any findings. The
// threshold parameter controls the minimum Shannon entropy for entropy-based
// detection.
func ScanFile(path string, threshold float64) ([]Finding, error) {
	if threshold <= 0 {
		threshold = 4.5
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("scanner: opening %q: %w", path, err)
	}
	defer f.Close()

	var findings []Finding
	scanner := bufio.NewScanner(f)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Check each known pattern.
		for _, pat := range knownPatterns {
			matches := pat.re.FindAllString(line, -1)
			for _, m := range matches {
				findings = append(findings, Finding{
					FilePath: path,
					Line:     lineNum,
					Match:    redact(m),
					Pattern:  pat.label,
					Severity: pat.severity,
				})
			}
		}

		// Check for AWS Secret Key (requires high entropy).
		for _, m := range awsSecretKeyRe.FindAllString(line, -1) {
			if shannonEntropy(m) >= threshold {
				findings = append(findings, Finding{
					FilePath: path,
					Line:     lineNum,
					Match:    redact(m),
					Pattern:  "AWS Secret Key (high entropy)",
					Severity: "high",
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner: reading %q: %w", path, err)
	}

	return findings, nil
}

// shannonEntropy calculates the Shannon entropy of a string in bits per
// character. Higher entropy indicates more randomness, which is characteristic
// of secrets and cryptographic keys.
func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}

	freq := make(map[rune]int)
	for _, c := range s {
		freq[c]++
	}

	length := float64(len([]rune(s)))
	entropy := 0.0

	for _, count := range freq {
		p := float64(count) / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

// redact masks the middle portion of a matched string, preserving only the
// first 4 and last 4 characters for context.
func redact(s string) string {
	if len(s) <= 12 {
		// For very short strings, show first 2 and last 2.
		if len(s) <= 4 {
			return strings.Repeat("*", len(s))
		}
		return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
	}
	return s[:4] + strings.Repeat("*", len(s)-8) + s[len(s)-4:]
}
