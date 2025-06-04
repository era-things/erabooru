# Erabooru Development Guide

This repository contains a Go backend with a SvelteKit frontend. During development you can run the UI with the Vite dev server for hot reloading while using the existing backend services.

## Prerequisites
- Docker (for Postgres and MinIO services)
- Go
- Node.js and npm

## Starting the stack
1. Start the databases with Docker Compose:
   ```sh
   docker compose up -d
   ```
2. In a separate terminal, start the Go API server (for example with [Air](https://github.com/cosmtrek/air)):
   ```sh
   air
   ```
   The API listens on `http://localhost:8080`.
3. Run the Vite dev server for the UI:
   ```sh
   cd web && npm install && npm run dev
   ```
   The UI will be available at `http://localhost:5173` and will communicate with the API.

With this setup you can edit the Svelte application and see changes instantly while still interacting with the backend databases.

## Dockerized workflow

A `Dockerfile` and compose configuration allow running everything in containers.

### Development

Launch the backend, databases and the Vite dev server with:

```sh
docker compose -f docker-compose.yml -f docker-compose.dev.yml up --build
```

The API will be available at `http://localhost:8080` and the UI with hot reload at `http://localhost:5173`.

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
