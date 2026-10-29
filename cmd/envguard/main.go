package main

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/RINKESH2497/envguard/internal/dotenv"
	"github.com/RINKESH2497/envguard/internal/envsync"
	"github.com/RINKESH2497/envguard/internal/interactive"
	"github.com/RINKESH2497/envguard/internal/scanner"
	"github.com/RINKESH2497/envguard/internal/schema"
	"github.com/RINKESH2497/envguard/internal/validator"
)

var (
	// version is populated at build time via -ldflags="-X main.version=..."
	version = "0.1.0"

	// Global flags
	configPath string
	quiet      bool
	noColor    bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "envguard",
	Short: "envguard — Stop deploying with missing or invalid environment variables",
	Long: `envguard is a schema-driven environment variable validator, secret scanner,
and developer onboarding tool. Keep your .env files in sync, catch missing secrets
in CI, and validate configurations with zero guesswork.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func init() {
	rootCmd.Version = version
	rootCmd.PersistentFlags().StringVarP(&configPath, "config", "c", "envguard.yaml", "Path to schema configuration file")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(onboardCmd)
	rootCmd.AddCommand(versionCmd)
}

// -----------------------------------------------------------------------------
// Color & Formatting Helpers
// -----------------------------------------------------------------------------

func shouldColor() bool {
	if noColor || os.Getenv("NO_COLOR") != "" {
		return false
	}
	return true
}

func colorGreen(s string) string {
	if !shouldColor() {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}

func colorRed(s string) string {
	if !shouldColor() {
		return s
	}
	return "\033[31m" + s + "\033[0m"
}

func colorYellow(s string) string {
	if !shouldColor() {
		return s
	}
	return "\033[33m" + s + "\033[0m"
}

func colorCyan(s string) string {
	if !shouldColor() {
		return s
	}
	return "\033[36m" + s + "\033[0m"
}

func colorBold(s string) string {
	if !shouldColor() {
		return s
	}
	return "\033[1m" + s + "\033[0m"
}

// -----------------------------------------------------------------------------
// Command: init
// -----------------------------------------------------------------------------

var (
	initEnvPath string
	initForce   bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create an envguard.yaml schema by inspecting existing .env or scaffolding",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !initForce {
			if _, err := os.Stat(configPath); err == nil {
				return fmt.Errorf("%s already exists; use --force to overwrite", configPath)
			}
		}

		sch := &schema.Schema{
			Version: "1",
			Vars:    make(map[string]schema.VarSpec),
		}

		varsFound := 0
		if envData, err := dotenv.Parse(initEnvPath); err == nil && len(envData) > 0 {
			if !quiet {
				fmt.Printf("🔍 Inspecting %s to infer schema...\n", colorCyan(initEnvPath))
			}

			// Sort keys for deterministic output
			keys := make([]string, 0, len(envData))
			for k := range envData {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, k := range keys {
				val := envData[k]
				inferredType := inferType(val)
				isSecret := inferSecret(k)

				spec := schema.VarSpec{
					Type:        inferredType,
					Required:    true,
					Description: fmt.Sprintf("%s configuration variable", k),
					Secret:      isSecret,
				}
				if strings.HasPrefix(val, "postgres://") || strings.HasPrefix(val, "postgresql://") {
					spec.Pattern = `^postgres(ql)?://.+`
				}
				if !isSecret && val != "" {
					spec.Example = val
				}
				sch.Vars[k] = spec
				varsFound++
			}
		} else {
			if !quiet {
				fmt.Printf("ℹ️  No existing %s found. Scaffolding default schema...\n", initEnvPath)
			}
			sch.Vars["PORT"] = schema.VarSpec{
				Type:        "integer",
				Required:    false,
				Default:     "3000",
				Description: "Server listening port",
			}
			sch.Vars["NODE_ENV"] = schema.VarSpec{
				Type:        "string",
				Required:    true,
				Description: "Application runtime environment",
				Allowed:     []string{"development", "staging", "production"},
			}
			sch.Vars["DATABASE_URL"] = schema.VarSpec{
				Type:        "string",
				Required:    true,
				Description: "Database connection URL",
				Pattern:     `^postgres://`,
				Secret:      true,
			}
			varsFound = 3
		}

		data, err := yaml.Marshal(sch)
		if err != nil {
			return fmt.Errorf("marshal schema: %w", err)
		}

		if err := os.WriteFile(configPath, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", configPath, err)
		}

		if !quiet {
			fmt.Printf("%s Successfully generated %s with %d variables!\n",
				colorGreen("✓"), colorBold(configPath), varsFound)
			fmt.Println("Next steps:")
			fmt.Println("  1. Review and refine your schema in", colorCyan(configPath))
			fmt.Println("  2. Run", colorCyan("envguard sync"), "to generate .env.example")
			fmt.Println("  3. Run", colorCyan("envguard check"), "to validate your environment")
		}
		return nil
	},
}

