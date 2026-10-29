# envguard — Node.js/TypeScript SDK

Runtime environment variable validation against `envguard.yaml` schemas.
Catch misconfigurations at startup, not in production.

## Installation

```bash
npm install envguard
```

## Quick Start

1. **Define your schema** in `envguard.yaml`:

```yaml
version: "1"
vars:
  DATABASE_URL:
    type: url
    required: true
    description: "PostgreSQL connection string"
  PORT:
    type: integer
    default: "8000"
  DEBUG:
    type: boolean
    default: "false"
  LOG_LEVEL:
    type: string
    allowed: ["debug", "info", "warning", "error"]
    default: "info"
```

2. **Validate at startup**:

```typescript
import { check } from 'envguard';

check();  // Throws ValidationError on failure
```

## API Reference

### `check(options?): ValidationResult`

Main validation function.

| Option        | Type     | Default            | Description                                          |
|---------------|----------|--------------------|------------------------------------------------------|
| `schemaPath`  | `string` | `"envguard.yaml"`  | Path to the schema file                              |
| `envSource`   | `string` | `"auto"`           | `"auto"`, `"file"` (.env only), or `"process"` (process.env only) |
| `envPath`     | `string` | `".env"`           | Path to the .env file                                |
| `strict`      | `boolean`| `false`            | Treat warnings as errors                             |
| `onError`     | `string` | `"throw"`          | `"throw"`, `"exit"`, or `"warn"`                     |

### `guard(options?): Express middleware`

Express/Connect middleware that validates env vars on the first request.
All options are forwarded to `check()`.

### `loadSchema(path): Schema`

Load and parse an `envguard.yaml` schema file.

### `parseDotenv(path): Record<string, string>`

Parse a `.env` file into an object. Handles comments, quotes, and `export`
prefix.

### `validate(schema, envVars): ValidationResult`

Validate an object of env vars against a loaded `Schema`.

### `ValidationResult`

| Property   | Type       | Description                      |
|------------|------------|----------------------------------|
| `errors`   | `Issue[]`  | Error-level issues               |
| `warnings` | `Issue[]`  | Warning-level issues             |
| `valid`    | `number`   | Count of valid variables         |
| `total`    | `number`   | Total variables in schema        |

Methods: `hasErrors()`, `summary()`.

### `ValidationError`

Extends `Error`. Thrown when validation fails. Has a `result` property
containing the `ValidationResult`.

## Type Validation

| Type      | Rule                                                       |
|-----------|------------------------------------------------------------|
| `string`  | Any string value                                           |
| `integer` | Matches `/^-?\d+$/`                                       |
| `float`   | `Number(value)` is not NaN                                 |
| `boolean` | Value is one of `true`, `false`, `1`, `0`, `yes`, `no`     |
| `url`     | `new URL(value)` succeeds with a protocol                  |
| `email`   | Matches `/^[^@]+@[^@]+\.[^@]+$/`                          |

## Express Integration

```typescript
import express from 'express';
import { guard } from 'envguard';

const app = express();

// Validates on the first request, then skips
app.use(guard());

app.get('/', (req, res) => {
  res.send('Hello!');
});

app.listen(3000);
```

## Error Handling Modes

```typescript
import { check } from 'envguard';

// Throw an exception (default)
check({ onError: 'throw' });

// Print warnings to stderr and continue
check({ onError: 'warn' });

// Exit the process with code 1
check({ onError: 'exit' });

// Strict mode: treat warnings as errors
check({ strict: true });
```

## License

MIT
