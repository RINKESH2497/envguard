// Package interactive provides an interactive onboarding experience that walks
// new contributors through setting up their .env file by prompting for each
// variable defined in the envguard schema.
package interactive

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/envguard/envguard/internal/dotenv"
	"github.com/envguard/envguard/internal/schema"
)

// Options configures the interactive setup session.
type Options struct {
	// SchemaPath is the path to the envguard.yaml schema file.
	SchemaPath string

	// OutputPath is the destination for the generated .env file.
	// Defaults to ".env" when empty.
	OutputPath string

	// SkipOptional automatically skips all optional variables.
	SkipOptional bool

	// UseDefaults automatically accepts default values without prompting.
	UseDefaults bool

	// In is the input reader for user responses. Defaults to os.Stdin.
	In io.Reader

	// Out is the output writer for prompts and messages. Defaults to os.Stdout.
	Out io.Writer
}

// SetupResult summarises the outcome of an interactive setup session.
type SetupResult struct {
	// VarsSet is the number of variables that were assigned a value.
	VarsSet int

	// VarsSkipped is the number of variables that were skipped.
	VarsSkipped int

	// OutputPath is the path where the .env file was written.
	OutputPath string
}

// defaults fills zero-value fields in Options with sensible defaults.
func (o *Options) defaults() {
	if o.OutputPath == "" {
		o.OutputPath = ".env"
	}
	if o.In == nil {
		o.In = os.Stdin
	}
	if o.Out == nil {
		o.Out = os.Stdout
	}
}

// Run starts an interactive onboarding session. It loads the schema, prompts
// the user for each variable, validates input, and writes the resulting .env
// file.
func Run(opts Options) (*SetupResult, error) {
	opts.defaults()

	s, err := schema.LoadFile(opts.SchemaPath)
	if err != nil {
		return nil, fmt.Errorf("interactive: loading schema: %w", err)
	}

	out := opts.Out
	scanner := bufio.NewScanner(opts.In)

	// Check for existing .env file.
	existing := make(map[string]string)
	mergeMode := false

	if _, statErr := os.Stat(opts.OutputPath); statErr == nil {
		existing, err = dotenv.ReadFile(opts.OutputPath)
		if err != nil {
			return nil, fmt.Errorf("interactive: reading existing .env: %w", err)
		}

		fmt.Fprintf(out, "\n⚠  Existing .env file found at %s\n", opts.OutputPath)
		fmt.Fprintf(out, "   [o] Overwrite — start fresh\n")
		fmt.Fprintf(out, "   [m] Merge — keep existing values, prompt for new ones\n")
		fmt.Fprintf(out, "   Choose (o/m): ")

		if scanner.Scan() {
			choice := strings.TrimSpace(strings.ToLower(scanner.Text()))
			if choice == "m" || choice == "merge" {
				mergeMode = true
				fmt.Fprintf(out, "   → Merging with existing values.\n\n")
			} else {
				existing = make(map[string]string)
				fmt.Fprintf(out, "   → Starting fresh.\n\n")
			}
		}
	}

	// Organise vars: required first, then optional.
	required, optional := splitByRequired(s.OrderedVars())
	orderedVars := append(required, optional...)

	result := &SetupResult{OutputPath: opts.OutputPath}
	envValues := make(map[string]string)

	// Copy existing values if merging.
	if mergeMode {
		for k, v := range existing {
			envValues[k] = v
		}
	}

	fmt.Fprintf(out, "🔧 envguard interactive setup\n")
	fmt.Fprintf(out, "   Schema: %s\n", opts.SchemaPath)
	fmt.Fprintf(out, "   Variables: %d required, %d optional\n\n", len(required), len(optional))

	for _, v := range orderedVars {
		// Skip optional vars if requested.
		if !v.Required && opts.SkipOptional {
			result.VarsSkipped++
			continue
		}

		// In merge mode, skip vars that already have values.
		if mergeMode {
			if val, exists := existing[v.Name]; exists && val != "" {
				fmt.Fprintf(out, "   ✓ %s — keeping existing value\n", v.Name)
				result.VarsSet++
				continue
			}
		}

		// Auto-accept defaults if UseDefaults is set and a default exists.
		if opts.UseDefaults && v.Default != "" {
			envValues[v.Name] = v.Default
			fmt.Fprintf(out, "   ✓ %s = %s (default)\n", v.Name, v.Default)
			result.VarsSet++
			continue
		}

		value, skipped, err := promptForVar(v, scanner, out)
		if err != nil {
			return nil, err
		}

		if skipped {
			result.VarsSkipped++
			continue
		}

		envValues[v.Name] = value
		result.VarsSet++
	}

	// Write the .env file.
	if err := writeEnvFile(opts.OutputPath, orderedVars, envValues); err != nil {
		return nil, fmt.Errorf("interactive: writing .env: %w", err)
	}

	// Print summary.
	fmt.Fprintf(out, "\n──────────────────────────────────────\n")
	fmt.Fprintf(out, "✅ Setup complete!\n")
	fmt.Fprintf(out, "   File:     %s\n", opts.OutputPath)
	fmt.Fprintf(out, "   Set:      %d variables\n", result.VarsSet)
	fmt.Fprintf(out, "   Skipped:  %d variables\n", result.VarsSkipped)
	fmt.Fprintf(out, "──────────────────────────────────────\n")

	return result, nil
}

