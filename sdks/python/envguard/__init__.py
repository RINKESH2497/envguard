"""
envguard - Runtime environment variable validation for Python.

Validates environment variables against an ``envguard.yaml`` schema at
application startup, catching misconfigurations before they cause runtime
errors in production.

Usage::

    import envguard
    envguard.check()  # Validates env vars against envguard.yaml, raises on error

    # Or with options:
    envguard.check(
        schema_path="envguard.yaml",
        env_source="auto",    # "auto" (file+os), "file" (.env only), "os" (os.environ only)
        env_path=".env",
        strict=False,         # Treat warnings as errors
        on_error="raise",     # "raise" (ValidationError), "exit" (sys.exit(1)), "warn" (print warnings)
    )

    # Or use the guard decorator for Flask/FastAPI:
    @envguard.guard()
    def create_app():
        ...
"""

from __future__ import annotations

import os
import re
import sys
import urllib.parse
from dataclasses import dataclass, field
from enum import Enum
from pathlib import Path
from typing import Any, Callable, Dict, List, Optional, Sequence, TypeVar, Union

import yaml

__version__ = "0.1.0"

__all__ = [
    "check",
    "guard",
    "load_schema",
    "parse_dotenv",
    "validate",
    "Schema",
    "VarSpec",
    "ValidationResult",
    "ValidationError",
    "Issue",
    "__version__",
]

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

_BOOLEAN_TRUE = frozenset({"true", "1", "yes"})
_BOOLEAN_FALSE = frozenset({"false", "0", "no"})
_BOOLEAN_VALUES = _BOOLEAN_TRUE | _BOOLEAN_FALSE

_VALID_TYPES = frozenset({"string", "integer", "float", "boolean", "url", "email"})
_VALID_ENV_SOURCES = frozenset({"auto", "file", "os"})
_VALID_ON_ERROR = frozenset({"raise", "exit", "warn"})

_EMAIL_RE = re.compile(r"^[^@]+@[^@]+\.[^@]+$")

F = TypeVar("F", bound=Callable[..., Any])


# ---------------------------------------------------------------------------
# Data classes
# ---------------------------------------------------------------------------


@dataclass
class VarSpec:
    """Specification for a single environment variable declared in the schema."""

    type: str = "string"
    required: bool = False
    default: Optional[str] = None
    description: str = ""
    pattern: Optional[str] = None
    allowed: Optional[List[str]] = None
    secret: bool = False
    example: Optional[str] = None

    def __post_init__(self) -> None:
        if self.type not in _VALID_TYPES:
            raise ValueError(
                f"Invalid type {self.type!r}; must be one of {sorted(_VALID_TYPES)}"
            )


@dataclass
class Issue:
    """A single validation issue (error or warning) for an environment variable."""

    var_name: str
    message: str
    severity: str  # "error" | "warning"

    def __str__(self) -> str:
        tag = "ERROR" if self.severity == "error" else "WARN"
        return f"[{tag}] {self.var_name}: {self.message}"


@dataclass
class ValidationResult:
    """Aggregate result of environment variable validation."""

    errors: List[Issue] = field(default_factory=list)
    warnings: List[Issue] = field(default_factory=list)
    valid: int = 0
    total: int = 0

    # -- convenience helpers -------------------------------------------------

    def has_errors(self) -> bool:
        """Return ``True`` if there are any error-level issues."""
        return len(self.errors) > 0

    def has_warnings(self) -> bool:
        """Return ``True`` if there are any warning-level issues."""
        return len(self.warnings) > 0

    def summary(self) -> str:
        """Return a human-readable summary line."""
        parts: list[str] = []
        parts.append(f"{self.valid}/{self.total} variables valid")
        if self.errors:
            parts.append(f"{len(self.errors)} error(s)")
        if self.warnings:
            parts.append(f"{len(self.warnings)} warning(s)")
        return ", ".join(parts)


class ValidationError(Exception):
    """Raised when environment variable validation fails."""

    def __init__(self, result: ValidationResult) -> None:
        self.result = result
        lines = [result.summary()]
        for issue in result.errors + result.warnings:
            lines.append(f"  {issue}")
        super().__init__("\n".join(lines))


# ---------------------------------------------------------------------------
# Schema loading
# ---------------------------------------------------------------------------


