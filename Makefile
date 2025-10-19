.PHONY: all build clean help

# Default target builds all binaries
all: build

# Build all binaries from cmd/*/main.go
build:
	@mkdir -p build
	@for dir in cmd/*/; do \
		if [ -f "$$dir/main.go" ]; then \
			binary=$$(basename "$$dir"); \
			echo "Building $$binary..."; \
			go build -o "build/$$binary" "./$$dir"; \
			echo "Built build/$$binary"; \
		fi; \
	done
	@echo "All binaries built successfully"

# Build specific binary (usage: make controller)
%:
	@mkdir -p build
	@if [ -f "cmd/$@/main.go" ]; then \
		echo "Building $@..."; \
		go build -o "build/$@" "./cmd/$@"; \
		echo "Built build/$@"; \
	else \
		echo "Error: cmd/$@/main.go not found"; \
		exit 1; \
	fi

# Clean build artifacts
clean:
	rm -rf build/

# Deploy Minecraft server: build Docker image and apply OpenTofu
deploy-minecraft: machine-agent
	@echo "Building Docker image for machine-agent..."
	gcloud builds submit . --config cmd/machine-agent/cloudbuild.yaml
	@echo "Deploying infrastructure with OpenTofu..."
	tofu -chdir=cloud apply || terraform -chdir=cloud apply

# Show available targets
help:
	@echo "Available targets:"
	@echo "  all             - Build all binaries (default)"
	@echo "  build           - Build all binaries"
	@echo "  <binary>        - Build specific binary (e.g., make controller)"
	@echo "  deploy-minecraft - Build Docker image and deploy with OpenTofu"
	@echo "  clean           - Remove build artifacts"
	@echo "  help            - Show this help"