.PHONY: all build clean build-images build-machine-agent-image build-controller-image deploy help

USERNAME := $(shell whoami)

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

# Run all tests and create coverage report
test:
	@mkdir -p build
	go test ./... -coverprofile=build/coverage.out -covermode=atomic
	go tool cover -html=build/coverage.out -o build/coverage.html
	@echo "Coverage report generated at build/coverage.html"

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

# Build machine-agent Docker image and save tag to file
build-machine-agent-image:
	@mkdir -p build
	@echo "Building Docker image for machine-agent..."
	@MACHINE_AGENT_BUILD_ID=$$(gcloud builds submit . --config cmd/machine-agent/cloudbuild.yaml --format="value(id)" --region europe-west3 --substitutions=COMMIT_SHA="$(USERNAME)-local" --polling-interval=3) ;\
	echo "Build ID: $${MACHINE_AGENT_BUILD_ID}" ;\
	MACHINE_AGENT_IMAGE_TAG=$$(gcloud builds describe $${MACHINE_AGENT_BUILD_ID} --format="value(images[0])" --region europe-west3) ;\
	echo "Built image: $${MACHINE_AGENT_IMAGE_TAG}" ;\
	echo "$${MACHINE_AGENT_IMAGE_TAG}" > build/machine-agent-image.txt ;\
	echo "Machine agent image tag saved to build/machine-agent-image.txt"

# Build controller Docker image and save tag to file
build-controller-image:
	@mkdir -p build
	@echo "Building Docker image for controller..."
	@CONTROLLER_BUILD_ID=$$(gcloud builds submit . --config cmd/controller/cloudbuild.yaml --format="value(id)" --region europe-west3 --substitutions=COMMIT_SHA="$(USERNAME)-local" --polling-interval=3) ;\
	echo "Build ID: $${CONTROLLER_BUILD_ID}" ;\
	CONTROLLER_IMAGE_TAG=$$(gcloud builds describe $${CONTROLLER_BUILD_ID} --format="value(images[0])" --region europe-west3) ;\
	echo "Built image: $${CONTROLLER_IMAGE_TAG}" ;\
	echo "$${CONTROLLER_IMAGE_TAG}" > build/controller-image.txt ;\
	echo "Controller image tag saved to build/controller-image.txt"

# Build both Docker images
build-images: build-machine-agent-image build-controller-image
	@echo "All Docker images built successfully"

# Deploy infrastructure: apply OpenTofu with pre-built Docker images
deploy: build-images
	@set -e ;\
	if [ ! -f "build/machine-agent-image.txt" ] || [ ! -f "build/controller-image.txt" ]; then \
		echo "Error: Image tag files not found. Run 'make build-images' first." ;\
		exit 1 ;\
	fi ;\
	MACHINE_AGENT_IMAGE_TAG=$$(cat build/machine-agent-image.txt) ;\
	CONTROLLER_IMAGE_TAG=$$(cat build/controller-image.txt) ;\
	echo "Deploying infrastructure with OpenTofu..." ;\
	echo "Machine agent image: $${MACHINE_AGENT_IMAGE_TAG}" ;\
	echo "Controller image: $${CONTROLLER_IMAGE_TAG}" ;\
	tofu -chdir=cloud apply -var="machine_agent_image=$${MACHINE_AGENT_IMAGE_TAG}" -var="controller_image=$${CONTROLLER_IMAGE_TAG}" -auto-approve

# Show available targets
help:
	@echo "Available targets:"
	@echo "  all                     - Build all binaries (default)"
	@echo "  build                   - Build all binaries"
	@echo "  <binary>                - Build specific binary (e.g., make controller)"
	@echo "  build-machine-agent-image - Build machine-agent Docker image"
	@echo "  build-controller-image   - Build controller Docker image"
	@echo "  build-images            - Build both Docker images"
	@echo "  deploy                  - Build images and deploy with OpenTofu"
	@echo "  clean                   - Remove build artifacts"
	@echo "  help                    - Show this help"