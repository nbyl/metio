.PHONY: all build clean build-images deploy deploy-full deploy-infrastructure deploy-machine-agent deploy-controller check-images use-default-images cleanup-old-images install-web build-web test test-backend test-web develop lint-web verify-backend ci-controller-image ci-machine-agent-image ci-mc-backup-image ci-daprd-image controller-image machine-agent-image mc-backup-image daprd-image push-images promote promote-distribution help dev-up dev-down dev-dapr-setup test-dapr-integration

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
# Dapr sidecar + local Postgres are always started (and torn down on exit).
develop: generate-env
	@echo "Starting development servers..."
	@set -a; . build/local.env; set +a; \
	echo "Starting local Dapr infrastructure..."; \
	$(MAKE) dev-up || exit 1; \
	trap 'make dev-down; kill 0' EXIT; \
	cd web && npm run dev & \
	air & \
	wait

# ──────────────────────────────────────────────
# Dapr local development targets
# ──────────────────────────────────────────────

DAPRD_BIN ?= $(HOME)/.dapr/bin/daprd
POSTGRES_PORT ?= 5432
POSTGRES_CONTAINER ?= metio-postgres
POSTGRES_IMAGE ?= postgres:18
POSTGRES_DB ?= metio
LOCAL_SECRETS_FILE ?= dapr/secrets/secrets.json

dev-dapr-setup: ## Initialize Dapr runtime if not already done
	@if [ ! -f "$(DAPRD_BIN)" ]; then \
		echo "Running dapr init --slim to download the daprd binary..."; \
		dapr init --slim; \
	else \
		echo "daprd already installed at $(DAPRD_BIN)"; \
	fi

dev-up: dev-dapr-setup ## Start Dapr infrastructure (Postgres container + daprd)
	@if [ ! -f "$(LOCAL_SECRETS_FILE)" ]; then \
		echo "Creating local secrets file $(LOCAL_SECRETS_FILE)..."; \
		mkdir -p $$(dirname $(LOCAL_SECRETS_FILE)); \
		printf '{\n  "postgres-connection-string": "host=localhost user=postgres password=postgres port=$(POSTGRES_PORT) connect_timeout=10 database=$(POSTGRES_DB)"\n}\n' > $(LOCAL_SECRETS_FILE); \
	fi
	@echo "Starting Postgres container..."
	@docker rm -f $(POSTGRES_CONTAINER) 2>/dev/null || true
	@docker run -d --name $(POSTGRES_CONTAINER) \
		-p $(POSTGRES_PORT):5432 \
		-e POSTGRES_PASSWORD=postgres \
		-e POSTGRES_DB=$(POSTGRES_DB) \
		-v metio-postgres-data:/var/lib/postgresql \
		$(POSTGRES_IMAGE) \
		&> /tmp/postgres-container.log
	@echo "Waiting for Postgres to be ready..."
	@ready=0; \
	for i in $$(seq 1 30); do \
		if docker exec $(POSTGRES_CONTAINER) pg_isready -U postgres -d $(POSTGRES_DB) > /dev/null 2>&1; then \
			ready=1; \
			break; \
		fi; \
		sleep 1; \
	done; \
	if [ "$$ready" -eq 0 ]; then \
		echo "ERROR: Postgres did not become ready within 30s"; \
		echo "--- /tmp/postgres-container.log ---"; \
		cat /tmp/postgres-container.log; \
		echo "--- end ---"; \
		docker logs $(POSTGRES_CONTAINER) 2>&1; \
		exit 1; \
	fi
	@echo "Postgres ready on localhost:$(POSTGRES_PORT) (db: $(POSTGRES_DB))"
	@echo "Starting daprd..."
	@$(DAPRD_BIN) --app-id controller \
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
	@echo "  Postgres:      localhost:$(POSTGRES_PORT) (db: $(POSTGRES_DB))"
	@echo "  daprd (gRPC):  localhost:50001"
	@echo ""
	@echo "Run 'make dev-down' to stop."

dev-down: ## Stop Dapr infrastructure
	@echo "Stopping daprd..."
	@pkill daprd 2>/dev/null || true
	@echo "Stopping Postgres container..."
	@docker rm -f $(POSTGRES_CONTAINER) 2>/dev/null || true
	@echo "Dapr infrastructure stopped."


