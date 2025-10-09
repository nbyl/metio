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

# Show available targets
help:
	@echo "Available targets:"
	@echo "  all        - Build all binaries (default)"
	@echo "  build      - Build all binaries"
	@echo "  <binary>   - Build specific binary (e.g., make controller)"
	@echo "  clean      - Remove build artifacts"
	@echo "  help       - Show this help"