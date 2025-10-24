# Agent Instructions for Metio

## Build/Test Commands
- **Development server**: `air` (auto-reloads on file changes)
- **Build single binary**: `make <binary_name>` (e.g., `make controller`)
- **Build all binaries**: `make` or `make all` (builds all cmd/*/main.go files)
- **Clean build artifacts**: `make clean`
- **Generate templates/CSS**: `go generate ./...`
- **Docker build**: `docker build -t <image> .` (builds controller binary)
- **No test framework configured** - add Go tests as needed

## Code Style Guidelines

### Go Code
- **Formatting**: Use standard `gofmt` (no custom formatter)
- **Imports**: Group by standard library, third-party, local (blank line between groups)
- **Naming**: camelCase for functions/variables, PascalCase for exported types
- **Error handling**: Log errors with `log.Print(err)`, return HTTP 500 for server errors
- **Configuration**: Use `viper` with environment variables (PORT, GCP_PROJECT, etc.)
- **Routing**: Use `gorilla/mux` with method-specific handlers
- **Templates**: Use `templ` for HTML generation, run `go generate` after changes

### Frontend
- **CSS**: Use Tailwind CSS, generate with `npx tailwindcss` via `go generate`
- **Static files**: Serve from `./static/` directory

### Infrastructure
- **Terraform**: Format with `terraform fmt` (enforced by pre-commit)
- **Pre-commit hooks**: Run `pre-commit install` to enable formatting/linting

### General
- **No linting configured** - consider adding `golangci-lint`
- **No Cursor/Copilot rules** - follow Go community standards