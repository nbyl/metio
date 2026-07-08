.PHONY: all build clean build-images build-machine-agent-image build-controller-image deploy deploy-full deploy-infrastructure deploy-machine-agent deploy-controller check-images use-default-images cleanup-old-images install-web build-web test test-backend test-web develop lint-web verify-backend ci-controller-image ci-machine-agent-image controller-image machine-agent-image push-images promote promote-distribution help dev-up dev-down dev-dapr-setup test-dapr-integration

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

# Start backend (air) and frontend (Vite) with hot reload.
# DB_BACKEND (from the env file) controls the datastore:
#   dapr      -> local Dapr sidecar + Datastore emulator (auto-started, torn down on exit)
#   firestore -> Firestore (cloud), no local infra   [default]
develop: generate-env
	@echo "Starting development servers..."
	@set -a; . build/local.env; set +a; \
	if [ "$${DB_BACKEND:-firestore}" = "dapr" ]; then \
		echo "DB_BACKEND=dapr -> starting local Dapr infrastructure..."; \
		$(MAKE) dev-up || exit 1; \
		trap 'make dev-down; kill 0' EXIT; \
	else \
		trap 'kill 0' EXIT; \
	fi; \
	cd web && npm run dev & \
	air & \
	wait

# ──────────────────────────────────────────────
# Dapr local development targets
# ──────────────────────────────────────────────

DAPRD_BIN ?= $(HOME)/.dapr/bin/daprd
DATASTORE_PORT ?= 8081

dev-dapr-setup: ## Initialize Dapr runtime and gcloud components if not already done
	@gcloud components install beta cloud-datastore-emulator --quiet 2>/dev/null; \
	if [ $$? -ne 0 ]; then \
		echo "WARNING: gcloud components install failed; emulator may not work."; \
	fi
	@if [ ! -f "$(DAPRD_BIN)" ]; then \
		echo "Running dapr init --slim to download the daprd binary..."; \
		dapr init --slim; \
	else \
		echo "daprd already installed at $(DAPRD_BIN)"; \
	fi

dev-up: dev-dapr-setup ## Start Dapr infrastructure (datastore emulator + daprd)
	@echo "Starting Datastore emulator..."
	@gcloud beta emulators datastore start --quiet \
		--host-port=localhost:$(DATASTORE_PORT) \
		--project=metio-local \
		--no-store-on-disk \
		&> /tmp/datastore-emulator.log &
	@echo "Waiting for Datastore emulator to be ready..."
	@for i in $$(seq 1 30); do \
		if curl -s http://localhost:$(DATASTORE_PORT) > /dev/null 2>&1; then \
			break; \
		fi; \
		sleep 1; \
	done
	@echo "Datastore emulator ready on localhost:$(DATASTORE_PORT)"
	@echo "Starting daprd..."
	@DATASTORE_EMULATOR_HOST=localhost:$(DATASTORE_PORT) \
	 GOOGLE_CLOUD_PROJECT=metio-local \
	 $(DAPRD_BIN) --app-id controller \
		--resources-path ./dapr/components \
		--dapr-grpc-port 50001 \
		--dapr-http-port 3500 \
		&> /tmp/daprd.log &
	@echo "Waiting for daprd to be ready..."
	@ready=0; \
	for i in $$(seq 1 30); do \
		if curl -s http://localhost:3500/v1.0/healthz/outbound > /dev/null 2>&1; then \
			ready=1; \
			break; \
		fi; \
		sleep 1; \
	done; \
	if [ "$$ready" -eq 0 ]; then \
		echo "ERROR: daprd did not become ready within 30s"; \
		echo "--- /tmp/daprd.log ---"; \
		cat /tmp/daprd.log; \
		echo "--- end ---"; \
		exit 1; \
	fi
	@echo "daprd ready"
	@echo ""
	@echo "Dapr infrastructure is running:"
	@echo "  Datastore emulator: localhost:$(DATASTORE_PORT)"
	@echo "  daprd (gRPC):       localhost:50001"
	@echo ""
	@echo "Run 'make dev-down' to stop."

dev-down: ## Stop Dapr infrastructure
	@echo "Stopping daprd..."
	@pkill daprd 2>/dev/null || true
	@echo "Stopping Datastore emulator..."
	@pkill -f "datastore" 2>/dev/null || true
	@echo "Dapr infrastructure stopped."


test-dapr-integration: dev-up ## Run DaprDB integration tests against the local Datastore emulator
	@echo "Running DaprDB integration tests..."
	@trap 'make dev-down' EXIT; \
	go test -tags=integration -count=1 ./internal/db/ -run TestDaprDB_Integration -v

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

# CI controller image build — tag for ghcr.io, load into local daemon (no push)
ci-controller-image:
	@SHA=$$(git rev-parse --short HEAD); \
	echo "Building controller image for CI: ghcr.io/nbyl/metio/controller:$${SHA}"; \
	docker buildx build --platform linux/amd64 -t ghcr.io/nbyl/metio/controller:$${SHA} -f cmd/controller/Dockerfile --load .

# CI machine-agent image build — tag for ghcr.io, load into local daemon (no push)
ci-machine-agent-image:
	@SHA=$$(git rev-parse --short HEAD); \
	echo "Building machine-agent image for CI: ghcr.io/nbyl/metio/machine-agent:$${SHA}"; \
	docker buildx build --platform linux/amd64 -t ghcr.io/nbyl/metio/machine-agent:$${SHA} -f cmd/machine-agent/Dockerfile --load .