test-dapr-integration: dev-up ## Run DaprDB integration tests against the local Postgres
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

# Version baked into controller/machine-agent binaries via ldflags
# (git describe, e.g. "1.7.0" or "1.7.0-3-gabc1234"; override with VERSION=x make ...)
VERSION ?= $(shell git describe --tags --always 2>/dev/null | sed 's/^v//' || echo local)

# CI controller image build — tag for ghcr.io, load into local daemon (no push)
ci-controller-image:
	@SHA=$$(git rev-parse --short HEAD); \
	echo "Building controller image for CI: ghcr.io/nbyl/metio/controller:$${SHA}"; \
	docker buildx build --platform linux/amd64 -t ghcr.io/nbyl/metio/controller:$${SHA} --build-arg VERSION=$(VERSION) -f cmd/controller/Dockerfile --load .

# CI machine-agent image build — tag for ghcr.io, load into local daemon (no push)
ci-machine-agent-image:
	@SHA=$$(git rev-parse --short HEAD); \
	echo "Building machine-agent image for CI: ghcr.io/nbyl/metio/machine-agent:$${SHA}"; \
	docker buildx build --platform linux/amd64 -t ghcr.io/nbyl/metio/machine-agent:$${SHA} --build-arg VERSION=$(VERSION) -f cmd/machine-agent/Dockerfile --load .

# CI mc-backup image build — tag for ghcr.io, load into local daemon (no push)
ci-mc-backup-image:
	@SHA=$$(git rev-parse --short HEAD); \
	echo "Building mc-backup image for CI: ghcr.io/nbyl/metio/mc-backup:$${SHA}"; \
	docker buildx build --platform linux/amd64 -t ghcr.io/nbyl/metio/mc-backup:$${SHA} -f cmd/mc-backup/Dockerfile --load .

# CI daprd image build — tag for ghcr.io, load into local daemon (no push)
ci-daprd-image:
	@SHA=$$(git rev-parse --short HEAD); \
	echo "Building daprd image for CI: ghcr.io/nbyl/metio/daprd:$${SHA}"; \
	docker buildx build --platform linux/amd64 -t ghcr.io/nbyl/metio/daprd:$${SHA} -f cmd/daprd/Dockerfile --load .

# Local controller image build + push to Artifact Registry
controller-image:
	@mkdir -p build
	@SHA=$$(git rev-parse --short HEAD); \
	IMAGE="europe-west3-docker.pkg.dev/minecraftbyl/metio/controller:$${SHA}"; \
	echo "Building controller image: $${IMAGE}"; \
	docker buildx build --platform linux/amd64 -f cmd/controller/Dockerfile -t $${IMAGE} --build-arg VERSION=$(VERSION) --push . && \
	echo "$${IMAGE}" > build/controller-image.txt

# Local machine-agent image build + push to Artifact Registry
machine-agent-image:
	@mkdir -p build
	@SHA=$$(git rev-parse --short HEAD); \
	IMAGE="europe-west3-docker.pkg.dev/minecraftbyl/metio/machine-agent:$${SHA}"; \
	echo "Building machine-agent image: $${IMAGE}"; \
	docker buildx build --platform linux/amd64 -f cmd/machine-agent/Dockerfile -t $${IMAGE} --build-arg VERSION=$(VERSION) --push . && \
	echo "$${IMAGE}" > build/machine-agent-image.txt

# Local mc-backup image build + push to Artifact Registry
mc-backup-image:
	@mkdir -p build
	@SHA=$$(git rev-parse --short HEAD); \
	IMAGE="europe-west3-docker.pkg.dev/minecraftbyl/metio/mc-backup:$${SHA}"; \
	echo "Building mc-backup image: $${IMAGE}"; \
	docker buildx build --platform linux/amd64 -f cmd/mc-backup/Dockerfile -t $${IMAGE} --push . && \
	echo "$${IMAGE}" > build/mc-backup-image.txt

# Local daprd image build + push to Artifact Registry
daprd-image:
	@mkdir -p build
	@SHA=$$(git rev-parse --short HEAD); \
	IMAGE="europe-west3-docker.pkg.dev/minecraftbyl/metio/daprd:$${SHA}"; \
	echo "Building daprd image: $${IMAGE}"; \
	docker buildx build --platform linux/amd64 -f cmd/daprd/Dockerfile -t $${IMAGE} --push . && \
	echo "$${IMAGE}" > build/daprd-image.txt

