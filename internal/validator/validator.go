// Package validator implements the core environment variable validation engine
// for envguard. It loads a schema and a set of environment variables, then
// checks each declared variable against its specification — verifying presence,
// type, pattern, allowed values, and secret hygiene.
package validator

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/RINKESH2497/envguard/internal/dotenv"
	"github.com/RINKESH2497/envguard/internal/schema"
)

// placeholderPatterns are values that suggest a secret has not been configured.
// Each entry is matched case-insensitively against the raw value.
var placeholderPatterns = []string{
	"todo",
	"changeme",
	"change_me",
	"change-me",
	"xxx",
	"xxxx",
	"xxxxx",
	"placeholder",
	"replace_me",
	"replace-me",
	"your_api_key",
	"your-api-key",
	"your_secret",
	"your-secret",
	"insert_here",
	"insert-here",
	"fixme",
	"fill_me_in",
	"fill-me-in",
	"example",
	"test",
	"dummy",
}

var emailRegex = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// Issue represents a single validation finding — either an error or a warning.
type Issue struct {
	// VarName is the environment variable this issue pertains to.
	VarName string

	// Message describes the problem in human-readable form.
	Message string

	// Severity is either "error" or "warning".
	Severity string
}

// String returns a formatted representation of the issue suitable for display.
func (i Issue) String() string {
	return fmt.Sprintf("[%s] %s: %s", i.Severity, i.VarName, i.Message)
}

// Result aggregates all findings from a validation run.
type Result struct {
	// Errors contains issues that represent hard failures.
	Errors []Issue

	// Warnings contains advisory issues that do not cause a non-zero exit.
	Warnings []Issue

	// Valid is the number of variables that passed all checks.
	Valid int

	// Total is the total number of variables declared in the schema.
	Total int
}

// HasErrors reports whether the validation produced any errors.
func (r *Result) HasErrors() bool {
	return len(r.Errors) > 0
}

// Summary returns a human-readable overview of the validation result.
func (r *Result) Summary() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("Validated %d/%d variables", r.Valid, r.Total))

	if len(r.Errors) > 0 {
		b.WriteString(fmt.Sprintf(" — %d error(s)", len(r.Errors)))
	}
	if len(r.Warnings) > 0 {
		b.WriteString(fmt.Sprintf(", %d warning(s)", len(r.Warnings)))
	}

	if !r.HasErrors() && len(r.Warnings) == 0 {
		b.WriteString(" ✓")
	}

	return b.String()
}

// ExitCode returns the appropriate process exit code: 0 when all variables are
// valid, 1 when any errors were found.
func (r *Result) ExitCode() int {
	if r.HasErrors() {
		return 1
	}
	return 0
}

// Options configures a validation run.
type Options struct {
	// SchemaPath is the path to the envguard schema file. When empty,
	// schema.DefaultSchemaPath() ("envguard.yaml") is used.
	SchemaPath string

	// EnvPath is the path to a .env file to load variables from. When empty
	// and EnvVars is nil, the process environment (os.Environ) is used.
	EnvPath string

	// EnvVars supplies environment variables directly. When non-nil this map
	// is used instead of reading from a file or the process environment.
	EnvVars map[string]string

	// StrictMode promotes all warnings to errors, causing them to contribute
	// to a non-zero exit code.
	StrictMode bool

	// IgnoreUndocumented disables warnings/errors for variables present in
	// the environment but not declared in the schema.
	IgnoreUndocumented bool
}

