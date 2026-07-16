# ──────────────────────────────────────────
# Vars (override with make VAR=foo)
# ──────────────────────────────────────────
GO        ?= go
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
	@echo "✅ All dependencies installed"
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
	docker-compose build app
	docker-compose up
	@echo "🟢  Production services up  |  Access → http://localhost"

.PHONY: prod-gpu
prod-gpu:
	docker compose -f docker-compose.yml -f docker-compose.gpu.yml up --build

.PHONY: prod-pull
prod-pull:
	docker-compose -f docker-compose.yml -f docker-compose.pull.yml pull
	docker-compose -f docker-compose.yml -f docker-compose.pull.yml up
	@echo "🟢  Production services up  |  Access → http://localhost"
# ──────────────────────────────────────────
# BACK-END  (Go + pgx)
# ──────────────────────────────────────────
.PHONY: vet test
vet:
	$(GO) vet ./...
test:
	$(GO) test ./...		
## -race

# ──────────────────────────────────────────
# TESTS
# ──────────────────────────────────────────
.PHONY: integration-test
integration-test:
	RUN_INTEGRATION_TESTS=1 $(GO) test ./internal/integration -v

# ──────────────────────────────────────────
# CLEAN
# ──────────────────────────────────────────
.PHONY: clean
clean:
	rm -rf $(BIN_DIR) $(WEB_DIR)/.svelte-kit/output
	git fetch --prune && git branch --merged | egrep -v "(^\*|main|dev)" | xargs -n 1 git branch -d
	@echo "🧹  Cleaned build artifacts"

.PHONY: clean-all
clean-all: clean
	docker-compose down -v
	@echo "🧹  Cleaned everything including Docker volumes"
