# ──────────────────────────────────────────
# Vars (override with make VAR=foo)
# ──────────────────────────────────────────
GO        ?= go
WEB_DIR   ?= web
ASSET_DIR ?= internal/assets
BIN_DIR   ?= bin


# ──────────────────────────────────────────
# BUILD
# ──────────────────────────────────────────
.PHONY: dev
dev: 
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
generate:
	$(GO) generate ./ent

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
	rm -rf $(BIN_DIR) $(ASSET_DIR)/build $(WEB_DIR)/.svelte-kit/output
