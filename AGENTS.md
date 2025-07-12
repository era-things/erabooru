# AGENTS Instructions

## Project Overview
Erabooru is a web application composed of a Go backend and a SvelteKit frontend. The backend exposes a REST API and background workers, while the frontend provides the user interface. Docker and docker-compose are used for local development and production builds.

From a user perspective, Erabooru works like a personal "booru" site. You upload images or videos and the system processes them in the background, generates previews, and indexes them for fast search. The web UI then lets you browse, tag and search your collection.

### Backend stack
- **Go 1.24**
- **Gin** HTTP framework
- **Ent** ORM
- **PGX** PostgreSQL driver
- **pgvector** extension for vector data
- **Bleve** search engine
- **MinIO** for object storage
- **River** queue system – tasks like media processing and index updates run via workers in `cmd/media_worker`
- **Testcontainers** for integration tests

### Frontend stack
- **SvelteKit 5** with rune syntax
- **TypeScript**
- **Vite** bundler
- **Prettier** and **ESLint** for formatting
- Uses the **runed** library for additional rune helpers

Related instructions for the `web/` directory (including the rune cheatsheet) live in [`web/AGENTS.md`](web/AGENTS.md).

## Repository layout
- `cmd/` – main binaries (`server`, `media_worker`, etc.)
- `internal/` – application packages
- `ent/` – generated Ent ORM code
- `web/` – SvelteKit frontend

## Common tasks
- `make prepare` – install Node packages, download Go modules and generate Ent schema
- `make dev` – start backend and frontend with hot reload
- `make prod` – build and run the full stack
- `go vet ./...` – run static analysis on Go code
- `go test ./...` – run unit tests (integration tests live under `internal/integration`)

Before submitting changes that touch Go code, run `go vet ./...` and `go test ./...`. For frontend changes see the lint instructions in `web/AGENTS.md`.