// promptForVar handles the interactive prompt for a single variable.
// It returns the entered value, whether it was skipped, and any error.
func promptForVar(v schema.Var, sc *bufio.Scanner, out io.Writer) (string, bool, error) {
	reqLabel := "optional"
	if v.Required {
		reqLabel = "REQUIRED"
	}

	fmt.Fprintf(out, "\n── %s (%s) ──\n", v.Name, reqLabel)

	// Show description.
	if v.Description != "" {
		fmt.Fprintf(out, "   %s\n", v.Description)
	}

	// Show type.
	if v.Type != "" {
		fmt.Fprintf(out, "   Type: %s\n", v.Type)
	}

	// Show secret note.
	if v.Secret {
		fmt.Fprintf(out, "   🔒 This is a secret value\n")
	}

	// Show allowed values as a numbered list.
	if len(v.Allowed) > 0 {
		fmt.Fprintf(out, "   Allowed values:\n")
		for i, a := range v.Allowed {
			fmt.Fprintf(out, "     %d) %s\n", i+1, a)
		}
	}

	// Build prompt.
	prompt := fmt.Sprintf("   Enter value")
	if v.Default != "" {
		prompt += fmt.Sprintf(" [%s]", v.Default)
	}
	if !v.Required {
		prompt += " (or 's' to skip)"
	}
	prompt += ": "

	for {
		fmt.Fprint(out, prompt)

		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				return "", false, fmt.Errorf("interactive: reading input: %w", err)
			}
			// EOF — treat as skip for optional, error for required.
			if v.Required {
				return "", false, fmt.Errorf("interactive: unexpected end of input for required var %s", v.Name)
			}
			return "", true, nil
		}

		input := strings.TrimSpace(sc.Text())

		// Handle skip.
		if input == "s" && !v.Required {
			fmt.Fprintf(out, "   → Skipped\n")
			return "", true, nil
		}

		// Handle empty input — use default if available.
		if input == "" {
			if v.Default != "" {
				fmt.Fprintf(out, "   → Using default: %s\n", v.Default)
				return v.Default, false, nil
			}
			if v.Required {
				fmt.Fprintf(out, "   ✗ This variable is required. Please enter a value.\n")
				continue
			}
			return "", true, nil
		}

		// Handle numbered selection for allowed values.
		if len(v.Allowed) > 0 {
			if num, err := strconv.Atoi(input); err == nil && num >= 1 && num <= len(v.Allowed) {
				input = v.Allowed[num-1]
				fmt.Fprintf(out, "   → Selected: %s\n", input)
			}
		}

		// Validate the input.
		if validationErr := validateInput(input, v); validationErr != nil {
			fmt.Fprintf(out, "   ✗ %s\n", validationErr)
			continue
		}

		return input, false, nil
	}
}

// validateInput checks that the provided value is acceptable for the variable
// according to its type, pattern, and allowed values.
func validateInput(value string, v schema.Var) error {
	// Check allowed values.
	if len(v.Allowed) > 0 {
		found := false
		for _, a := range v.Allowed {
			if value == a {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("value must be one of: %s", strings.Join(v.Allowed, ", "))
		}
	}

	// Check type-based validation.
	switch v.Type {
	case "int", "integer":
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return fmt.Errorf("expected an integer value")
		}
	case "float", "number":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return fmt.Errorf("expected a numeric value")
		}
	case "bool", "boolean":
		lower := strings.ToLower(value)
		valid := map[string]bool{
			"true": true, "false": true,
			"1": true, "0": true,
			"yes": true, "no": true,
		}
		if !valid[lower] {
			return fmt.Errorf("expected a boolean (true/false, 1/0, yes/no)")
		}
	case "url":
		if !strings.HasPrefix(value, "http://") && !strings.HasPrefix(value, "https://") {
			return fmt.Errorf("expected a URL starting with http:// or https://")
		}
	case "email":
		if !strings.Contains(value, "@") || !strings.Contains(value, ".") {
			return fmt.Errorf("expected a valid email address")
		}
	}

	// Check pattern if specified.
	if v.Pattern != "" {
		// We import regexp here lazily to avoid a dependency just for this case.
		// Since schema.Var has a Pattern field we validate against it.
		matched, err := matchPattern(v.Pattern, value)
		if err != nil {
			return fmt.Errorf("invalid pattern %q in schema: %w", v.Pattern, err)
		}
		if !matched {
			return fmt.Errorf("value does not match required pattern: %s", v.Pattern)
		}
	}

	return nil
}

// matchPattern compiles the pattern and tests the value against it.
func matchPattern(pattern, value string) (bool, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, err
	}
	return re.MatchString(value), nil
}

// splitByRequired partitions variables into required and optional slices,
// preserving original order within each group.
func splitByRequired(vars []schema.Var) (required, optional []schema.Var) {
	for _, v := range vars {
		if v.Required {
			required = append(required, v)
		} else {
			optional = append(optional, v)
		}
	}
	return required, optional
}

// writeEnvFile writes the final .env file with variable assignments.
func writeEnvFile(path string, orderedVars []schema.Var, values map[string]string) error {
	var buf strings.Builder

	buf.WriteString("# Generated by envguard onboard\n\n")

	for _, v := range orderedVars {
		val, exists := values[v.Name]
		if !exists {
			continue
		}

		if v.Description != "" {
			buf.WriteString(fmt.Sprintf("# %s\n", v.Description))
		}

		// Quote values that contain spaces or special characters.
		if strings.ContainsAny(val, " \t#'\"") {
			val = fmt.Sprintf("%q", val)
		}

		buf.WriteString(fmt.Sprintf("%s=%s\n", v.Name, val))
	}

	return os.WriteFile(path, []byte(buf.String()), 0644)
}
