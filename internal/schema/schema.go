// Package schema provides types and parsing logic for envguard schema files.
//
// An envguard schema is a YAML file (by default "envguard.yaml") that declares
// the environment variables an application expects, along with their types,
// constraints, and documentation.
package schema

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// supportedTypes enumerates every type value that a VarSpec may declare.
var supportedTypes = []string{
	"string",
	"integer",
	"float",
	"boolean",
	"url",
	"email",
}

// supportedTypeSet is a lookup set built from supportedTypes for O(1) membership
// checks during schema validation.
var supportedTypeSet map[string]struct{}

func init() {
	supportedTypeSet = make(map[string]struct{}, len(supportedTypes))
	for _, t := range supportedTypes {
		supportedTypeSet[t] = struct{}{}
	}
}

// Schema is the top-level representation of an envguard schema file.
type Schema struct {
	// Version is the schema format version (e.g. "1").
	Version string `yaml:"version"`

	// Meta contains arbitrary project metadata (e.g. name, description).
	Meta map[string]any `yaml:"meta,omitempty"`

	// Vars maps environment variable names to their specifications.
	Vars map[string]VarSpec `yaml:"vars,omitempty"`

	// Variables is an accepted alias for Vars in the schema YAML.
	Variables map[string]VarSpec `yaml:"variables,omitempty"`
}

// VarSpec describes the constraints and metadata for a single environment
// variable declared in the schema.
type VarSpec struct {
	// Type is the expected value type. Must be one of the values returned by
	// SupportedTypes (string, integer, float, boolean, url, email).
	// Defaults to "string" when empty.
	Type string `yaml:"type"`

	// Required indicates whether the variable must be present. When true and
	// the variable is missing, validation produces an error.
	Required bool `yaml:"required"`

	// Default is the fallback value used when the variable is absent.
	Default string `yaml:"default,omitempty"`

	// Description is a human-readable explanation of the variable's purpose.
	Description string `yaml:"description,omitempty"`

	// Pattern is a regular expression the value must match. When non-empty the
	// pattern is compiled and tested against the raw string value.
	Pattern string `yaml:"pattern,omitempty"`

	// Allowed is a whitelist of acceptable values. When non-empty the variable
	// value must be an exact match for one of the listed strings.
	Allowed []string `yaml:"allowed,omitempty"`

	// Secret marks the variable as sensitive (e.g. API keys, passwords). The
	// validator may emit warnings when the value looks like a placeholder.
	Secret bool `yaml:"secret,omitempty"`

	// Example is a sample value shown in documentation and generated .env
	// templates.
	Example string `yaml:"example,omitempty"`

	// Min is the minimum numeric value for integer or float variables.
	Min *float64 `yaml:"min,omitempty"`

	// Max is the maximum numeric value for integer or float variables.
	Max *float64 `yaml:"max,omitempty"`

	// MinLength is the minimum string length.
	MinLength *int `yaml:"min_length,omitempty"`

	// MaxLength is the maximum string length.
	MaxLength *int `yaml:"max_length,omitempty"`

	// Deprecated emits an advisory warning if this variable is set.
	Deprecated string `yaml:"deprecated,omitempty"`
}

// DefaultSchemaPath returns the conventional file name for an envguard schema.
func DefaultSchemaPath() string {
	return "envguard.yaml"
}

// SupportedTypes returns the list of type names that VarSpec.Type may contain.
// The returned slice is a copy; callers may modify it freely.
func SupportedTypes() []string {
	out := make([]string, len(supportedTypes))
	copy(out, supportedTypes)
	return out
}

// Load reads and parses an envguard schema from the file at path.
// It returns a validated Schema or an error describing what went wrong.
func Load(path string) (*Schema, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("schema: open %q: %w", path, err)
	}
	defer f.Close()

	return LoadFromReader(f)
}

// LoadFile is an alias for Load.
func LoadFile(path string) (*Schema, error) {
	return Load(path)
}

// Var pairs a variable name with its specification.
type Var struct {
	Name string
	VarSpec
}

// OrderedVars returns all variables sorted alphabetically by name.
func (s *Schema) OrderedVars() []Var {
	if s == nil || s.Vars == nil {
		return nil
	}
	names := make([]string, 0, len(s.Vars))
	for name := range s.Vars {
		names = append(names, name)
	}
	sort.Strings(names)

	vars := make([]Var, len(names))
	for i, name := range names {
		vars[i] = Var{
			Name:    name,
			VarSpec: s.Vars[name],
		}
	}
	return vars
}

// LoadFromReader parses an envguard schema from the supplied reader.
// After unmarshalling the YAML the schema is validated via Schema.Validate.
func LoadFromReader(r io.Reader) (*Schema, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("schema: read: %w", err)
	}

	var s Schema
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("schema: parse yaml: %w", err)
	}

	if err := s.Validate(); err != nil {
		return nil, err
	}

	return &s, nil
}

// Validate checks the schema for internal consistency. It verifies that every
// VarSpec uses a supported type and that every pattern is a valid regular
// expression. An error is returned listing all problems found, or nil if the
// schema is valid.
func (s *Schema) Validate() error {
	if s.Vars == nil {
		s.Vars = make(map[string]VarSpec)
	}
	// Merge variables into vars if both are present
	if s.Variables != nil {
		for k, v := range s.Variables {
			if _, exists := s.Vars[k]; !exists {
				s.Vars[k] = v
			}
		}
	}
	if len(s.Vars) == 0 {
		return fmt.Errorf("schema: no vars defined")
	}

	var errs []string

	// Sort keys for deterministic output.
	names := make([]string, 0, len(s.Vars))
	for name := range s.Vars {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		spec := s.Vars[name]

		// Default empty type to "string".
		if spec.Type == "" {
			spec.Type = "string"
			s.Vars[name] = spec
		}

		// Check type is supported.
		lower := strings.ToLower(spec.Type)
		if _, ok := supportedTypeSet[lower]; !ok {
			errs = append(errs, fmt.Sprintf("var %q: unsupported type %q (supported: %s)",
				name, spec.Type, strings.Join(supportedTypes, ", ")))
		} else if lower != spec.Type {
			// Normalise to lowercase.
			spec.Type = lower
			s.Vars[name] = spec
		}

		// Check pattern is valid regex.
		if spec.Pattern != "" {
			if _, err := regexp.Compile(spec.Pattern); err != nil {
				errs = append(errs, fmt.Sprintf("var %q: invalid pattern %q: %v",
					name, spec.Pattern, err))
			}
		}

		// Check min/max bounds.
		if spec.Min != nil && spec.Max != nil && *spec.Min > *spec.Max {
			errs = append(errs, fmt.Sprintf("var %q: min (%v) cannot be greater than max (%v)",
				name, *spec.Min, *spec.Max))
		}

		// Check min_length/max_length bounds.
		if spec.MinLength != nil && spec.MaxLength != nil && *spec.MinLength > *spec.MaxLength {
			errs = append(errs, fmt.Sprintf("var %q: min_length (%d) cannot be greater than max_length (%d)",
				name, *spec.MinLength, *spec.MaxLength))
		}

		// Validate allowed list doesn't contain empty strings.
		for i, v := range spec.Allowed {
			if v == "" {
				errs = append(errs, fmt.Sprintf("var %q: allowed[%d] is empty string",
					name, i))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("schema: validation failed:\n  %s", strings.Join(errs, "\n  "))
	}

	return nil
}