func init() {
	initCmd.Flags().StringVarP(&initEnvPath, "env", "e", ".env", "Source .env file to inspect")
	initCmd.Flags().BoolVarP(&initForce, "force", "f", false, "Overwrite existing schema file")
}

func inferType(val string) string {
	lower := strings.ToLower(val)
	if lower == "true" || lower == "false" || lower == "yes" || lower == "no" {
		return "boolean"
	}
	if _, err := strconv.Atoi(val); err == nil {
		return "integer"
	}
	if _, err := strconv.ParseFloat(val, 64); err == nil && strings.Contains(val, ".") {
		return "float"
	}
	if u, err := url.Parse(val); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		return "url"
	}
	if strings.Contains(val, "@") && strings.Contains(val, ".") && !strings.Contains(val, " ") {
		return "email"
	}
	return "string"
}

func inferSecret(key string) bool {
	upper := strings.ToUpper(key)
	secretKeywords := []string{
		"SECRET", "KEY", "TOKEN", "PASSWORD", "PASSWD",
		"AUTH", "CREDENTIAL", "PRIVATE", "CERT", "SIGNING",
	}
	for _, kw := range secretKeywords {
		if strings.Contains(upper, kw) {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Command: check
// -----------------------------------------------------------------------------

var (
	checkEnvPath   string
	checkStrict    bool
	checkEnvSource string
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Validate environment variables against the schema",
	Run: func(cmd *cobra.Command, args []string) {
		opts := validator.Options{
			SchemaPath: configPath,
			StrictMode: checkStrict,
		}

		switch checkEnvSource {
		case "file":
			opts.EnvPath = checkEnvPath
		case "env":
			opts.EnvVars = envFromProcess()
			opts.IgnoreUndocumented = true
		case "auto":
			merged := make(map[string]string)
			if fVars, err := dotenv.Parse(checkEnvPath); err == nil {
				for k, v := range fVars {
					merged[k] = v
				}
			}
			pVars := envFromProcess()
			for k, v := range pVars {
				merged[k] = v
			}
			opts.EnvVars = merged
			opts.IgnoreUndocumented = true
		default:
			fmt.Fprintf(os.Stderr, "Error: invalid --env-source %q (must be 'file', 'env', or 'auto')\n", checkEnvSource)
			os.Exit(1)
		}

		result, err := validator.Validate(opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s %v\n", colorRed("✗ Error:"), err)
			os.Exit(1)
		}

		if !quiet {
			for _, issue := range result.Errors {
				fmt.Printf("%s [%s] %s\n", colorRed("✗"), colorBold(issue.VarName), issue.Message)
			}
			for _, issue := range result.Warnings {
				fmt.Printf("%s [%s] %s\n", colorYellow("⚠"), colorBold(issue.VarName), issue.Message)
			}
			fmt.Println()
			fmt.Println(result.Summary())
		}

		os.Exit(result.ExitCode())
	},
}

func init() {
	checkCmd.Flags().StringVarP(&checkEnvPath, "env", "e", ".env", "Path to .env file")
	checkCmd.Flags().BoolVar(&checkStrict, "strict", false, "Treat warnings as errors")
	checkCmd.Flags().StringVar(&checkEnvSource, "env-source", "file", "Source of environment variables ('file', 'env', 'auto')")
}

func envFromProcess() map[string]string {
	out := make(map[string]string)
	for _, entry := range os.Environ() {
		idx := strings.IndexByte(entry, '=')
		if idx > 0 {
			out[entry[:idx]] = entry[idx+1:]
		}
	}
	return out
}

// -----------------------------------------------------------------------------
// Command: sync
// -----------------------------------------------------------------------------

var (
	syncOutputPath          string
	syncIncludeDescriptions bool
	syncIncludeTypes        bool
	syncGroupByRequired     bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Generate or update .env.example from schema",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := envsync.Options{
			SchemaPath:          configPath,
			OutputPath:          syncOutputPath,
			IncludeDescriptions: syncIncludeDescriptions,
			IncludeTypes:        syncIncludeTypes,
			GroupByRequired:     syncGroupByRequired,
		}

		res, err := envsync.Sync(opts)
		if err != nil {
			return err
		}

		if !quiet {
			action := "Updated"
			if res.Created {
				action = "Created"
			}
			fmt.Printf("%s %s %s\n", colorGreen("✓"), action, colorBold(syncOutputPath))
			if len(res.Added) > 0 {
				fmt.Printf("   Added:   %s\n", strings.Join(res.Added, ", "))
			}
			if len(res.Removed) > 0 {
				fmt.Printf("   Removed: %s\n", strings.Join(res.Removed, ", "))
			}
			fmt.Printf("   Unchanged: %d variables\n", res.Unchanged)
		}
		return nil
	},
}

func init() {
	syncCmd.Flags().StringVarP(&syncOutputPath, "output", "o", ".env.example", "Path to output .env.example file")
	syncCmd.Flags().BoolVar(&syncIncludeDescriptions, "descriptions", true, "Include descriptions as comments")
	syncCmd.Flags().BoolVar(&syncIncludeTypes, "types", true, "Include type and constraint annotations")
	syncCmd.Flags().BoolVar(&syncGroupByRequired, "group-required", false, "Group variables into Required and Optional sections")
}

// -----------------------------------------------------------------------------
// Command: scan
// -----------------------------------------------------------------------------

var (
	scanRecursive bool
	scanEntropy   float64
	scanExclude   []string
)

var scanCmd = &cobra.Command{
	Use:   "scan [paths...]",
	Short: "Scan files for leaked secrets and high-entropy tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths := args
		if len(paths) == 0 {
			paths = []string{"."}
		}

		opts := scanner.Options{
			Paths:            paths,
			Recursive:        scanRecursive,
			EntropyThreshold: scanEntropy,
			ExcludePatterns:  scanExclude,
		}

		res, err := scanner.Scan(opts)
		if err != nil {
			return err
		}

		if !quiet {
			for _, f := range res.Findings {
				sev := colorYellow("[" + strings.ToUpper(f.Severity) + "]")
				if f.Severity == "high" {
					sev = colorRed("[" + strings.ToUpper(f.Severity) + "]")
				}
				fmt.Printf("%s %s:%d — %s (%s)\n", sev, f.FilePath, f.Line, f.Pattern, f.Match)
			}
			fmt.Println()
			fmt.Printf("Scanned %d file(s) — found %d secret(s)\n", res.FilesScanned, res.SecretsFound)
		}

		if res.SecretsFound > 0 {
			os.Exit(1)
		}
		return nil
	},
}

