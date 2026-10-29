# envguard — Python SDK

Runtime environment variable validation against `envguard.yaml` schemas.
Catch misconfigurations at startup, not in production.

## Installation

```bash
pip install py-envguard
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

```python
import envguard

envguard.check()  # Raises ValidationError on failure
```

## API Reference

### `envguard.check(**kwargs) -> ValidationResult`

Main validation function.

| Parameter       | Type   | Default            | Description                                        |
|-----------------|--------|--------------------|----------------------------------------------------|
| `schema_path`   | `str`  | `"envguard.yaml"`  | Path to the schema file                            |
| `env_source`    | `str`  | `"auto"`           | `"auto"`, `"file"` (.env only), or `"os"` (os.environ only) |
| `env_path`      | `str`  | `".env"`           | Path to the .env file                              |
| `strict`        | `bool` | `False`            | Treat warnings as errors                           |
| `on_error`      | `str`  | `"raise"`          | `"raise"`, `"exit"`, or `"warn"`                   |

### `envguard.guard(**kwargs) -> decorator`

Decorator that validates env vars before the decorated function runs. All
keyword arguments are forwarded to `check()`.

### `envguard.load_schema(path) -> Schema`

Load and parse an `envguard.yaml` schema file.

### `envguard.parse_dotenv(path) -> dict`

Parse a `.env` file into a `dict`. Handles comments, quotes, and `export`
prefix.

### `envguard.validate(schema, env_vars) -> ValidationResult`

Validate a dictionary of env vars against a loaded `Schema`.

### `ValidationResult`

| Attribute   | Type          | Description                      |
|-------------|---------------|----------------------------------|
| `errors`    | `list[Issue]` | Error-level issues               |
| `warnings`  | `list[Issue]` | Warning-level issues             |
| `valid`     | `int`         | Count of valid variables         |
| `total`     | `int`         | Total variables in schema        |

Methods: `has_errors()`, `has_warnings()`, `summary()`.

### `ValidationError`

Raised when validation fails. Has a `result` attribute containing the
`ValidationResult`.

## Type Validation

| Type      | Rule                                                       |
|-----------|------------------------------------------------------------|
| `string`  | Any string value                                           |
| `integer` | `int(value)` succeeds                                      |
| `float`   | `float(value)` succeeds                                    |
| `boolean` | Value is one of `true`, `false`, `1`, `0`, `yes`, `no`     |
| `url`     | Parsed URL has a scheme                                    |
| `email`   | Matches `^[^@]+@[^@]+\.[^@]+$`                            |

## Flask Integration

```python
import envguard
from flask import Flask

@envguard.guard()
def create_app():
    app = Flask(__name__)
    # ... configure app ...
    return app

app = create_app()
```

## FastAPI Integration

```python
import envguard
from fastapi import FastAPI

envguard.check()  # Validate before creating the app

app = FastAPI()
```

## Error Handling Modes

```python
# Raise an exception (default)
envguard.check(on_error="raise")

# Print warnings to stderr and continue
envguard.check(on_error="warn")

# Exit the process with code 1
envguard.check(on_error="exit")

# Strict mode: treat warnings as errors
envguard.check(strict=True)
```

## License

MIT