# Local controller image build + push to Artifact Registry
controller-image:
	@mkdir -p build
	@SHA=$$(git rev-parse --short HEAD); \
	IMAGE="europe-west3-docker.pkg.dev/minecraftbyl/metio/controller:$${SHA}"; \
	echo "Building controller image: $${IMAGE}"; \
	docker buildx build --platform linux/amd64 -f cmd/controller/Dockerfile -t $${IMAGE} --push . ; \
	echo "$${IMAGE}" > build/controller-image.txt

# Local machine-agent image build + push to Artifact Registry
machine-agent-image:
	@mkdir -p build
	@SHA=$$(git rev-parse --short HEAD); \
	IMAGE="europe-west3-docker.pkg.dev/minecraftbyl/metio/machine-agent:$${SHA}"; \
	echo "Building machine-agent image: $${IMAGE}"; \
	docker buildx build --platform linux/amd64 -f cmd/machine-agent/Dockerfile -t $${IMAGE} --push . ; \
	echo "$${IMAGE}" > build/machine-agent-image.txt

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
DISTRO_REGISTRY ?= europe-docker.pkg.dev/metio-distribution/metio
promote-distribution:
	docker tag ghcr.io/nbyl/metio/controller:$(SHA) $(DISTRO_REGISTRY)/controller:$(SHA)
	docker tag ghcr.io/nbyl/metio/machine-agent:$(SHA) $(DISTRO_REGISTRY)/machine-agent:$(SHA)
	docker push $(DISTRO_REGISTRY)/controller:$(SHA)
	docker push $(DISTRO_REGISTRY)/machine-agent:$(SHA)
	if [ -n "$(VERSION)" ]; then \
		docker tag ghcr.io/nbyl/metio/controller:$(SHA) $(DISTRO_REGISTRY)/controller:$(VERSION); \
		docker tag ghcr.io/nbyl/metio/machine-agent:$(SHA) $(DISTRO_REGISTRY)/machine-agent:$(VERSION); \
		docker push $(DISTRO_REGISTRY)/controller:$(VERSION); \
		docker push $(DISTRO_REGISTRY)/machine-agent:$(VERSION); \
	fi

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

# Deploy infrastructure only: use existing images or module defaults
deploy-infrastructure:
	@set -e ;\
	ARGS="" ;\
	if [ -f "build/machine-agent-image.txt" ]; then \
		MACHINE_AGENT_IMAGE_TAG=$$(cat build/machine-agent-image.txt) ;\
		ARGS="$${ARGS} -var=\"machine_agent_image=$${MACHINE_AGENT_IMAGE_TAG}\"" ;\
		echo "Machine agent image: $${MACHINE_AGENT_IMAGE_TAG}" ;\
	fi ;\
	if [ -f "build/controller-image.txt" ]; then \
		CONTROLLER_IMAGE_TAG=$$(cat build/controller-image.txt) ;\
		ARGS="$${ARGS} -var=\"controller_image=$${CONTROLLER_IMAGE_TAG}\"" ;\
		echo "Controller image: $${CONTROLLER_IMAGE_TAG}" ;\
	fi ;\
	echo "Deploying infrastructure only with OpenTofu..." ;\
	tofu -chdir=deploy apply $${ARGS} -auto-approve

# Deploy machine-agent only: build machine-agent image and deploy infrastructure
deploy-machine-agent: build-machine-agent-image
	@set -e ;\
	if [ ! -f "build/machine-agent-image.txt" ]; then \
		echo "Error: Machine agent image tag file not found. Run 'make build-machine-agent-image' first." ;\
		exit 1 ;\
	fi ;\
	MACHINE_AGENT_IMAGE_TAG=$$(cat build/machine-agent-image.txt) ;\
	ARGS="-var=\"machine_agent_image=$${MACHINE_AGENT_IMAGE_TAG}\"" ;\
	if [ -f "build/controller-image.txt" ]; then \
		CONTROLLER_IMAGE_TAG=$$(cat build/controller-image.txt) ;\
		ARGS="$${ARGS} -var=\"controller_image=$${CONTROLLER_IMAGE_TAG}\"" ;\
	fi ;\
	echo "Deploying machine-agent and infrastructure with OpenTofu..." ;\
	echo "Machine agent image: $${MACHINE_AGENT_IMAGE_TAG}" ;\
	tofu -chdir=deploy apply $${ARGS} -auto-approve

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
	tofu -chdir=deploy apply -target=module.gcp-cloud-run -var="controller_image=$${CONTROLLER_IMAGE_TAG}" -auto-approve

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
	@echo "  controller-image        - Build controller Docker image and push to Artifact Registry"
	@echo "  machine-agent-image     - Build machine-agent image and push to Artifact Registry"
	@echo "  ci-controller-image     - Build controller image for CI (ghcr.io tag, no push)"
	@echo "  ci-machine-agent-image  - Build machine-agent image for CI (ghcr.io tag, no push)"
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
	@echo "  develop                 - Start backend + frontend hot reload (DB_BACKEND selects Firestore or Dapr)"
	@echo "  test-dapr-integration   - Run DaprDB integration tests against local Datastore emulator"
	@echo ""
	@echo "Dapr Infrastructure (auto-started by 'make develop' when DB_BACKEND=dapr):"
	@echo "  dev-up                  - Start Datastore emulator + daprd sidecar"
	@echo "  dev-down                - Stop all Dapr infrastructure"
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