# Push images to ghcr.io
push-images:
	docker push ghcr.io/nbyl/metio/controller:$(shell git rev-parse --short HEAD)
	docker push ghcr.io/nbyl/metio/machine-agent:$(shell git rev-parse --short HEAD)
	docker push ghcr.io/nbyl/metio/mc-backup:$(shell git rev-parse --short HEAD)
	docker push ghcr.io/nbyl/metio/daprd:$(shell git rev-parse --short HEAD)

# Promote image tags (usage: make promote FROM=a1b2c3d4 TO=main)
promote:
	@if [ -z "$(FROM)" ] || [ -z "$(TO)" ]; then \
		echo "Usage: make promote FROM=<sha> TO=<tag>"; \
		exit 1; \
	fi
	docker buildx imagetools create -t ghcr.io/nbyl/metio/controller:$(TO) ghcr.io/nbyl/metio/controller:$(FROM)
	docker buildx imagetools create -t ghcr.io/nbyl/metio/machine-agent:$(TO) ghcr.io/nbyl/metio/machine-agent:$(FROM)
	docker buildx imagetools create -t ghcr.io/nbyl/metio/mc-backup:$(TO) ghcr.io/nbyl/metio/mc-backup:$(FROM)
	docker buildx imagetools create -t ghcr.io/nbyl/metio/daprd:$(TO) ghcr.io/nbyl/metio/daprd:$(FROM)

# Promote images from ghcr.io to GCP Artifact Registry (distribution repo)
DISTRO_REGISTRY ?= europe-docker.pkg.dev/metio-distribution/metio
promote-distribution:
	docker tag ghcr.io/nbyl/metio/controller:$(SHA) $(DISTRO_REGISTRY)/controller:$(SHA)
	docker tag ghcr.io/nbyl/metio/machine-agent:$(SHA) $(DISTRO_REGISTRY)/machine-agent:$(SHA)
	docker tag ghcr.io/nbyl/metio/mc-backup:$(SHA) $(DISTRO_REGISTRY)/mc-backup:$(SHA)
	docker tag ghcr.io/nbyl/metio/daprd:$(SHA) $(DISTRO_REGISTRY)/daprd:$(SHA)
	docker push $(DISTRO_REGISTRY)/controller:$(SHA)
	docker push $(DISTRO_REGISTRY)/machine-agent:$(SHA)
	docker push $(DISTRO_REGISTRY)/mc-backup:$(SHA)
	docker push $(DISTRO_REGISTRY)/daprd:$(SHA)
	if [ -n "$(VERSION)" ]; then \
		docker tag ghcr.io/nbyl/metio/controller:$(SHA) $(DISTRO_REGISTRY)/controller:$(VERSION); \
		docker tag ghcr.io/nbyl/metio/machine-agent:$(SHA) $(DISTRO_REGISTRY)/machine-agent:$(VERSION); \
		docker tag ghcr.io/nbyl/metio/mc-backup:$(SHA) $(DISTRO_REGISTRY)/mc-backup:$(VERSION); \
		docker tag ghcr.io/nbyl/metio/daprd:$(SHA) $(DISTRO_REGISTRY)/daprd:$(VERSION); \
		docker push $(DISTRO_REGISTRY)/controller:$(VERSION); \
		docker push $(DISTRO_REGISTRY)/machine-agent:$(VERSION); \
		docker push $(DISTRO_REGISTRY)/mc-backup:$(VERSION); \
		docker push $(DISTRO_REGISTRY)/daprd:$(VERSION); \
	fi

# Build all Docker images (local, without gcloud)
build-images: controller-image machine-agent-image mc-backup-image daprd-image
	@echo "All Docker images built successfully"

# Deploy infrastructure: apply OpenTofu with pre-built Docker images
deploy: deploy-full

