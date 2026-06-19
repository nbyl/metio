.PHONY: all build clean build-images build-machine-agent-image build-controller-image deploy deploy-full deploy-infrastructure deploy-machine-agent deploy-controller check-images use-default-images cleanup-old-images install-web build-web test test-backend test-web develop lint-web verify-backend controller-image machine-agent-image push-images promote promote-distribution help

USERNAME := $(shell whoami)

# Default target builds all binaries
all: build

# Build all binaries from cmd/*/main.go
build: build-web
	@mkdir -p build
	@for dir in cmd/*/; do \
		if [ -f "$$dir/main.go" ]; then \
			binary=$$(basename "$$dir"); \
			version=$$(git rev-parse --short HEAD 2>/dev/null || echo "local"); \
			echo "Building $$binary with version $$version..."; \
			go build -ldflags "-X main.Version=$$version" -o "build/$$binary" "./$$dir"; \
			echo "Built build/$$binary"; \
		fi; \
	done
	@echo "All binaries built successfully"

# Run Go tests with coverage
test-backend: build-web
	@mkdir -p build
	go test ./... -coverprofile=build/coverage.out -covermode=atomic
	go tool cover -html=build/coverage.out -o build/coverage.html
	@echo "Coverage report generated at build/coverage.html"

# Run Go verification (tidy, verify, fmt, vet)
verify-backend: build-web
	go mod tidy
	go mod verify
	go fmt ./...
	go vet ./...

# Install frontend dependencies
install-web:
	cd web && npm ci

# Build frontend assets
build-web: install-web
	cd web && npm run build

# Run frontend vitest suite
test-web: install-web
	cd web && npm run test:run

# Run frontend linter
lint-web: install-web
	cd web && npm run lint

# Run all tests (backend + frontend)
test: test-backend test-web

# Generate combined local env file from devcontainer env and machine-agent image tag
generate-env:
	@mkdir -p build
	@cp .devcontainer/devcontainer.env build/local.env
	@if [ -f build/machine-agent-image.txt ]; then \
		echo "" >> build/local.env; \
		echo "MACHINE_AGENT_IMAGE=$$(cat build/machine-agent-image.txt)" >> build/local.env; \
	fi
	@echo "Generated build/local.env"

# Start backend (air) and frontend (Vite) with hot reload
develop: generate-env
	@echo "Starting development servers..."
	@trap 'kill 0' EXIT; \
	set -a; . build/local.env; set +a; \
	cd web && npm run dev & \
	air & \
	wait

# Build specific binary (usage: make controller)
%:
	@mkdir -p build
	@if [ -f "cmd/$@/main.go" ]; then \
		version=$$(git rev-parse --short HEAD 2>/dev/null || echo "local"); \
		echo "Building $@ with version $$version..."; \
		go build -ldflags "-X main.Version=$$version" -o "build/$@" "./cmd/$@"; \
		echo "Built build/$@"; \
	else \
		echo "Error: cmd/$@/main.go not found"; \
		exit 1; \
	fi

# Clean build artifacts
clean:
	rm -rf build/
	rm -rf static/dist/


# Build machine-agent Docker image and save tag to file
build-machine-agent-image:
	@mkdir -p build
	@TIMESTAMP=$$(date +%Y%m%d-%H%M%S) ;\
	echo "Building Docker image for machine-agent with timestamp: $${TIMESTAMP}..." ;\
	MACHINE_AGENT_BUILD_ID=$$(gcloud builds submit . --config cmd/machine-agent/cloudbuild.yaml --format="value(id)" --region europe-west3 --substitutions=COMMIT_SHA="$(USERNAME)-local-$${TIMESTAMP}" --polling-interval=3) ;\
	echo "Build ID: $${MACHINE_AGENT_BUILD_ID}" ;\
	MACHINE_AGENT_IMAGE_TAG=$$(gcloud builds describe $${MACHINE_AGENT_BUILD_ID} --format="value(images[0])" --region europe-west3) ;\
	echo "Built image: $${MACHINE_AGENT_IMAGE_TAG}" ;\
	echo "$${MACHINE_AGENT_IMAGE_TAG}" > build/machine-agent-image.txt ;\
	echo "Machine agent image tag saved to build/machine-agent-image.txt"

# Build controller Docker image and save tag to file
build-controller-image:
	@mkdir -p build
	@TIMESTAMP=$$(date +%Y%m%d-%H%M%S) ;\
	echo "Building Docker image for controller with timestamp: $${TIMESTAMP}..." ;\
	CONTROLLER_BUILD_ID=$$(gcloud builds submit . --config cmd/controller/cloudbuild.yaml --format="value(id)" --region europe-west3 --substitutions=COMMIT_SHA="$(USERNAME)-local-$${TIMESTAMP}" --polling-interval=3) ;\
	echo "Build ID: $${CONTROLLER_BUILD_ID}" ;\
	CONTROLLER_IMAGE_TAG=$$(gcloud builds describe $${CONTROLLER_BUILD_ID} --format="value(images[0])" --region europe-west3) ;\
	echo "Built image: $${CONTROLLER_IMAGE_TAG}" ;\
	echo "$${CONTROLLER_IMAGE_TAG}" > build/controller-image.txt ;\
	echo "Controller image tag saved to build/controller-image.txt"

# Local controller image build (gcloud-free)
controller-image:
	docker buildx build --platform linux/amd64 -f cmd/controller/Dockerfile -t ghcr.io/nbyl/metio/controller:$(shell git rev-parse --short HEAD) .

# Local machine-agent image build (gcloud-free)
machine-agent-image:
	docker buildx build --platform linux/amd64 -f cmd/machine-agent/Dockerfile -t ghcr.io/nbyl/metio/machine-agent:$(shell git rev-parse --short HEAD) .

# Push images to ghcr.io
push-images:
	docker push ghcr.io/nbyl/metio/controller:$(shell git rev-parse --short HEAD)
	docker push ghcr.io/nbyl/metio/machine-agent:$(shell git rev-parse --short HEAD)

# Promote image tags (usage: make promote FROM=a1b2c3d4 TO=main)
promote:
	@if [ -z "$(FROM)" ] || [ -z "$(TO)" ]; then \
		echo "Usage: make promote FROM=<sha> TO=<tag>"; \
		exit 1; \
	fi
	docker buildx imagetools create -t ghcr.io/nbyl/metio/controller:$(TO) ghcr.io/nbyl/metio/controller:$(FROM)
	docker buildx imagetools create -t ghcr.io/nbyl/metio/machine-agent:$(TO) ghcr.io/nbyl/metio/machine-agent:$(FROM)

# Promote images from ghcr.io to GCP Artifact Registry (distribution repo)
DISTRO_REGISTRY ?= europe-docker.pkg.dev/metio/metio
promote-distribution:
	docker tag ghcr.io/nbyl/metio/controller:$(SHA) $(DISTRO_REGISTRY)/controller:$(SHA)
	docker tag ghcr.io/nbyl/metio/controller:$(SHA) $(DISTRO_REGISTRY)/controller:latest
	docker tag ghcr.io/nbyl/metio/machine-agent:$(SHA) $(DISTRO_REGISTRY)/machine-agent:$(SHA)
	docker tag ghcr.io/nbyl/metio/machine-agent:$(SHA) $(DISTRO_REGISTRY)/machine-agent:latest
	docker push $(DISTRO_REGISTRY)/controller:$(SHA)
	docker push $(DISTRO_REGISTRY)/controller:latest
	docker push $(DISTRO_REGISTRY)/machine-agent:$(SHA)
	docker push $(DISTRO_REGISTRY)/machine-agent:latest

# Build both Docker images (local, without gcloud)
build-images: controller-image machine-agent-image
	@echo "All Docker images built successfully"

# Deploy infrastructure: apply OpenTofu with pre-built Docker images
deploy: deploy-full

# Deploy full system: build both images and deploy all infrastructure
deploy-full: build-images
	@set -e ;\
	if [ ! -f "build/machine-agent-image.txt" ] || [ ! -f "build/controller-image.txt" ]; then \
		echo "Error: Image tag files not found. Run 'make build-images' first." ;\
		exit 1 ;\
	fi ;\
	MACHINE_AGENT_IMAGE_TAG=$$(cat build/machine-agent-image.txt) ;\
	CONTROLLER_IMAGE_TAG=$$(cat build/controller-image.txt) ;\
	echo "Deploying full system with OpenTofu..." ;\
	echo "Machine agent image: $${MACHINE_AGENT_IMAGE_TAG}" ;\
	echo "Controller image: $${CONTROLLER_IMAGE_TAG}" ;\
	tofu -chdir=deploy apply -var="machine_agent_image=$${MACHINE_AGENT_IMAGE_TAG}" -var="controller_image=$${CONTROLLER_IMAGE_TAG}" -auto-approve

# Deploy infrastructure only: use existing images or defaults
deploy-infrastructure:
	@set -e ;\
	if [ -f "build/machine-agent-image.txt" ]; then \
		MACHINE_AGENT_IMAGE_TAG=$$(cat build/machine-agent-image.txt) ;\
	else \
		echo "Machine agent image tag not found, using default" ;\
		MACHINE_AGENT_IMAGE_TAG="ghcr.io/nbyl/metio/machine-agent:latest" ;\
	fi ;\
	if [ -f "build/controller-image.txt" ]; then \
		CONTROLLER_IMAGE_TAG=$$(cat build/controller-image.txt) ;\
	else \
		echo "Controller image tag not found, using default" ;\
		CONTROLLER_IMAGE_TAG="ghcr.io/nbyl/metio/controller:latest" ;\
	fi ;\
	echo "Deploying infrastructure only with OpenTofu..." ;\
	echo "Machine agent image: $${MACHINE_AGENT_IMAGE_TAG}" ;\
	echo "Controller image: $${CONTROLLER_IMAGE_TAG}" ;\
	tofu -chdir=deploy apply -var="machine_agent_image=$${MACHINE_AGENT_IMAGE_TAG}" -var="controller_image=$${CONTROLLER_IMAGE_TAG}" -auto-approve

# Deploy machine-agent only: build machine-agent image and deploy infrastructure
deploy-machine-agent: build-machine-agent-image
	@set -e ;\
	if [ ! -f "build/machine-agent-image.txt" ]; then \
		echo "Error: Machine agent image tag file not found. Run 'make build-machine-agent-image' first." ;\
		exit 1 ;\
	fi ;\
	MACHINE_AGENT_IMAGE_TAG=$$(cat build/machine-agent-image.txt) ;\
	if [ -f "build/controller-image.txt" ]; then \
		CONTROLLER_IMAGE_TAG=$$(cat build/controller-image.txt) ;\
	else \
		echo "Controller image tag not found, using default" ;\
		CONTROLLER_IMAGE_TAG="ghcr.io/nbyl/metio/controller:latest" ;\
	fi ;\
	echo "Deploying machine-agent and infrastructure with OpenTofu..." ;\
	echo "Machine agent image: $${MACHINE_AGENT_IMAGE_TAG}" ;\
	echo "Controller image: $${CONTROLLER_IMAGE_TAG}" ;\
	tofu -chdir=deploy apply -var="machine_agent_image=$${MACHINE_AGENT_IMAGE_TAG}" -var="controller_image=$${CONTROLLER_IMAGE_TAG}" -auto-approve

# Deploy controller only: build controller image and update Cloud Run service
deploy-controller: controller-image
	@set -e ;\
	if [ ! -f "build/controller-image.txt" ]; then \
		echo "Error: Controller image tag file not found. Run 'make build-controller-image' first." ;\
		exit 1 ;\
	fi ;\
	CONTROLLER_IMAGE_TAG=$$(cat build/controller-image.txt) ;\
	echo "Deploying controller only..." ;\
	echo "Controller image: $${CONTROLLER_IMAGE_TAG}" ;\
	tofu -chdir=deploy apply -target=google_cloud_run_v2_service.controller -var="controller_image=$${CONTROLLER_IMAGE_TAG}" -auto-approve

# Clean up old local images to prevent registry bloat
cleanup-old-images:
	@echo "Cleaning old local machine-agent images..."
	@PROJECT_ID=$$(gcloud config get-value project) ;\
	LOCATION="europe-west3" ;\
	gcloud artifacts docker images list "$$LOCATION-docker.pkg.dev/$$PROJECT_ID/metio" --filter="tags~$(USERNAME)-local" --format="value(version)" --limit=10 | tail -n +6 | while read -r digest; do \
		if [ -n "$$digest" ]; then \
			echo "Deleting old image: $$LOCATION-docker.pkg.dev/$$PROJECT_ID/metio/machine-agent@$$digest" ;\
			gcloud artifacts docker images delete "$$LOCATION-docker.pkg.dev/$$PROJECT_ID/metio/machine-agent@$$digest" --quiet || true ;\
		fi ;\
	done
	@echo "Cleaning old local controller images..."
	@gcloud artifacts docker images list "$$LOCATION-docker.pkg.dev/$$PROJECT_ID/metio" --filter="tags~$(USERNAME)-local" --format="value(version)" --limit=10 | tail -n +6 | while read -r digest; do \
		if [ -n "$$digest" ]; then \
			echo "Deleting old image: $$LOCATION-docker.pkg.dev/$$PROJECT_ID/metio/controller@$$digest" ;\
			gcloud artifacts docker images delete "$$LOCATION-docker.pkg.dev/$$PROJECT_ID/metio/controller@$$digest" --quiet || true ;\
		fi ;\
	done
	@echo "Cleanup completed"

# Show available targets
help:
	@echo "Available targets:"
	@echo "  build                   - Build all binaries"
	@echo "  <binary>                - Build specific binary (e.g., make controller)"
	@echo "  controller-image        - Build controller Docker image (local, no gcloud)"
	@echo "  machine-agent-image     - Build machine-agent Docker image (local, no gcloud)"
	@echo "  push-images             - Push images to ghcr.io"
	@echo "  promote                 - Retag image (make promote FROM=<sha> TO=<tag>)"
	@echo "  build-machine-agent-image - Build machine-agent via gcloud builds (legacy)"
	@echo "  build-controller-image  - Build controller via gcloud builds (legacy)"
	@echo "  build-images            - Build both Docker images"
	@echo ""
	@echo "Test targets:"
	@echo "  test                    - Run all tests (backend + frontend)"
	@echo "  test-backend            - Run Go tests with coverage report"
	@echo "  test-web                - Run frontend Vitest suite"
	@echo ""
	@echo "Development:"
	@echo "  develop                 - Start backend (air) and frontend (Vite) with hot reload"
	@echo ""
	@echo "Deployment targets:"
	@echo "  deploy                  - Deploy full system (alias for deploy-full)"
	@echo "  deploy-full             - Build both images and deploy all infrastructure"
	@echo "  deploy-infrastructure   - Deploy infrastructure only (use existing/default images)"
	@echo "  deploy-machine-agent    - Build machine-agent image and deploy infrastructure"
	@echo "  deploy-controller       - Build controller image and update Cloud Run service only"
	@echo ""
	@echo "Other targets:"
	@echo "  clean                   - Remove build artifacts"
	@echo "  cleanup-old-images      - Clean old local images from registry"
	@echo "  help                    - Show this help"
	@echo ""
	@echo "Features:"
	@echo "  - Images are tagged with git SHA for CI builds and promoted on main merge"
	@echo "  - VM recreation is automatically triggered when machine-agent image changes"
	@echo ""
	@echo "Examples:"
	@echo "  make test                     # Run all tests"
	@echo "  make test-backend             # Run Go tests only"
	@echo "  make test-web                 # Run frontend tests only"
	@echo "  make develop                  # Start dev servers with hot reload"
	@echo "  make deploy-full              # Deploy everything with new images"
	@echo "  make deploy-infrastructure    # Update infrastructure without rebuilding"
	@echo "  make deploy-machine-agent     # Update only machine-agent (triggers VM recreation)"
	@echo "  make deploy-controller        # Update only web interface"
	@echo "  make promote FROM=a1b2c3d4 TO=main  # Promote image to main tag"
