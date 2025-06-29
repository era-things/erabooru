# ──────────────────────────────────────────
# Vars (override with make VAR=foo)
# ──────────────────────────────────────────
GO        ?= go
GOA_ENV   := PATH="$(HOME)/go/bin:$$PATH"
WEB_DIR   ?= web
BIN_DIR   ?= bin


# ──────────────────────────────────────────
# SETUP
# ──────────────────────────────────────────
.PHONY: setup-env
setup-env:
	@if [ ! -f .env ]; then \
		echo "→ Creating .env file from .env.example..."; \
		cp .env.example .env; \
		echo "✅ Created .env file. You may want to edit it before continuing."; \
	fi

.PHONY: prepare
prepare: setup-env
	@echo "→ Installing frontend dependencies..."
	cd $(WEB_DIR) && npm install
	@echo "→ Downloading Go modules..."
	$(GO) mod download
	@echo "→ Generating Ent schema..."
	$(GO) generate ./ent
	@echo "✅ All dependencies installed and schemas generated"
	@echo ""
	@echo "🟢 Ready for development! You can now run:"
	@echo "   make dev    # Start development servers"
	@echo "   make prod   # Build and start production"

# ──────────────────────────────────────────
# BUILD
# ──────────────────────────────────────────
.PHONY: dev
dev: setup-env
	docker-compose -f docker-compose.yml -f docker-compose.dev.yml up --build --remove-orphans
	@echo "🟢  Dev services up  |  API → http://localhost:8080  UI (vite) → http://localhost:5173"

.PHONY: prod
prod:
	docker-compose build app video-worker
	docker-compose up
	@echo "🟢  Production services up  |  Access → http://localhost"

.PHONY: prod-pull
prod-pull:
	docker-compose -f docker-compose.yml -f docker-compose.pull.yml pull
	docker-compose -f docker-compose.yml -f docker-compose.pull.yml up
	@echo "🟢  Production services up  |  Access → http://localhost"
# ──────────────────────────────────────────
# BACK-END  (Go + Ent)
# ──────────────────────────────────────────
.PHONY: generate
generate: generate-go generate-ts

.PHONY: generate-go
generate-go:
	@echo "→ Generating Go code..."
	$(GO) generate ./ent
	$(GO) run goa.design/goa/v3/cmd/goa@latest gen era/booru/design
	oapi-codegen -config design/oapi-codegen.yaml gen/http/openapi3.json

.PHONY: generate-ts
generate-ts:
	@echo "→ Generating TypeScript types..."
	@if [ ! -f gen/http/openapi3.json ]; then \
		echo "❌ OpenAPI spec not found at gen/http/openapi3.json"; \
		echo "   Make sure 'make generate-go' runs first"; \
		exit 1; \
	fi
	@echo "→ Generating TypeScript API types..."
	cd $(WEB_DIR) && npx openapi-typescript ../gen/http/openapi3.json -o src/lib/types/api.d.ts
	@echo "✅ TypeScript generation complete"


.PHONY: vet test
vet:
	$(GO) vet ./...
test:
	$(GO) test ./...		
## -race

# ──────────────────────────────────────────
# CLEAN
# ──────────────────────────────────────────
.PHONY: clean
clean:
	rm -rf $(BIN_DIR) $(WEB_DIR)/.svelte-kit/output
	@echo "🧹  Cleaned build artifacts"

.PHONY: clean-all
clean-all: clean
	docker-compose down -v
	sudo rm -rf bleve-index minio-data
	@echo "🧹  Cleaned everything including Docker volumes"