# Deploy full system: build all images and deploy all infrastructure
deploy-full: build-images
	@set -e ;\
	if [ ! -f "build/machine-agent-image.txt" ] || [ ! -f "build/controller-image.txt" ] || [ ! -f "build/daprd-image.txt" ] || [ ! -f "build/mc-backup-image.txt" ]; then \
		echo "Error: Image tag files not found. Run 'make build-images' first." ;\
		exit 1 ;\
	fi ;\
	MACHINE_AGENT_IMAGE_TAG=$$(cat build/machine-agent-image.txt) ;\
	CONTROLLER_IMAGE_TAG=$$(cat build/controller-image.txt) ;\
	DAPRD_IMAGE_TAG=$$(cat build/daprd-image.txt) ;\
	MC_BACKUP_IMAGE_TAG=$$(cat build/mc-backup-image.txt) ;\
	DEPLOY_ID=$$(date +%s) ;\
	echo "Deploying full system with OpenTofu..." ;\
	echo "Machine agent image: $${MACHINE_AGENT_IMAGE_TAG}" ;\
	echo "Controller image: $${CONTROLLER_IMAGE_TAG}" ;\
	echo "Daprd image: $${DAPRD_IMAGE_TAG}" ;\
	echo "mc-backup image: $${MC_BACKUP_IMAGE_TAG}" ;\
	tofu -chdir=deploy apply -var="machine_agent_image=$${MACHINE_AGENT_IMAGE_TAG}" -var="controller_image=$${CONTROLLER_IMAGE_TAG}" -var="daprd_image=$${DAPRD_IMAGE_TAG}" -var="backup_image=$${MC_BACKUP_IMAGE_TAG}" -var="deploy_id=$${DEPLOY_ID}" -auto-approve

# Deploy infrastructure only: use existing images or module defaults
deploy-infrastructure:
	@set -e ;\
	DEPLOY_ID=$$(date +%s) ;\
	set -- -var="deploy_id=$${DEPLOY_ID}" ;\
	if [ -f "build/machine-agent-image.txt" ]; then \
		set -- "$$@" -var="machine_agent_image=$$(cat build/machine-agent-image.txt)" ;\
		echo "Machine agent image: $$(cat build/machine-agent-image.txt)" ;\
	fi ;\
	if [ -f "build/controller-image.txt" ]; then \
		set -- "$$@" -var="controller_image=$$(cat build/controller-image.txt)" ;\
		echo "Controller image: $$(cat build/controller-image.txt)" ;\
	fi ;\
	if [ -f "build/daprd-image.txt" ]; then \
		set -- "$$@" -var="daprd_image=$$(cat build/daprd-image.txt)" ;\
		echo "Daprd image: $$(cat build/daprd-image.txt)" ;\
	fi ;\
	if [ -f "build/mc-backup-image.txt" ]; then \
		set -- "$$@" -var="backup_image=$$(cat build/mc-backup-image.txt)" ;\
		echo "mc-backup image: $$(cat build/mc-backup-image.txt)" ;\
	fi ;\
	echo "Deploying infrastructure only with OpenTofu..." ;\
	tofu -chdir=deploy apply "$$@" -auto-approve

# Deploy machine-agent only: build machine-agent image and deploy infrastructure
deploy-machine-agent: machine-agent-image
	@set -e ;\
	if [ ! -f "build/machine-agent-image.txt" ]; then \
		echo "Error: Machine agent image tag file not found. Run 'make machine-agent-image' first." ;\
		exit 1 ;\
	fi ;\
	MACHINE_AGENT_IMAGE_TAG=$$(cat build/machine-agent-image.txt) ;\
	DEPLOY_ID=$$(date +%s) ;\
	set -- -var="machine_agent_image=$${MACHINE_AGENT_IMAGE_TAG}" -var="deploy_id=$${DEPLOY_ID}" ;\
	if [ -f "build/controller-image.txt" ]; then \
		CONTROLLER_IMAGE_TAG=$$(cat build/controller-image.txt) ;\
		set -- "$$@" -var="controller_image=$${CONTROLLER_IMAGE_TAG}" ;\
	fi ;\
	if [ -f "build/daprd-image.txt" ]; then \
		DAPRD_IMAGE_TAG=$$(cat build/daprd-image.txt) ;\
		set -- "$$@" -var="daprd_image=$${DAPRD_IMAGE_TAG}" ;\
		echo "Daprd image: $${DAPRD_IMAGE_TAG}" ;\
	fi ;\
	if [ -f "build/mc-backup-image.txt" ]; then \
		MC_BACKUP_IMAGE_TAG=$$(cat build/mc-backup-image.txt) ;\
		set -- "$$@" -var="backup_image=$${MC_BACKUP_IMAGE_TAG}" ;\
	fi ;\
	echo "Deploying machine-agent and infrastructure with OpenTofu..." ;\
	echo "Machine agent image: $${MACHINE_AGENT_IMAGE_TAG}" ;\
	tofu -chdir=deploy apply "$$@" -auto-approve

