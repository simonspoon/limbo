# Contributing to limbo

Thank you for your interest in contributing to limbo!

## Development Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/simonspoon/limbo.git
   cd limbo
   ```

2. Build the project:
   ```bash
   go build -o limbo ./cmd/limbo
   ```

3. Run tests:
   ```bash
   go test ./...
   ```

4. Run tests with coverage:
   ```bash
   go test ./... -coverprofile=coverage.out
   go tool cover -html=coverage.out
   ```

5. Run the linter:
   ```bash
   golangci-lint run
   ```

## Code Style

- Follow the Go style conventions
- Run `golangci-lint run` before submitting changes
- Refer to `.golangci.yml` for linting configuration

## Pull Request Process

1. Fork the repository and create your branch from `main`
2. Add tests for any new functionality
3. Ensure all tests pass (`go test ./...`)
4. Ensure the linter passes (`golangci-lint run`)
5. Update documentation if needed
6. Submit a pull request with a clear description of the changes

### PR Checklist

- [ ] Tests added/updated
- [ ] Linter passes
- [ ] Documentation updated (if applicable)
- [ ] Commit messages are clear and descriptive

## Reporting Issues

When reporting bugs, please include:

- Your Go version (`go version`)
- Your operating system
- Steps to reproduce the issue
- Expected vs actual behavior
- Any relevant error messages or output

## Project Structure

```
limbo/
├── cmd/limbo/          # Entry point
├── internal/
│   ├── commands/       # Cobra command implementations
│   ├── models/         # Task model and constants
│   └── storage/        # JSON file storage
```

## Questions?

Open an issue for any questions about contributing.
