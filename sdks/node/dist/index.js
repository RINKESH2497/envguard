"use strict";
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
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
exports.ValidationError = void 0;
exports.loadSchema = loadSchema;
exports.parseDotenv = parseDotenv;
exports.validate = validate;
exports.check = check;
exports.guard = guard;
const fs = __importStar(require("fs"));
const path = __importStar(require("path"));
const yaml = __importStar(require("js-yaml"));
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
// ValidationError
// ---------------------------------------------------------------------------
/** Thrown when environment variable validation fails. */
class ValidationError extends Error {
    constructor(result) {
        const lines = [result.summary()];
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
exports.ValidationError = ValidationError;
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
function loadSchema(schemaPath = 'envguard.yaml') {
    const resolved = path.resolve(schemaPath);
    if (!fs.existsSync(resolved)) {
        throw new Error(`Schema file not found: ${resolved}`);
    }
    const raw = fs.readFileSync(resolved, 'utf-8');
    const data = yaml.load(raw);
    if (!data || typeof data !== 'object') {
        throw new Error('Schema must be a YAML mapping');
    }
    const version = String(data.version ?? '1');
    const varsSection = data.vars ??
        data.variables ??
        {};
    if (typeof varsSection !== 'object' || varsSection === null) {
        throw new Error("'vars' section in schema must be a mapping");
    }
    const vars = {};
    for (const [name, specRaw] of Object.entries(varsSection)) {
        vars[name] = parseVarSpec(name, (specRaw ?? {}));
    }
    return { version, vars };
}
/**
 * Parse a raw YAML mapping into a {@link VarSpec}.
 * @internal
 */
function parseVarSpec(name, data) {
    const varType = String(data.type ?? 'string');
    if (!VALID_TYPES.has(varType)) {
        throw new Error(`Variable "${name}": invalid type "${varType}"; must be one of ${[...VALID_TYPES].sort().join(', ')}`);
    }
    let allowed;
    if (data.allowed !== undefined) {
        if (!Array.isArray(data.allowed)) {
            throw new Error(`Variable "${name}": 'allowed' must be an array`);
        }
        allowed = data.allowed.map(String);
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
function parseDotenv(envPath = '.env') {
    const resolved = path.resolve(envPath);
    const env = {};
    if (!fs.existsSync(resolved)) {
        return env;
    }
    const content = fs.readFileSync(resolved, 'utf-8');
    for (const rawLine of content.split(/\r?\n/)) {
        let line = rawLine.trim();
        // Skip empty lines and comments.
        if (!line || line.startsWith('#'))
            continue;
        // Strip optional `export` prefix.
        if (line.startsWith('export ')) {
            line = line.slice('export '.length);
        }
        // Locate the first `=`.
        const eqIndex = line.indexOf('=');
        if (eqIndex === -1)
            continue;
        const key = line.slice(0, eqIndex).trim();
        const valuePart = line.slice(eqIndex + 1).trim();
        if (!key)
            continue;
        env[key] = parseDotenvValue(valuePart);
    }
    return env;
}
function unescapeDoubleQuoted(str) {
    let result = '';
    let i = 0;
    while (i < str.length) {
        if (str[i] === '\\' && i + 1 < str.length) {
            const next = str[i + 1];
            if (next === 'n') {
                result += '\n';
            }
            else if (next === 't') {
                result += '\t';
            }
            else if (next === '"') {
                result += '"';
            }
            else if (next === '\\') {
                result += '\\';
            }
            else {
                result += next;
            }
            i += 2;
        }
        else {
            result += str[i];
            i += 1;
        }
    }
    return result;
}
/**
 * Extract the value portion of a `.env` line, respecting quotes.
 * @internal
 */
function parseDotenvValue(raw) {
    if (!raw)
        return '';
    // Double-quoted value.
    if (raw.startsWith('"')) {
        const end = findClosingQuote(raw, '"');
        if (end !== -1) {
            const inner = raw.slice(1, end);
            return unescapeDoubleQuoted(inner);
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
function findClosingQuote(s, quote) {
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
function validateType(value, expectedType) {
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
            }
            catch {
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
function validatePattern(value, pattern) {
    try {
        const re = new RegExp(`^(?:${pattern})$`);
        if (!re.test(value)) {
            return `does not match pattern "${pattern}"`;
        }
    }
    catch (err) {
        return `invalid regex pattern "${pattern}": ${err}`;
    }
    return undefined;
}
/**
 * Return an error message if `value` is not in the `allowed` list.
 * @internal
 */
function validateAllowed(value, allowed) {
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
function validate(schema, envVars) {
    const errors = [];
    const warnings = [];
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
            }
            else {
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
function createValidationResult(errors, warnings, valid, total) {
    return {
        errors,
        warnings,
        valid,
        total,
        hasErrors() {
            return errors.length > 0;
        },
        summary() {
            const parts = [`${valid}/${total} variables valid`];
            if (errors.length > 0)
                parts.push(`${errors.length} error(s)`);
            if (warnings.length > 0)
                parts.push(`${warnings.length} warning(s)`);
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
function resolveEnvVars(envSource, envPath) {
    if (envSource === 'file') {
        return parseDotenv(envPath);
    }
    if (envSource === 'process') {
        return { ...process.env };
    }
    // "auto": .env is the base, process.env overrides.
    const merged = parseDotenv(envPath);
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
function check(options) {
    const { schemaPath = 'envguard.yaml', envPath = '.env', envSource = 'auto', strict = false, onError = 'throw', } = options ?? {};
    if (!VALID_ENV_SOURCES.has(envSource)) {
        throw new Error(`envSource must be one of ${[...VALID_ENV_SOURCES].sort().join(', ')}, got "${envSource}"`);
    }
    if (!VALID_ON_ERROR.has(onError)) {
        throw new Error(`onError must be one of ${[...VALID_ON_ERROR].sort().join(', ')}, got "${onError}"`);
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
        }
        else if (onError === 'exit') {
            console.error(`envguard: validation failed\n${result.summary()}`);
            for (const issue of [...result.errors, ...result.warnings]) {
                const tag = issue.severity === 'error' ? 'ERROR' : 'WARN';
                console.error(`  [${tag}] ${issue.varName}: ${issue.message}`);
            }
            process.exit(1);
        }
        else {
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
function guard(options) {
    let validated = false;
    return (_req, _res, next) => {
        if (!validated) {
            try {
                check({ ...options, onError: 'throw' });
                validated = true;
            }
            catch (err) {
                next(err instanceof Error ? err : new Error(String(err)));
                return;
            }
        }
        next();
    };
}
//# sourceMappingURL=index.js.map