class Schema:
    """Parsed ``envguard.yaml`` schema.

    Attributes:
        version: Schema format version (currently ``"1"``).
        vars: Mapping of variable name to its :class:`VarSpec`.
        raw: The raw parsed YAML dictionary.
    """

    def __init__(
        self,
        data: Optional[Dict[str, Any]] = None,
        *,
        version: str = "1",
        vars: Optional[Dict[str, Any]] = None,
    ) -> None:
        if data is None:
            data = {"version": version, "vars": vars or {}}
        self.raw = data
        self.version = str(data.get("version", version))
        self.vars = {}

        vars_section = data.get("vars") or data.get("variables") or {}
        if not isinstance(vars_section, dict):
            raise ValueError("'vars' section in schema must be a mapping")

        for name, spec_data in vars_section.items():
            if isinstance(spec_data, VarSpec):
                self.vars[name] = spec_data
                continue
            if spec_data is None:
                spec_data = {}
            if not isinstance(spec_data, dict):
                raise ValueError(
                    f"Variable {name!r}: spec must be a mapping or VarSpec, got {type(spec_data).__name__}"
                )
            self.vars[name] = _parse_var_spec(name, spec_data)

    def __repr__(self) -> str:
        return f"Schema(version={self.version!r}, vars={len(self.vars)})"


def _parse_var_spec(name: str, data: Dict[str, Any]) -> VarSpec:
    """Parse a raw YAML mapping into a :class:`VarSpec`."""
    allowed_raw = data.get("allowed")
    if allowed_raw is not None:
        if not isinstance(allowed_raw, list):
            raise ValueError(f"Variable {name!r}: 'allowed' must be a list")
        allowed_raw = [str(v) for v in allowed_raw]

    default_raw = data.get("default")
    if default_raw is not None:
        default_raw = str(default_raw)

    example_raw = data.get("example")
    if example_raw is not None:
        example_raw = str(example_raw)

    return VarSpec(
        type=str(data.get("type", "string")),
        required=bool(data.get("required", False)),
        default=default_raw,
        description=str(data.get("description", "")),
        pattern=data.get("pattern"),
        allowed=allowed_raw,
        secret=bool(data.get("secret", False)),
        example=example_raw,
    )


def load_schema(path: Union[str, Path] = "envguard.yaml") -> Schema:
    """Load and parse an envguard schema from *path*.

    Args:
        path: Filesystem path to the YAML schema file.

    Returns:
        A :class:`Schema` instance.

    Raises:
        FileNotFoundError: If *path* does not exist.
        ValueError: If the YAML content is invalid.
    """
    path = Path(path)
    if not path.is_file():
        raise FileNotFoundError(f"Schema file not found: {path}")

    with open(path, "r", encoding="utf-8") as fh:
        data = yaml.safe_load(fh)

    if not isinstance(data, dict):
        raise ValueError(f"Schema must be a YAML mapping, got {type(data).__name__}")

    return Schema(data)


# ---------------------------------------------------------------------------
# .env parser
# ---------------------------------------------------------------------------


def parse_dotenv(path: Union[str, Path] = ".env") -> Dict[str, str]:
    """Parse a ``.env`` file into a dictionary.

    Handles:
    - Blank lines and ``#`` comments
    - ``export`` prefix (``export KEY=value``)
    - Single-quoted, double-quoted, and unquoted values
    - Inline comments for unquoted values

    Args:
        path: Filesystem path to the ``.env`` file.

    Returns:
        A dictionary mapping variable names to their string values.
    """
    env: Dict[str, str] = {}
    path = Path(path)
    if not path.is_file():
        return env

    with open(path, "r", encoding="utf-8") as fh:
        for raw_line in fh:
            line = raw_line.strip()

            # Skip empty lines and comments.
            if not line or line.startswith("#"):
                continue

            # Strip optional ``export`` prefix.
            if line.startswith("export "):
                line = line[len("export ") :]

            # Locate the first ``=``.
            eq_index = line.find("=")
            if eq_index == -1:
                continue

            key = line[:eq_index].strip()
            value_part = line[eq_index + 1 :].strip()

            if not key:
                continue

            value = _parse_dotenv_value(value_part)
            env[key] = value

    return env


def _unescape_double_quoted(raw: str) -> str:
    result = []
    i = 0
    while i < len(raw):
        if raw[i] == "\\" and i + 1 < len(raw):
            nxt = raw[i + 1]
            if nxt == "n":
                result.append("\n")
            elif nxt == "t":
                result.append("\t")
            elif nxt == '"':
                result.append('"')
            elif nxt == "\\":
                result.append("\\")
            else:
                result.append(nxt)
            i += 2
        else:
            result.append(raw[i])
            i += 1
    return "".join(result)


