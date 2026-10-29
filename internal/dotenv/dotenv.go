// Package dotenv provides a lightweight parser and writer for .env files.
//
// The parser handles the common .env conventions:
//   - KEY=value (basic assignment)
//   - KEY="quoted value" (double-quoted, with escape sequences \n, \t, \\, \")
//   - KEY='literal value' (single-quoted, no escape processing)
//   - KEY= (empty value)
//   - # full-line comments
//   - Inline comments: KEY=value # comment (unquoted values only)
//   - export KEY=value (optional export prefix)
//   - Blank lines are silently ignored
//   - Whitespace is trimmed around keys and unquoted values
//
// Malformed lines are skipped without causing a parse error.
package dotenv

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// Parse reads a .env file at the given path and returns a map of key-value
// pairs. Malformed lines are silently skipped.
func Parse(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("dotenv: open %q: %w", path, err)
	}
	defer f.Close()

	return ParseReader(f)
}

// ReadFile reads a .env file at the given path. It is an alias for Parse.
func ReadFile(path string) (map[string]string, error) {
	return Parse(path)
}

// ParseReader reads key-value pairs from r in .env format. Malformed lines are
// silently skipped.
func ParseReader(r io.Reader) (map[string]string, error) {
	vars := make(map[string]string)
	scanner := bufio.NewScanner(r)

	for scanner.Scan() {
		line := scanner.Text()
		key, value, ok := parseLine(line)
		if ok {
			vars[key] = value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("dotenv: read: %w", err)
	}

	return vars, nil
}

// parseLine parses a single .env line and returns the key, value, and whether
// the line was valid. Invalid or empty/comment lines return ok == false.
func parseLine(raw string) (key, value string, ok bool) {
	line := strings.TrimSpace(raw)

	// Skip empty lines and comments.
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}

	// Strip optional "export " prefix.
	if strings.HasPrefix(line, "export ") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
	} else if strings.HasPrefix(line, "export\t") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "export"))
	}

	// Find the = separator.
	eqIdx := strings.IndexByte(line, '=')
	if eqIdx < 0 {
		// No equals sign — malformed, skip.
		return "", "", false
	}

	key = strings.TrimSpace(line[:eqIdx])
	if key == "" {
		return "", "", false
	}

	raw = line[eqIdx+1:]

	// Parse the value portion.
	value = parseValue(raw)

	return key, value, true
}

// parseValue interprets the right-hand side of a KEY=... assignment, handling
// double-quoted strings (with escape sequences), single-quoted strings (literal),
// and bare values (with optional inline comments).
func parseValue(raw string) string {
	trimmed := strings.TrimSpace(raw)

	if len(trimmed) == 0 {
		return ""
	}

	// Double-quoted value: KEY="value"
	if trimmed[0] == '"' {
		return parseDoubleQuoted(trimmed)
	}

	// Single-quoted value: KEY='value'
	if trimmed[0] == '\'' {
		return parseSingleQuoted(trimmed)
	}

	// Bare (unquoted) value — strip inline comments.
	return parseBareValue(trimmed)
}

// parseDoubleQuoted extracts a double-quoted value and processes escape
// sequences (\n, \t, \\, \"). If the closing quote is missing the remainder
// of the string is returned as-is.
func parseDoubleQuoted(s string) string {
	// s starts with "
	var b strings.Builder
	i := 1 // skip opening quote

	for i < len(s) {
		ch := s[i]

		if ch == '\\' && i+1 < len(s) {
			next := s[i+1]
			switch next {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case '\\':
				b.WriteByte('\\')
			case '"':
				b.WriteByte('"')
			default:
				// Unknown escape — keep both characters.
				b.WriteByte('\\')
				b.WriteByte(next)
			}
			i += 2
			continue
		}

		if ch == '"' {
			// Closing quote found.
			return b.String()
		}

		b.WriteByte(ch)
		i++
	}

	// No closing quote — return what we have.
	return b.String()
}

// parseSingleQuoted extracts a single-quoted value literally (no escape
// processing). If the closing quote is missing the remainder is returned.
func parseSingleQuoted(s string) string {
	// s starts with '
	end := strings.IndexByte(s[1:], '\'')
	if end < 0 {
		// No closing quote — return everything after the opening quote.
		return s[1:]
	}
	return s[1 : end+1]
}

// parseBareValue extracts an unquoted value. An inline comment (` # ...` with
// a leading space) is stripped.
func parseBareValue(s string) string {
	// Look for inline comment: space followed by #.
	if idx := strings.Index(s, " #"); idx >= 0 {
		s = s[:idx]
	}
	return strings.TrimSpace(s)
}

// Write creates or overwrites a .env file at path with the given key-value
// pairs. If comments is non-nil, each key present in comments has a comment
// line written above its assignment.
//
// Keys are written in sorted order for deterministic output.
func Write(path string, vars map[string]string, comments map[string]string) error {
	data, err := Marshal(vars, comments)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("dotenv: write %q: %w", path, err)
	}
	return nil
}

// Marshal serialises key-value pairs into .env format. If comments is non-nil,
// each key present in comments receives a "# <comment>" line above its
// assignment. Keys are emitted in sorted order.
func Marshal(vars map[string]string, comments map[string]string) ([]byte, error) {
	keys := make([]string, 0, len(vars))
	for k := range vars {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder

	for i, k := range keys {
		// Blank line between entries for readability (except before the first).
		if i > 0 {
			b.WriteByte('\n')
		}

		// Optional comment.
		if comments != nil {
			if c, ok := comments[k]; ok && c != "" {
				b.WriteString("# ")
				b.WriteString(c)
				b.WriteByte('\n')
			}
		}

		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(quoteValue(vars[k]))
		b.WriteByte('\n')
	}

	return []byte(b.String()), nil
}

// quoteValue decides whether a value needs quoting and returns the
// appropriately formatted string. Values containing newlines, tabs, quotes,
// or the comment marker " #" are double-quoted with escapes applied. All
// other values are written bare.
func quoteValue(v string) string {
	if v == "" {
		return ""
	}

	needsQuote := strings.ContainsAny(v, "\n\t\"' #")

	if !needsQuote {
		return v
	}

	var b strings.Builder
	b.WriteByte('"')
	for _, ch := range v {
		switch ch {
		case '\n':
			b.WriteString(`\n`)
		case '\t':
			b.WriteString(`\t`)
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		default:
			b.WriteRune(ch)
		}
	}
	b.WriteByte('"')
	return b.String()
}
