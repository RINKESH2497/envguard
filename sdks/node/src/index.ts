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

import * as fs from 'fs';
import * as path from 'path';
import * as yaml from 'js-yaml';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const BOOLEAN_VALUES = new Set([
  'true', 'false', '1', '0', 'yes', 'no',
]);

const VALID_TYPES = new Set([
  'string', 'integer', 'float', 'boolean', 'url', 'email',
]);

const VALID_ENV_SOURCES = new Set(['auto', 'file', 'process']);
const VALID_ON_ERROR = new Set(['throw', 'exit', 'warn']);

const EMAIL_RE = /^[^@]+@[^@]+\.[^@]+$/;

// ---------------------------------------------------------------------------
// Public interfaces
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// ValidationError
// ---------------------------------------------------------------------------

/** Thrown when environment variable validation fails. */
export class ValidationError extends Error {
  /** The full validation result. */
  public readonly result: ValidationResult;

  constructor(result: ValidationResult) {
    const lines: string[] = [result.summary()];
    for (const issue of [...result.errors, ...result.warnings]) {
      const tag = issue.severity === 'error' ? 'ERROR' : 'WARN';
      lines.push(`  [${tag}] ${issue.varName}: ${issue.message}`);
    }
    super(lines.join('\n'));
    this.name = 'ValidationError';
    this.result = result;

    // Maintain proper prototype chain for instanceof checks.
    Object.setPrototypeOf(this, ValidationError.prototype);
  }
}

// ---------------------------------------------------------------------------
// Schema loading
// ---------------------------------------------------------------------------

/**
 * Load and parse an `envguard.yaml` schema file.
 *
 * @param schemaPath - Filesystem path to the YAML schema file.
 * @returns A parsed {@link Schema} object.
 * @throws If the file does not exist or contains invalid YAML.
 */
export function loadSchema(schemaPath: string = 'envguard.yaml'): Schema {
  const resolved = path.resolve(schemaPath);

  if (!fs.existsSync(resolved)) {
    throw new Error(`Schema file not found: ${resolved}`);
  }

  const raw = fs.readFileSync(resolved, 'utf-8');
  const data = yaml.load(raw) as Record<string, unknown> | null;

  if (!data || typeof data !== 'object') {
    throw new Error('Schema must be a YAML mapping');
  }

  const version = String(data.version ?? '1');
  const varsSection =
    (data.vars as Record<string, unknown>) ??
    (data.variables as Record<string, unknown>) ??
    {};

  if (typeof varsSection !== 'object' || varsSection === null) {
    throw new Error("'vars' section in schema must be a mapping");
  }

  const vars: Record<string, VarSpec> = {};

  for (const [name, specRaw] of Object.entries(varsSection)) {
    vars[name] = parseVarSpec(name, (specRaw ?? {}) as Record<string, unknown>);
  }

  return { version, vars };
}

/**
 * Parse a raw YAML mapping into a {@link VarSpec}.
 * @internal
 */
function parseVarSpec(
  name: string,
  data: Record<string, unknown>,
): VarSpec {
  const varType = String(data.type ?? 'string');

  if (!VALID_TYPES.has(varType)) {
    throw new Error(
      `Variable "${name}": invalid type "${varType}"; must be one of ${[...VALID_TYPES].sort().join(', ')}`,
    );
  }

  let allowed: string[] | undefined;
  if (data.allowed !== undefined) {
    if (!Array.isArray(data.allowed)) {
      throw new Error(`Variable "${name}": 'allowed' must be an array`);
    }
    allowed = (data.allowed as unknown[]).map(String);
  }

  return {
    type: varType,
    required: data.required !== undefined ? Boolean(data.required) : true,
    default: data.default !== undefined ? String(data.default) : undefined,
    description: String(data.description ?? ''),
    pattern: data.pattern !== undefined ? String(data.pattern) : undefined,
    allowed,
    secret: Boolean(data.secret ?? false),
    example: data.example !== undefined ? String(data.example) : undefined,
  };
}

// ---------------------------------------------------------------------------
// .env parser
// ---------------------------------------------------------------------------

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
export function parseDotenv(envPath: string = '.env'): Record<string, string> {
  const resolved = path.resolve(envPath);
  const env: Record<string, string> = {};

  if (!fs.existsSync(resolved)) {
    return env;
  }

  const content = fs.readFileSync(resolved, 'utf-8');

  for (const rawLine of content.split(/\r?\n/)) {
    let line = rawLine.trim();

    // Skip empty lines and comments.
    if (!line || line.startsWith('#')) continue;

    // Strip optional `export` prefix.
    if (line.startsWith('export ')) {
      line = line.slice('export '.length);
    }

    // Locate the first `=`.
    const eqIndex = line.indexOf('=');
    if (eqIndex === -1) continue;

    const key = line.slice(0, eqIndex).trim();
    const valuePart = line.slice(eqIndex + 1).trim();

    if (!key) continue;

    env[key] = parseDotenvValue(valuePart);
  }

  return env;
}