def _parse_dotenv_value(raw: str) -> str:
    """Extract the value portion of a .env line, respecting quotes."""
    if not raw:
        return ""

    # Double-quoted value — consume until matching unescaped quote.
    if raw.startswith('"'):
        end = _find_closing_quote(raw, '"')
        if end != -1:
            inner = raw[1:end]
            return _unescape_double_quoted(inner)
        # No closing quote — return everything after the opening quote.
        return raw[1:]

    # Single-quoted value — literal content, no escape processing.
    if raw.startswith("'"):
        end = _find_closing_quote(raw, "'")
        if end != -1:
            return raw[1:end]
        return raw[1:]

    # Unquoted — strip inline comments.
    comment_index = raw.find(" #")
    if comment_index != -1:
        raw = raw[:comment_index]
    return raw.rstrip()


def _find_closing_quote(s: str, quote: str) -> int:
    """Find the index of the closing *quote* in *s*, skipping escaped quotes."""
    i = 1
    while i < len(s):
        if s[i] == "\\" and quote == '"':
            i += 2  # skip escaped character
            continue
        if s[i] == quote:
            return i
        i += 1
    return -1


# ---------------------------------------------------------------------------
# Type & value validation helpers
# ---------------------------------------------------------------------------


def _validate_type(value: str, expected_type: str) -> Optional[str]:
    """Return an error message if *value* does not conform to *expected_type*."""
    if expected_type == "string":
        return None

    if expected_type == "integer":
        try:
            int(value)
            return None
        except ValueError:
            return f"expected integer, got {value!r}"

    if expected_type == "float":
        try:
            float(value)
            return None
        except ValueError:
            return f"expected float, got {value!r}"

    if expected_type == "boolean":
        if value.lower() not in _BOOLEAN_VALUES:
            return (
                f"expected boolean (true/false/1/0/yes/no), got {value!r}"
            )
        return None

    if expected_type == "url":
        parsed = urllib.parse.urlparse(value)
        if not parsed.scheme or not parsed.netloc:
            return f"expected URL with scheme and host, got {value!r}"
        return None

    if expected_type == "email":
        if not _EMAIL_RE.match(value):
            return f"expected email address, got {value!r}"
        return None

    return f"unknown type {expected_type!r}"


def _validate_pattern(value: str, pattern: str) -> Optional[str]:
    """Return an error message if *value* does not match *pattern*."""
    try:
        if not re.fullmatch(pattern, value):
            return f"does not match pattern {pattern!r}"
    except re.error as exc:
        return f"invalid regex pattern {pattern!r}: {exc}"
    return None


def _validate_allowed(value: str, allowed: Sequence[str]) -> Optional[str]:
    """Return an error message if *value* is not in the *allowed* list."""
    if value not in allowed:
        return f"value {value!r} not in allowed list {list(allowed)}"
    return None


# ---------------------------------------------------------------------------
# Core validation
# ---------------------------------------------------------------------------


def validate(schema: Schema, env_vars: Dict[str, str]) -> ValidationResult:
    """Validate *env_vars* against the provided *schema*.

    Args:
        schema: A loaded :class:`Schema`.
        env_vars: A mapping of environment variable names to their string
            values (as they appear in the runtime environment or ``.env``
            file).

    Returns:
        A :class:`ValidationResult` summarising errors and warnings.
    """
    result = ValidationResult(total=len(schema.vars))

    for var_name, spec in schema.vars.items():
        value = env_vars.get(var_name)

        # --- missing variable -------------------------------------------
        if value is None or value == "":
            if spec.default is not None:
                # Default is available — count as valid but inform the user.
                result.warnings.append(
                    Issue(
                        var_name=var_name,
                        message=f"not set; using default {spec.default!r}",
                        severity="warning",
                    )
                )
                result.valid += 1
                continue

            if spec.required:
                result.errors.append(
                    Issue(
                        var_name=var_name,
                        message="required but not set",
                        severity="error",
                    )
                )
            else:
                # Optional and missing — that's fine.
                result.valid += 1
            continue

        # --- type check --------------------------------------------------
        type_err = _validate_type(value, spec.type)
        if type_err:
            result.errors.append(
                Issue(var_name=var_name, message=type_err, severity="error")
            )
            continue

        # --- pattern check -----------------------------------------------
        if spec.pattern is not None:
            pat_err = _validate_pattern(value, spec.pattern)
            if pat_err:
                result.errors.append(
                    Issue(var_name=var_name, message=pat_err, severity="error")
                )
                continue

        # --- allowed list ------------------------------------------------
        if spec.allowed is not None:
            allowed_err = _validate_allowed(value, spec.allowed)
            if allowed_err:
                result.errors.append(
                    Issue(var_name=var_name, message=allowed_err, severity="error")
                )
                continue

        result.valid += 1

    return result


