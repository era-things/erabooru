# Erabooru Development Guide

This repository contains a Go backend with a SvelteKit frontend. During development you can run the UI with the Vite dev server for hot reloading while using the existing backend services. Bleve is used for search indexing, and an ent hook keeps the index synchronized with the Postgres metadata.

## Prerequisites
- Docker (for Postgres and MinIO services)
- Go
- Node.js and npm

## Starting the stack
1. Copy `.env.example` to `.env` to configure the database, MinIO credentials and Bleve index path. Docker Compose automatically loads these values and mounts `./bleve-index` to `${BLEVE_PATH}` inside the containers. When running the stack entirely in containers, set `MINIO_ENDPOINT=caddy:9000` so the backend uses the proxy.
2. Start the databases with Docker Compose:
   ```sh
   docker compose up -d
   ```
   Caddy will proxy MinIO so presigned URLs resolve to `localhost:9000`.
3. In a separate terminal, start the Go API server with [Air](https://github.com/air-verse/air) for hot reload:
   ```sh
   air
   ```
   The API listens on `http://localhost:8080` and automatically restarts on changes.
4. Run the Vite dev server for the UI:
   ```sh
   cd web && npm install && npm run dev
   ```
   The UI will be available at `http://localhost:5173` and will communicate with the API.

With this setup you can edit the Svelte application and see changes instantly while still interacting with the backend databases.

## Dockerized workflow

A `Dockerfile` and compose configuration allow running everything in containers.

### Development

Launch the backend, databases and the hot-reloading servers with:

```sh
make dev
```

This uses `docker-compose.dev.yml` to run `scripts/dev.sh` inside the container, starting Air and the Vite dev server. The API will be available at `http://localhost:8080`, the UI at `http://localhost:5173`, and MinIO through Caddy on `http://localhost:9000`.

### Production image

Build the release container that serves the prebuilt UI:

```sh
docker compose build app
docker compose up -d app
```

Or directly with Docker:

```sh
docker build -t erabooru .
docker run -p 8080:8080 erabooru
```

The application will then serve the static website on port 8080.
