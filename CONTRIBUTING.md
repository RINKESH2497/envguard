# Contributing to envguard

Thank you for your interest in contributing to envguard! This guide will help you get started.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Reporting Bugs](#reporting-bugs)
- [Requesting Features](#requesting-features)
- [Development Setup](#development-setup)
- [Code Style](#code-style)
- [Pull Request Process](#pull-request-process)

---

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](https://www.contributor-covenant.org/version/2/1/code_of_conduct/). By participating, you are expected to uphold this code. Please report unacceptable behavior to the maintainers.

## Reporting Bugs

Found a bug? Please [open an issue](https://github.com/RINKESH2497/envguard/issues/new?template=bug_report.yml) with:

1. **A clear, descriptive title.**
2. **Steps to reproduce** — include your `envguard.yaml`, the command you ran, and any relevant `.env` contents (redact secrets!).
3. **Expected behavior** — what you expected to happen.
4. **Actual behavior** — what actually happened, including full error output.
5. **Environment** — OS, Go version (`go version`), envguard version (`envguard --version`).

## Requesting Features

Have an idea? [Open a feature request](https://github.com/RINKESH2497/envguard/issues/new?template=feature_request.yml) with:

1. **Problem statement** — what pain point does this address?
2. **Proposed solution** — how would it work from a user's perspective?
3. **Alternatives considered** — any other approaches you thought of.

We'll discuss the proposal in the issue before implementation begins.

## Development Setup

### Prerequisites

- **Go 1.21+** — [Install Go](https://go.dev/dl/)
- **Git** — [Install Git](https://git-scm.com/)
- **golangci-lint** (optional, for linting) — [Install](https://golangci-lint.run/welcome/install/)

### Getting Started

1. **Fork and clone the repository:**

   ```bash
   git clone https://github.com/<your-username>/envguard.git
   cd envguard
   ```

2. **Build the project:**

   ```bash
   make build
   # or directly:
   go build -o bin/envguard ./cmd/envguard
   ```

3. **Run the tests:**

   ```bash
   make test
   # or directly:
   go test ./...
   ```

4. **Run the binary:**

   ```bash
   ./bin/envguard --help
   ```

### Project Structure

```
envguard/
├── cmd/envguard/       # CLI entrypoint
│   └── main.go
├── internal/           # Private application code
│   ├── config/         # Schema parsing and loading
│   ├── validator/      # Validation engine
│   ├── scanner/        # Secret scanning
│   ├── syncer/         # .env.example sync logic
│   └── onboard/        # Interactive onboarding
├── pkg/                # Public library code
├── action/             # GitHub Action definition
├── examples/           # Example schemas
├── .github/workflows/  # CI/CD pipelines
├── Makefile
├── envguard.yaml       # Schema for envguard itself (dogfooding)
└── README.md
```

## Code Style

- **Format all code** with `gofmt`:

  ```bash
  make fmt
  # or: gofmt -w .
  ```

- **Lint** with `golangci-lint`:

  ```bash
  make lint
  # or: golangci-lint run
  ```

- Follow the [Effective Go](https://go.dev/doc/effective_go) guidelines.
- Write clear, descriptive commit messages following [Conventional Commits](https://www.conventionalcommits.org/):
  - `feat: add JSON output format for check command`
  - `fix: handle missing newline at end of .env file`
  - `docs: update schema reference with min_length field`
  - `test: add integration tests for sync command`

## Pull Request Process

1. **Create a feature branch** from `main`:

   ```bash
   git checkout -b feat/my-feature
   ```

2. **Make your changes.** Write tests for new functionality.

3. **Ensure everything passes:**

   ```bash
   make all   # runs: fmt, lint, test, build
   ```

4. **Push and open a Pull Request** against `main`.

5. **In your PR description:**
   - Reference any related issues (`Fixes #123`).
   - Describe what changed and why.
   - Include before/after output if applicable.

6. **Address review feedback.** We aim to review PRs within a few days.

### What We Look For

- ✅ Tests pass and new code has test coverage
- ✅ Code is formatted (`gofmt`) and lint-free
- ✅ Public APIs have doc comments
- ✅ Commit messages are clear and descriptive
- ✅ No unrelated changes mixed in

---

Thank you for helping make envguard better! 🎉
