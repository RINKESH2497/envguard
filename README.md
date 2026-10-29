<p align="center">
  <!-- TODO: Replace with actual logo -->
  <img src="https://via.placeholder.com/200x200?text=envguard" alt="envguard logo" width="200"/>
  <br/>
  <strong>envguard</strong>
  <br/>
  <em>Stop deploying with missing env vars.</em>
</p>

<p align="center">
  <a href="https://goreportcard.com/report/github.com/envguard/envguard"><img src="https://goreportcard.com/badge/github.com/envguard/envguard" alt="Go Report Card"></a>
  <a href="https://github.com/envguard/envguard/actions/workflows/ci.yml"><img src="https://github.com/envguard/envguard/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/envguard/envguard/releases/latest"><img src="https://img.shields.io/github/v/release/envguard/envguard" alt="Release"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue.svg" alt="License: MIT"></a>
</p>

---

## The Problem

Environment variables are the #1 source of "works on my machine" bugs:

- **Missing vars crash production** — a forgotten `DATABASE_URL` takes down your app at 2 AM.
- **No single source of truth** — `.env.example` drifts out of sync with what the app actually needs.
- **Secrets leak into logs and repos** — without guardrails, API keys end up in places they shouldn't.
- **Onboarding is painful** — new developers spend hours figuring out which vars to set and what format they need.

## The Solution

**envguard** is a schema-driven CLI that validates, syncs, and secures your environment variables — before your code ever runs.

- 📋 **Schema-driven validation** — Define every variable's type, constraints, and defaults in a single `envguard.yaml` file. Validate on every build.
- 🔄 **`.env.example` sync** — Automatically keep your `.env.example` in lockstep with your schema. No more stale templates.
- 🔐 **Secret scanning** — Detect leaked secrets in `.env` files, shell history, and CI logs before they reach version control.
- 🚀 **Interactive onboarding** — `envguard onboard` walks new developers through every required variable with prompts, descriptions, and sensible defaults.

---

## Quick Start

### 1. Install

**Go install:**

```bash
go install github.com/envguard/envguard/cmd/envguard@latest
```

**Homebrew:**

```bash
brew install envguard/tap/envguard
```

**Binary releases:**