/**
 * Extract the value portion of a `.env` line, respecting quotes.
 * @internal
 */
function parseDotenvValue(raw: string): string {
  if (!raw) return '';

  // Double-quoted value.
  if (raw.startsWith('"')) {
    const end = findClosingQuote(raw, '"');
    if (end !== -1) {
      let inner = raw.slice(1, end);
      inner = inner
        .replace(/\\n/g, '\n')
        .replace(/\\t/g, '\t')
        .replace(/\\"/g, '"')
        .replace(/\\\\/g, '\\');
      return inner;
    }
    return raw.slice(1);
  }

  // Single-quoted value — literal, no escapes.
  if (raw.startsWith("'")) {
    const end = findClosingQuote(raw, "'");
    if (end !== -1) {
      return raw.slice(1, end);
    }
    return raw.slice(1);
  }

  // Unquoted — strip inline comments.
  const commentIndex = raw.indexOf(' #');
  if (commentIndex !== -1) {
    return raw.slice(0, commentIndex).trimEnd();
  }
  return raw.trimEnd();
}

/**
 * Find the index of the closing quote in `s`, skipping escaped quotes inside
 * double-quoted strings.
 * @internal
 */
function findClosingQuote(s: string, quote: string): number {
  let i = 1;
  while (i < s.length) {
    if (s[i] === '\\' && quote === '"') {
      i += 2;
      continue;
    }
    if (s[i] === quote) {
      return i;
    }
    i++;
  }
  return -1;
}

// ---------------------------------------------------------------------------
// Type & value validation helpers
// ---------------------------------------------------------------------------

/**
 * Return an error message if `value` doesn't match `expectedType`, or
 * `undefined` if valid.
 * @internal
 */
function validateType(value: string, expectedType: string): string | undefined {
  switch (expectedType) {
    case 'string':
      return undefined;

    case 'integer': {
      if (!/^-?\d+$/.test(value)) {
        return `expected integer, got "${value}"`;
      }
      return undefined;
    }

    case 'float': {
      if (isNaN(Number(value)) || value.trim() === '') {
        return `expected float, got "${value}"`;
      }
      return undefined;
    }

    case 'boolean': {
      if (!BOOLEAN_VALUES.has(value.toLowerCase())) {
        return `expected boolean (true/false/1/0/yes/no), got "${value}"`;
      }
      return undefined;
    }

    case 'url': {
      try {
        const parsed = new URL(value);
        if (!parsed.protocol) {
          return `expected URL with scheme, got "${value}"`;
        }
      } catch {
        return `expected URL with scheme, got "${value}"`;
      }
      return undefined;
    }

    case 'email': {
      if (!EMAIL_RE.test(value)) {
        return `expected email address, got "${value}"`;
      }
      return undefined;
    }

    default:
      return `unknown type "${expectedType}"`;
  }
}

/**
 * Return an error message if `value` doesn't fully match `pattern`.
 * @internal
 */
function validatePattern(value: string, pattern: string): string | undefined {
  try {
    const re = new RegExp(`^(?:${pattern})$`);
    if (!re.test(value)) {
      return `does not match pattern "${pattern}"`;
    }
  } catch (err) {
    return `invalid regex pattern "${pattern}": ${err}`;
  }
  return undefined;
}

/**
 * Return an error message if `value` is not in the `allowed` list.
 * @internal
 */
function validateAllowed(value: string, allowed: string[]): string | undefined {
  if (!allowed.includes(value)) {
    return `value "${value}" not in allowed list [${allowed.map(v => `"${v}"`).join(', ')}]`;
  }
  return undefined;
}

// ---------------------------------------------------------------------------
// Core validation
// ---------------------------------------------------------------------------

/**
 * Validate `envVars` against the provided schema.
 *
 * @param schema - A parsed {@link Schema}.
 * @param envVars - A record of environment variable names to string values.
 * @returns A {@link ValidationResult} summarising errors and warnings.
 */
export function validate(
  schema: Schema,
  envVars: Record<string, string | undefined>,
): ValidationResult {
  const errors: Issue[] = [];
  const warnings: Issue[] = [];
  let valid = 0;
  const total = Object.keys(schema.vars).length;

  for (const [varName, spec] of Object.entries(schema.vars)) {
    const value = envVars[varName];

    // --- missing variable ------------------------------------------------
    if (value === undefined || value === '') {
      if (spec.default !== undefined) {
        warnings.push({
          varName,
          message: `not set; using default "${spec.default}"`,
          severity: 'warning',
        });
        valid++;
        continue;
      }

      if (spec.required) {
        errors.push({
          varName,
          message: 'required but not set',
          severity: 'error',
        });
      } else {
        valid++;
      }
      continue;
    }

    // --- type check ------------------------------------------------------
    const typeErr = validateType(value, spec.type);
    if (typeErr) {
      errors.push({ varName, message: typeErr, severity: 'error' });
      continue;
    }

    // --- pattern check ---------------------------------------------------
    if (spec.pattern !== undefined) {
      const patErr = validatePattern(value, spec.pattern);
      if (patErr) {
        errors.push({ varName, message: patErr, severity: 'error' });
        continue;
      }
    }

    // --- allowed list ----------------------------------------------------
    if (spec.allowed !== undefined) {
      const allowedErr = validateAllowed(value, spec.allowed);
      if (allowedErr) {
        errors.push({ varName, message: allowedErr, severity: 'error' });
        continue;
      }
    }

    valid++;
  }

  return createValidationResult(errors, warnings, valid, total);
}

/**
 * Build a {@link ValidationResult} with helper methods attached.
 * @internal
 */
function createValidationResult(
  errors: Issue[],
  warnings: Issue[],
  valid: number,
  total: number,
): ValidationResult {
  return {
    errors,
    warnings,
    valid,
    total,
    hasErrors() {
      return errors.length > 0;
    },
    summary() {
      const parts: string[] = [`${valid}/${total} variables valid`];
      if (errors.length > 0) parts.push(`${errors.length} error(s)`);
      if (warnings.length > 0) parts.push(`${warnings.length} warning(s)`);
      return parts.join(', ');
    },
  };
}

// ---------------------------------------------------------------------------
// Env var resolution
// ---------------------------------------------------------------------------

/**
 * Build the effective env-var dictionary according to `envSource`.
 * @internal
 */
function resolveEnvVars(
  envSource: string,
  envPath: string,
): Record<string, string | undefined> {
  if (envSource === 'file') {
    return parseDotenv(envPath);
  }

  if (envSource === 'process') {
    return { ...process.env };
  }

  // "auto": .env is the base, process.env overrides.
  const merged: Record<string, string | undefined> = parseDotenv(envPath);
  Object.assign(merged, process.env);
  return merged;
}

// ---------------------------------------------------------------------------
// Public API — check()
// ---------------------------------------------------------------------------

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
export function check(options?: CheckOptions): ValidationResult {
  const {
    schemaPath = 'envguard.yaml',
    envPath = '.env',
    envSource = 'auto',
    strict = false,
    onError = 'throw',
  } = options ?? {};

  if (!VALID_ENV_SOURCES.has(envSource)) {
    throw new Error(
      `envSource must be one of ${[...VALID_ENV_SOURCES].sort().join(', ')}, got "${envSource}"`,
    );
  }
  if (!VALID_ON_ERROR.has(onError)) {
    throw new Error(
      `onError must be one of ${[...VALID_ON_ERROR].sort().join(', ')}, got "${onError}"`,
    );
  }

  const schema = loadSchema(schemaPath);
  const envVars = resolveEnvVars(envSource, envPath);
  const result = validate(schema, envVars);

  // Strict mode: promote warnings to errors.
  if (strict && result.warnings.length > 0) {
    result.errors.push(...result.warnings);
    result.warnings.length = 0;
  }

  // Handle errors according to onError strategy.
  if (result.hasErrors()) {
    if (onError === 'throw') {
      throw new ValidationError(result);
    } else if (onError === 'exit') {
      console.error(`envguard: validation failed\n${result.summary()}`);
      for (const issue of [...result.errors, ...result.warnings]) {
        const tag = issue.severity === 'error' ? 'ERROR' : 'WARN';
        console.error(`  [${tag}] ${issue.varName}: ${issue.message}`);
      }
      process.exit(1);
    } else {
      // "warn"
      console.warn(`envguard: ${result.summary()}`);
      for (const issue of [...result.errors, ...result.warnings]) {
        const tag = issue.severity === 'error' ? 'ERROR' : 'WARN';
        console.warn(`  [${tag}] ${issue.varName}: ${issue.message}`);
      }
    }
  }

  return result;
}

// ---------------------------------------------------------------------------
// Public API — guard() Express middleware
// ---------------------------------------------------------------------------

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
export function guard(
  options?: CheckOptions,
): (req: unknown, res: unknown, next: (err?: Error) => void) => void {
  let validated = false;

  return (_req: unknown, _res: unknown, next: (err?: Error) => void) => {
    if (!validated) {
      try {
        check({ ...options, onError: 'throw' });
        validated = true;
      } catch (err) {
        next(err instanceof Error ? err : new Error(String(err)));
        return;
      }
    }
    next();
  };
}