func init() {
	scanCmd.Flags().BoolVarP(&scanRecursive, "recursive", "r", true, "Recursively scan directories")
	scanCmd.Flags().Float64Var(&scanEntropy, "entropy", 4.5, "Shannon entropy threshold for random secret detection")
	scanCmd.Flags().StringSliceVar(&scanExclude, "exclude", nil, "Glob patterns to exclude from scanning")
}

// -----------------------------------------------------------------------------
// Command: onboard
// -----------------------------------------------------------------------------

var (
	onboardEnvPath      string
	onboardSkipOptional bool
	onboardUseDefaults  bool
)

var onboardCmd = &cobra.Command{
	Use:   "onboard",
	Short: "Interactive setup wizard for new project contributors",
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := interactive.Options{
			SchemaPath:   configPath,
			OutputPath:   onboardEnvPath,
			SkipOptional: onboardSkipOptional,
			UseDefaults:  onboardUseDefaults,
		}

		res, err := interactive.Run(opts)
		if err != nil {
			return err
		}

		if !quiet {
			fmt.Printf("\n%s Onboarding complete! Written to %s (Set: %d, Skipped: %d)\n",
				colorGreen("✓"), colorBold(res.OutputPath), res.VarsSet, res.VarsSkipped)
		}
		return nil
	},
}

func init() {
	onboardCmd.Flags().StringVarP(&onboardEnvPath, "env", "e", ".env", "Output path for the generated .env file")
	onboardCmd.Flags().BoolVar(&onboardSkipOptional, "skip-optional", false, "Skip optional variables")
	onboardCmd.Flags().BoolVar(&onboardUseDefaults, "use-defaults", false, "Accept default values automatically")
}

// -----------------------------------------------------------------------------
// Command: version
// -----------------------------------------------------------------------------

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print envguard version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("envguard version %s\n", version)
	},
}
