/**
 * envguard — Runtime environment variable validation for Node.js/TypeScript.
 *
 * Validates `process.env` against an `envguard.yaml` schema at application
 * startup, catching misconfigurations before they surface in production.
 *
 * @example
 * ```ts
 * import { check, guard } from 'envguard';
 *
 * // Quick validation — throws on error
 * check();
 *
 * // With options
 * check({
 *   schemaPath: 'envguard.yaml',
 *   envPath: '.env',
 *   envSource: 'auto',
 *   strict: false,
 *   onError: 'throw',
 * });
 *
 * // Express middleware
 * app.use(guard());
 * ```
 *
 * @packageDocumentation
 */
/** Specification for a single environment variable in the schema. */
export interface VarSpec {
    /** Expected value type. Defaults to `"string"`. */
    type: string;
    /** Whether the variable must be present. Defaults to `true`. */
    required: boolean;
    /** Fallback value when the variable is not set. */
    default?: string;
    /** Human-readable description of the variable's purpose. */
    description: string;
    /** Regular expression the value must match (full match). */
    pattern?: string;
    /** Exhaustive list of accepted values. */
    allowed?: string[];
    /** Whether the value is sensitive and should be masked in logs. */
    secret: boolean;
    /** Example value shown in documentation / error messages. */
    example?: string;
}
/** Parsed `envguard.yaml` schema. */
export interface Schema {
    /** Schema format version. */
    version: string;
    /** Mapping of variable names to their specifications. */
    vars: Record<string, VarSpec>;
}
/** A single validation issue (error or warning). */
export interface Issue {
    /** Name of the environment variable. */
    varName: string;
    /** Human-readable description of the problem. */
    message: string;
    /** `"error"` or `"warning"`. */
    severity: 'error' | 'warning';
}
/** Aggregate result of environment variable validation. */
export interface ValidationResult {
    /** Error-level issues. */
    errors: Issue[];
    /** Warning-level issues. */
    warnings: Issue[];
    /** Count of variables that passed validation. */
    valid: number;
    /** Total number of variables declared in the schema. */
    total: number;
    /** Returns `true` if any errors are present. */
    hasErrors(): boolean;
    /** Returns a human-readable summary string. */
    summary(): string;
}
/** Options accepted by {@link check} and {@link guard}. */
export interface CheckOptions {
    /** Path to the `envguard.yaml` schema file. Defaults to `"envguard.yaml"`. */
    schemaPath?: string;
    /** Path to the `.env` file. Defaults to `".env"`. */
    envPath?: string;
    /**
     * Where to read environment variables from.
     * - `"auto"` — merge `.env` file values under `process.env` (process wins).
     * - `"file"` — use only the `.env` file.
     * - `"process"` — use only `process.env`.
     *
     * Defaults to `"auto"`.
     */
    envSource?: 'auto' | 'file' | 'process';
    /** Treat warnings as errors. Defaults to `false`. */
    strict?: boolean;
    /**
     * Action on validation failure.
     * - `"throw"` — throw a {@link ValidationError}.
     * - `"exit"` — call `process.exit(1)`.
     * - `"warn"` — print to stderr and continue.
     *
     * Defaults to `"throw"`.
     */
    onError?: 'throw' | 'exit' | 'warn';
}
/** Thrown when environment variable validation fails. */
export declare class ValidationError extends Error {
    /** The full validation result. */
    readonly result: ValidationResult;
    constructor(result: ValidationResult);
}
/**
 * Load and parse an `envguard.yaml` schema file.
 *
 * @param schemaPath - Filesystem path to the YAML schema file.
 * @returns A parsed {@link Schema} object.
 * @throws If the file does not exist or contains invalid YAML.
 */
export declare function loadSchema(schemaPath?: string): Schema;
/**
 * Parse a `.env` file into a string-keyed record.
 *
 * Handles:
 * - Blank lines and `#` comments
 * - `export` prefix (`export KEY=value`)
 * - Single-quoted, double-quoted, and unquoted values
 * - Inline comments for unquoted values
 *
 * @param envPath - Path to the `.env` file.
 * @returns A record mapping variable names to string values.
 */
export declare function parseDotenv(envPath?: string): Record<string, string>;
/**
 * Validate `envVars` against the provided schema.
 *
 * @param schema - A parsed {@link Schema}.
 * @param envVars - A record of environment variable names to string values.
 * @returns A {@link ValidationResult} summarising errors and warnings.
 */
export declare function validate(schema: Schema, envVars: Record<string, string | undefined>): ValidationResult;
/**
 * Validate environment variables against an `envguard.yaml` schema.
 *
 * This is the primary entry-point for the library. Call it at application
 * startup to surface misconfiguration before any request is served.
 *
 * @param options - Validation options. All fields are optional.
 * @returns A {@link ValidationResult}.
 * @throws {@link ValidationError} if validation fails and `onError` is
 *   `"throw"` (the default).
 */
export declare function check(options?: CheckOptions): ValidationResult;
/**
 * Express middleware that validates env vars on the first incoming request.
 *
 * @example
 * ```ts
 * import express from 'express';
 * import { guard } from 'envguard';
 *
 * const app = express();
 * app.use(guard());
 * ```
 *
 * @param options - Validation options forwarded to {@link check}.
 * @returns An Express-compatible middleware function.
 */
export declare function guard(options?: CheckOptions): (req: unknown, res: unknown, next: (err?: Error) => void) => void;
//# sourceMappingURL=index.d.ts.map