# ---------------------------------------------------------------------------
# Public API — check() and guard()
# ---------------------------------------------------------------------------


def _resolve_env_vars(
    env_source: str,
    env_path: Union[str, Path],
) -> Dict[str, str]:
    """Build the effective env-var dictionary according to *env_source*.

    * ``"auto"`` — merge ``.env`` file values *under* ``os.environ`` (OS wins).
    * ``"file"`` — use only the ``.env`` file.
    * ``"os"``   — use only ``os.environ``.
    """
    if env_source == "file":
        return parse_dotenv(env_path)

    if env_source == "os":
        return dict(os.environ)

    # "auto": .env is the base, os.environ overrides.
    merged = parse_dotenv(env_path)
    merged.update(os.environ)
    return merged


def check(
    *,
    schema_path: Union[str, Path] = "envguard.yaml",
    env_source: str = "auto",
    env_path: Union[str, Path] = ".env",
    strict: bool = False,
    on_error: str = "raise",
) -> ValidationResult:
    """Validate environment variables against an ``envguard.yaml`` schema.

    This is the primary entry-point for the library.  Call it at application
    startup (e.g. in your ``create_app`` factory, your ``manage.py``, or your
    ``__main__.py``) to surface misconfiguration *before* any request is
    served.

    Args:
        schema_path: Path to the ``envguard.yaml`` schema file.
        env_source: Where to read env vars from — ``"auto"`` (merge .env and
            ``os.environ``), ``"file"`` (``.env`` only), or ``"os"``
            (``os.environ`` only).
        env_path: Path to the ``.env`` file (used when *env_source* is
            ``"auto"`` or ``"file"``).
        strict: When ``True``, warnings are promoted to errors.
        on_error: Action to take when errors are present — ``"raise"``
            (raise :class:`ValidationError`), ``"exit"`` (call
            ``sys.exit(1)``), or ``"warn"`` (print to stderr).

    Returns:
        A :class:`ValidationResult`.

    Raises:
        ValidationError: If validation fails and *on_error* is ``"raise"``.
        ValueError: If *env_source* or *on_error* has an invalid value.
    """
    if env_source not in _VALID_ENV_SOURCES:
        raise ValueError(
            f"env_source must be one of {sorted(_VALID_ENV_SOURCES)}, "
            f"got {env_source!r}"
        )
    if on_error not in _VALID_ON_ERROR:
        raise ValueError(
            f"on_error must be one of {sorted(_VALID_ON_ERROR)}, got {on_error!r}"
        )

    schema = load_schema(schema_path)
    env_vars = _resolve_env_vars(env_source, env_path)
    result = validate(schema, env_vars)

    # Strict mode: promote warnings to errors.
    if strict and result.warnings:
        result.errors.extend(result.warnings)
        result.warnings = []

    # Handle errors according to on_error strategy.
    if result.has_errors():
        if on_error == "raise":
            raise ValidationError(result)
        elif on_error == "exit":
            print(f"envguard: validation failed\n{result.summary()}", file=sys.stderr)
            for issue in result.errors + result.warnings:
                print(f"  {issue}", file=sys.stderr)
            sys.exit(1)
        else:  # "warn"
            print(f"envguard: {result.summary()}", file=sys.stderr)
            for issue in result.errors + result.warnings:
                print(f"  {issue}", file=sys.stderr)

    return result


def guard(
    **kwargs: Any,
) -> Callable[[F], F]:
    """Decorator that validates env vars before the decorated function runs.

    Intended for use with application factory functions::

        @envguard.guard()
        def create_app():
            app = Flask(__name__)
            return app

        @envguard.guard(strict=True)
        def create_app():
            ...

    All keyword arguments are forwarded to :func:`check`.
    """

    def decorator(func: F) -> F:
        import functools

        @functools.wraps(func)
        def wrapper(*args: Any, **kw: Any) -> Any:
            check(**kwargs)
            return func(*args, **kw)

        return wrapper  # type: ignore[return-value]

    return decorator
