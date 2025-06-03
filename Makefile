# ──────────────────────────────────────────
# Vars (override with make VAR=foo)
# ──────────────────────────────────────────
GO        ?= go
NPM       ?= npm
WEB_DIR   ?= web
ASSET_DIR ?= internal/assets
BIN_DIR   ?= bin
BINARY    ?= $(BIN_DIR)/erabooru       # final server binary name

# ──────────────────────────────────────────
# Default target ─ `make`  → starts dev stack
# ──────────────────────────────────────────
.PHONY: dev
dev: compose-up
	@echo "🟢  Dev services up  |  API → http://localhost:8080  UI (vite) → http://localhost:5173"

# ──────────────────────────────────────────
# FRONT-END  (SvelteKit + adapter-static)
# ──────────────────────────────────────────
.PHONY: ui-build
ui-build:                     ## Compile SvelteKit → .svelte-kit/output/client
	cd $(WEB_DIR) && \
	$(NPM) run build

.PHONY: ui-copy
ui-copy: ui-build             ## Copy bundle under internal/assets/build
	@echo "📦  Copying UI bundle ..."
	rm -rf $(ASSET_DIR)/build
	mkdir -p $(ASSET_DIR)
	cp -r $(WEB_DIR)/build $(ASSET_DIR)/build

# ──────────────────────────────────────────
# BACK-END  (Go + Ent)
# ──────────────────────────────────────────
.PHONY: generate
generate:                     ## Regenerate Ent code
	$(GO) generate ./ent

.PHONY: vet test
vet:
	$(GO) vet ./...
test:
	$(GO) test ./...		
## -race

.PHONY: backend-build
backend-build: generate vet test ui-copy ## Build API + worker binaries
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BINARY)     ./cmd/server
	$(GO) build -o $(BIN_DIR)/worker ./cmd/worker
	@echo "🛠  Built $(BINARY) and worker binary"


# ──────────────────────────────────────────
# PRODUCTION ARTIFACT  (`make release`)
# ──────────────────────────────────────────
.PHONY: release
release: ui-copy backend-build
	@echo "✅  Release ready ➜ $(BINARY)"

# ──────────────────────────────────────────
# DOCKER SERVICES (Postgres, MinIO, API autoreload)
# ──────────────────────────────────────────
.PHONY: compose-up compose-down
compose-up:
	docker compose up -d
compose-down:
	docker compose down

# ──────────────────────────────────────────
# CLEAN
# ──────────────────────────────────────────
.PHONY: clean
clean:
	rm -rf $(BIN_DIR) $(ASSET_DIR)/build $(WEB_DIR)/.svelte-kit/output