Download pre-built binaries from the [Releases](https://github.com/envguard/envguard/releases) page for Linux, macOS, and Windows (amd64/arm64).

### 2. Init

Generate an `envguard.yaml` schema from your existing `.env` file:

```bash
envguard init
```

This inspects your `.env`, infers types, and produces a ready-to-customize schema.

### 3. Check

Validate your environment against the schema:

```bash
envguard check
```

```
✓ DATABASE_URL    valid
✓ PORT            valid (default: 3000)
✗ API_KEY         MISSING — required variable not set
✓ NODE_ENV        valid

1 error, 0 warnings — validation failed
```

---

## Commands

| Command              | Description                                                        |
| -------------------- | ------------------------------------------------------------------ |
| `envguard init`      | Generate `envguard.yaml` from an existing `.env` file              |
| `envguard check`     | Validate current environment against the schema                    |
| `envguard sync`      | Sync `.env.example` with the schema (add missing, remove stale)    |
| `envguard scan`      | Scan files and directories for leaked secrets                      |
| `envguard onboard`   | Interactive wizard to populate a `.env` for new developers         |

### Global Flags

| Flag               | Description                              | Default           |
| ------------------ | ---------------------------------------- | ----------------- |
| `--config`         | Path to schema file                      | `envguard.yaml`   |
| `--env`            | Path to `.env` file                      | `.env`            |
| `--strict`         | Treat warnings as errors                 | `false`           |
| `--format`         | Output format (`text`, `json`, `github`) | `text`            |
| `--no-color`       | Disable colored output                   | `false`           |

---

## Schema Reference

The `envguard.yaml` file is the single source of truth for your environment. Here's the full specification:

```yaml
# envguard.yaml — full schema reference
version: "1"

# Optional metadata
meta:
  name: my-app
  description: Environment schema for My Application

variables:
  # Required string with regex pattern validation
  DATABASE_URL:
    type: string
    required: true
    description: "PostgreSQL connection string"
    pattern: "^postgres://.+"
    secret: true

  # URL type with automatic format validation
  REDIS_URL:
    type: url
    required: false
    default: "redis://localhost:6379"
    description: "Redis connection URL"

  # Integer with range constraints
  PORT:
    type: integer
    required: false
    default: 3000
    description: "HTTP server port"
    min: 1
    max: 65535

  # Enum — only specific values allowed
  NODE_ENV:
    type: string
    required: true
    description: "Application environment"
    allowed:
      - development
      - staging
      - production

  # Secret — masked in logs, flagged by scan
  API_KEY:
    type: string
    required: true
    description: "External API key"
    secret: true

  # Boolean with default
  ENABLE_CACHE:
    type: boolean
    required: false
    default: true
    description: "Enable Redis caching layer"

  # String with example value for onboarding
  CORS_ORIGINS:
    type: string
    required: false
    description: "Comma-separated allowed origins"
    example: "https://example.com,https://app.example.com"
```

### Field Types

| Type      | Description                          | Validation                       |
| --------- | ------------------------------------ | -------------------------------- |
| `string`  | Any string value                     | `pattern`, `allowed`, `min_length`, `max_length` |
| `integer` | Whole number                         | `min`, `max`                     |
| `float`   | Decimal number                       | `min`, `max`                     |
| `boolean` | `true`/`false`, `1`/`0`, `yes`/`no` | Automatic parsing                |
| `url`     | Valid URL                            | Format validation                |
| `email`   | Valid email address                  | Format validation                |

### Variable Properties

| Property      | Type       | Description                                          |
| ------------- | ---------- | ---------------------------------------------------- |
| `type`        | `string`   | Data type (see above)                                |
| `required`    | `bool`     | Whether the variable must be set (default: `false`)  |
| `default`     | `any`      | Default value when not set                           |
| `description` | `string`   | Human-readable description                           |
| `secret`      | `bool`     | Mask in output, flag in scans (default: `false`)     |
| `pattern`     | `string`   | Regex pattern for string validation                  |
| `allowed`     | `[]string` | List of allowed values (enum)                        |
| `example`     | `string`   | Example value shown during onboarding                |
| `min`         | `number`   | Minimum value (integer/float)                        |
| `max`         | `number`   | Maximum value (integer/float)                        |
| `min_length`  | `int`      | Minimum string length                                |
| `max_length`  | `int`      | Maximum string length                                |
| `deprecated`  | `string`   | Deprecation notice — emits a warning if set          |

---

## Runtime SDKs

envguard also provides runtime validation SDKs so your application can enforce the schema at startup — not just in CI.

### Python

```python
from envguard import load_env

# Validates on load — raises EnvGuardError on failure
config = load_env("envguard.yaml")

print(config.DATABASE_URL)  # type-safe access
print(config.PORT)          # int, already parsed
```

```bash
pip install envguard
```

### Node.js

```typescript
import { loadEnv } from '@envguard/sdk';

// Validates and returns typed config — throws on failure
const config = loadEnv('envguard.yaml');

console.log(config.DATABASE_URL); // string
console.log(config.PORT);         // number
```

```bash
npm install @envguard/sdk
```

---

## GitHub Action

Use envguard in your CI pipeline with the official GitHub Action:

```yaml
# .github/workflows/ci.yml
- name: Validate environment
  uses: envguard/envguard@v1
  with:
    command: check
    schema-path: envguard.yaml
    env-path: .env.ci
    strict: true
```

### Inputs

| Input          | Description                                    | Default           |
| -------------- | ---------------------------------------------- | ----------------- |
| `command`      | Command to run (`check`, `sync`, `scan`)       | `check`           |
| `schema-path`  | Path to `envguard.yaml`                        | `envguard.yaml`   |
| `env-path`     | Path to `.env` file                            | `.env`            |
| `strict`       | Treat warnings as errors                       | `false`           |
| `version`      | envguard version to install                    | `latest`          |

---

## Pre-commit Hook

Add envguard to your [pre-commit](https://pre-commit.com/) config:

```yaml
# .pre-commit-config.yaml
repos:
  - repo: https://github.com/envguard/envguard
    rev: v1.0.0
    hooks:
      - id: envguard-check
        name: envguard check
        entry: envguard check --strict
        language: golang
        always_run: true
        pass_filenames: false
      - id: envguard-scan
        name: envguard scan
        entry: envguard scan
        language: golang
        always_run: true
        pass_filenames: false
```

Or add it manually to `.git/hooks/pre-commit`:

```bash
#!/bin/sh
envguard check --strict
envguard scan
```

---

## CI Integration

### GitHub Actions (full example)

```yaml
name: CI
on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  validate-env:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.23"
      - run: go install github.com/envguard/envguard/cmd/envguard@latest
      - run: envguard check --config envguard.yaml --env .env.ci --strict
      - run: envguard scan .
```

### GitLab CI

```yaml
validate-env:
  image: golang:1.23
  script:
    - go install github.com/envguard/envguard/cmd/envguard@latest
    - envguard check --strict
    - envguard scan .
```

### Docker

```dockerfile
# Multi-stage: install envguard
FROM golang:1.23-alpine AS envguard
RUN go install github.com/envguard/envguard/cmd/envguard@latest

FROM your-app-image
COPY --from=envguard /go/bin/envguard /usr/local/bin/envguard
COPY envguard.yaml .
RUN envguard check --strict
```

---

## Contributing

We welcome contributions! Please see [CONTRIBUTING.md](./CONTRIBUTING.md) for guidelines on:

- Reporting bugs and requesting features
- Setting up your development environment
- Code style and PR process

---

## License

envguard is released under the [MIT License](./LICENSE).

Copyright (c) 2024 envguard contributors
