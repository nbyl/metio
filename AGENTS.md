# Agent Instructions for Metio

## Build/Test Commands
- **Development server**: `air` (auto-reloads on file changes)
- **Build single binary**: `make <binary_name>` (e.g., `make controller`)
- **Build all binaries**: `make` or `make all` (builds all cmd/*/main.go files)
- **Clean build artifacts**: `make clean`
- **Generate templates/CSS**: `go generate ./...`
- **Docker build Controller**: `make build-controller-image`
- **Docker build machine-agent**: `make build-machine-agent-image`
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

## Development Workflow & Infrastructure

- **Issues and Ticket system:** Issues and features are stored in Linear, to retrieve them use the linear tool.
- **Branch Naming:** Use the generated branch name from linear.
- **Testing:** Before comitting, deploy the full system at least once using `make deploy-full`. Check the controller website and ssh into the machine using `gcloud compute ssh --zone "europe-west3-a" $SERVERNAME --project "minecraftbyl"`.
- **Commit Messages:** Follow the Conventional Commits specification. (e.g., \`feat: add user profile page\`)
- **Pull Requests:** Once the ticket is ready, push the code to the gitlab repository and create a merge request for it. When done, set the linear issue to "in review".