# Deploy controller only: build controller and daprd images and update Cloud Run service
deploy-controller: controller-image daprd-image
	@set -e ;\
	if [ ! -f "build/controller-image.txt" ] || [ ! -f "build/daprd-image.txt" ]; then \
		echo "Error: Image tag files not found. Run 'make controller-image daprd-image' first." ;\
		exit 1 ;\
	fi ;\
	CONTROLLER_IMAGE_TAG=$$(cat build/controller-image.txt) ;\
	DAPRD_IMAGE_TAG=$$(cat build/daprd-image.txt) ;\
	DEPLOY_ID=$$(date +%s) ;\
	echo "Deploying controller only..." ;\
	echo "Controller image: $${CONTROLLER_IMAGE_TAG}" ;\
	echo "Daprd image: $${DAPRD_IMAGE_TAG}" ;\
	set -- -var="controller_image=$${CONTROLLER_IMAGE_TAG}" -var="daprd_image=$${DAPRD_IMAGE_TAG}" -var="deploy_id=$${DEPLOY_ID}" ;\
	if [ -f "build/mc-backup-image.txt" ]; then \
		MC_BACKUP_IMAGE_TAG=$$(cat build/mc-backup-image.txt) ;\
		set -- "$$@" -var="backup_image=$${MC_BACKUP_IMAGE_TAG}" ;\
		echo "mc-backup image: $${MC_BACKUP_IMAGE_TAG}" ;\
	fi ;\
	tofu -chdir=deploy apply "$$@" -auto-approve

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
	@echo "Cleaning old local mc-backup images..."
	@gcloud artifacts docker images list "$$LOCATION-docker.pkg.dev/$$PROJECT_ID/metio" --filter="tags~$(USERNAME)-local" --format="value(version)" --limit=10 | tail -n +6 | while read -r digest; do \
		if [ -n "$$digest" ]; then \
			echo "Deleting old image: $$LOCATION-docker.pkg.dev/$$PROJECT_ID/metio/mc-backup@$$digest" ;\
			gcloud artifacts docker images delete "$$LOCATION-docker.pkg.dev/$$PROJECT_ID/metio/mc-backup@$$digest" --quiet || true ;\
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
	@echo "  mc-backup-image         - Build mc-backup image and push to Artifact Registry"
	@echo "  daprd-image             - Build daprd image with baked statestore and push to Artifact Registry"
	@echo "  ci-controller-image     - Build controller image for CI (ghcr.io tag, no push)"
	@echo "  ci-machine-agent-image  - Build machine-agent image for CI (ghcr.io tag, no push)"
	@echo "  ci-mc-backup-image      - Build mc-backup image for CI (ghcr.io tag, no push)"
	@echo "  ci-daprd-image          - Build daprd image for CI (ghcr.io tag, no push)"
	@echo "  push-images             - Push images to ghcr.io"
	@echo "  promote                 - Retag image (make promote FROM=<sha> TO=<tag>)"
	@echo "  build-images            - Build all Docker images (controller, machine-agent, mc-backup, daprd)"
	@echo ""
	@echo "Test targets:"
	@echo "  test                    - Run all tests (backend + frontend)"
	@echo "  test-backend            - Run Go tests with coverage report"
	@echo "  test-web                - Run frontend Vitest suite"
	@echo ""
	@echo "Development:"
	@echo "  develop                 - Start backend + frontend hot reload (Dapr + Postgres)"
	@echo "  test-dapr-integration   - Run DaprDB integration tests against local Postgres"
	@echo ""
	@echo "Dapr Infrastructure (auto-started by 'make develop'):"
	@echo "  dev-up                  - Start Postgres container + daprd sidecar"
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