// Validate performs a full validation of environment variables against the
// schema. It loads the schema from opts.SchemaPath (defaulting to
// "envguard.yaml"), resolves environment variables from opts.EnvVars, opts.EnvPath,
// or os.Environ(), and checks every declared variable.
//
// The returned Result contains all errors and warnings discovered. A non-nil
// error return indicates an infrastructure failure (e.g. the schema file could
// not be read) rather than a validation finding.
func Validate(opts Options) (*Result, error) {
	// --- 1. Load schema ---------------------------------------------------
	schemaPath := opts.SchemaPath
	if schemaPath == "" {
		schemaPath = schema.DefaultSchemaPath()
	}

	sch, err := schema.Load(schemaPath)
	if err != nil {
		return nil, fmt.Errorf("validator: load schema: %w", err)
	}

	// --- 2. Load environment variables ------------------------------------
	envVars, err := resolveEnvVars(opts)
	if err != nil {
		return nil, fmt.Errorf("validator: load env vars: %w", err)
	}

	// --- 3. Validate each declared variable -------------------------------
	result := &Result{
		Total: len(sch.Vars),
	}

	// Process variables in sorted order for deterministic output.
	varNames := make([]string, 0, len(sch.Vars))
	for name := range sch.Vars {
		varNames = append(varNames, name)
	}
	sort.Strings(varNames)

	for _, name := range varNames {
		spec := sch.Vars[name]
		value, exists := envVars[name]

		issues := validateVar(name, value, exists, spec)

		hasError := false
		for _, issue := range issues {
			if issue.Severity == "error" {
				result.Errors = append(result.Errors, issue)
				hasError = true
			} else {
				if opts.StrictMode {
					issue.Severity = "error"
					result.Errors = append(result.Errors, issue)
					hasError = true
				} else {
					result.Warnings = append(result.Warnings, issue)
				}
			}
		}

		if !hasError {
			result.Valid++
		}
	}

	// --- 4. Check for undocumented variables ------------------------------
	if !opts.IgnoreUndocumented {
		for envKey := range envVars {
			if _, declared := sch.Vars[envKey]; !declared {
				issue := Issue{
					VarName:  envKey,
					Message:  "variable is set but not declared in schema (undocumented)",
					Severity: "warning",
				}
				if opts.StrictMode {
					issue.Severity = "error"
					result.Errors = append(result.Errors, issue)
				} else {
					result.Warnings = append(result.Warnings, issue)
				}
			}
		}
	}

	return result, nil
}

// resolveEnvVars determines the source of environment variables based on the
// Options. Priority: EnvVars map > EnvPath file > os.Environ().
func resolveEnvVars(opts Options) (map[string]string, error) {
	// Explicit map takes highest priority.
	if opts.EnvVars != nil {
		return opts.EnvVars, nil
	}

	// Try .env file.
	if opts.EnvPath != "" {
		return dotenv.Parse(opts.EnvPath)
	}

	// Default: try the conventional .env file, fall back to os.Environ.
	if _, err := os.Stat(".env"); err == nil {
		return dotenv.Parse(".env")
	}

	return environToMap(), nil
}

// environToMap converts os.Environ() ([]string of "KEY=VALUE") into a map.
func environToMap() map[string]string {
	env := os.Environ()
	m := make(map[string]string, len(env))
	for _, entry := range env {
		if idx := strings.IndexByte(entry, '='); idx >= 0 {
			m[entry[:idx]] = entry[idx+1:]
		}
	}
	return m
}

// validateVar checks a single variable against its specification and returns
// any issues found.
func validateVar(name, value string, exists bool, spec schema.VarSpec) []Issue {
	var issues []Issue

	// Apply default when the variable is absent.
	if (!exists || value == "") && spec.Default != "" {
		value = spec.Default
		exists = true
	}

	// --- Deprecated check -------------------------------------------------
	if spec.Deprecated != "" && exists && value != "" {
		issues = append(issues, Issue{
			VarName:  name,
			Message:  fmt.Sprintf("variable is deprecated: %s", spec.Deprecated),
			Severity: "warning",
		})
	}

	// --- Required check ---------------------------------------------------
	if spec.Required && (!exists || strings.TrimSpace(value) == "") {
		issues = append(issues, Issue{
			VarName:  name,
			Message:  "required variable is not set",
			Severity: "error",
		})
		return issues // no further checks possible
	}

	// If the variable is absent and not required, nothing more to check.
	if !exists || value == "" {
		return issues
	}

	// --- Type check -------------------------------------------------------
	if typeErr := checkType(name, value, spec.Type); typeErr != nil {
		issues = append(issues, *typeErr)
	}

	// --- Range checks (min/max for integer/float) --------------------------
	if spec.Type == "integer" || spec.Type == "float" {
		if num, err := strconv.ParseFloat(value, 64); err == nil {
			if spec.Min != nil && num < *spec.Min {
				issues = append(issues, Issue{
					VarName:  name,
					Message:  fmt.Sprintf("value %v is less than minimum %v", num, *spec.Min),
					Severity: "error",
				})
			}
			if spec.Max != nil && num > *spec.Max {
				issues = append(issues, Issue{
					VarName:  name,
					Message:  fmt.Sprintf("value %v exceeds maximum %v", num, *spec.Max),
					Severity: "error",
				})
			}
		}
	}

	// --- Length checks (min_length/max_length) -----------------------------
	valLen := len(value)
	if spec.MinLength != nil && valLen < *spec.MinLength {
		issues = append(issues, Issue{
			VarName:  name,
			Message:  fmt.Sprintf("length %d is less than minimum length %d", valLen, *spec.MinLength),
			Severity: "error",
		})
	}
	if spec.MaxLength != nil && valLen > *spec.MaxLength {
		issues = append(issues, Issue{
			VarName:  name,
			Message:  fmt.Sprintf("length %d exceeds maximum length %d", valLen, *spec.MaxLength),
			Severity: "error",
		})
	}

	// --- Pattern check (anchored for full match) --------------------------
	if spec.Pattern != "" {
		pattern := spec.Pattern
		if !strings.HasPrefix(pattern, "^") && !strings.HasSuffix(pattern, "$") {
			pattern = "^(?:" + pattern + ")$"
		}
		matched, err := regexp.MatchString(pattern, value)
		if err != nil {
			issues = append(issues, Issue{
				VarName:  name,
				Message:  fmt.Sprintf("pattern %q failed to compile: %v", spec.Pattern, err),
				Severity: "error",
			})
		} else if !matched {
			issues = append(issues, Issue{
				VarName:  name,
				Message:  fmt.Sprintf("value %q does not match pattern %q", value, spec.Pattern),
				Severity: "error",
			})
		}
	}

	// --- Allowed values check ---------------------------------------------
	if len(spec.Allowed) > 0 {
		found := false
		for _, a := range spec.Allowed {
			if value == a {
				found = true
				break
			}
		}
		if !found {
			issues = append(issues, Issue{
				VarName:  name,
				Message:  fmt.Sprintf("value %q is not in allowed set [%s]", value, strings.Join(spec.Allowed, ", ")),
				Severity: "error",
			})
		}
	}

	// --- Secret placeholder check -----------------------------------------
	if spec.Secret && strings.TrimSpace(value) != "" {
		if looksLikePlaceholder(value) {
			issues = append(issues, Issue{
				VarName:  name,
				Message:  fmt.Sprintf("secret variable appears to contain a placeholder value %q", value),
				Severity: "warning",
			})
		}
	}

	return issues
}

// checkType validates that value conforms to the declared type. Returns an
// Issue pointer on failure, or nil when the check passes.
func checkType(name, value, typ string) *Issue {
	switch typ {
	case "string":
		// All values are valid strings.
		return nil

	case "integer":
		if _, err := strconv.Atoi(value); err != nil {
			return &Issue{
				VarName:  name,
				Message:  fmt.Sprintf("expected integer, got %q", value),
				Severity: "error",
			}
		}

	case "float":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return &Issue{
				VarName:  name,
				Message:  fmt.Sprintf("expected float, got %q", value),
				Severity: "error",
			}
		}

	case "boolean":
		lower := strings.ToLower(strings.TrimSpace(value))
		switch lower {
		case "1", "0", "t", "f", "true", "false", "yes", "no", "y", "n", "on", "off":
			return nil
		default:
			return &Issue{
				VarName:  name,
				Message:  fmt.Sprintf("expected boolean, got %q (accepted: 1, 0, t, f, true, false, yes, no)", value),
				Severity: "error",
			}
		}

	case "url":
		u, err := url.Parse(value)
		if err != nil || u.Scheme == "" || u.Host == "" {
			return &Issue{
				VarName:  name,
				Message:  fmt.Sprintf("expected url with scheme and host, got %q", value),
				Severity: "error",
			}
		}

	case "email":
		if !emailRegex.MatchString(value) {
			return &Issue{
				VarName:  name,
				Message:  fmt.Sprintf("expected email address, got %q", value),
				Severity: "error",
			}
		}

	default:
		// Unknown type — should have been caught by schema validation, but
		// guard against it regardless.
		return &Issue{
			VarName:  name,
			Message:  fmt.Sprintf("unknown type %q", typ),
			Severity: "error",
		}
	}

	return nil
}

// looksLikePlaceholder returns true if value appears to be a dummy or
// placeholder value commonly left in .env files.
func looksLikePlaceholder(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return true
	}

	for _, p := range placeholderPatterns {
		if lower == p {
			return true
		}
	}

	return false
}
