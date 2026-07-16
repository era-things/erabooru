# Erabooru Development Guide

This repository contains a Go backend with a SvelteKit frontend. During development you can run the UI with the Vite dev server for hot reloading while using the existing backend services. PostgreSQL is the source of truth for metadata, tag search, vector search, and background jobs.

## Easiest way to run
You will need Docker and env that can run curl (It can be ran on Windows from git bash)
Run `curl -fsSL https://raw.githubusercontent.com/era-things/erabooru/main/deploy/quickstart.sh | bash`
* It will use quickstart script from deploy/quistart.sh
* It will use `docker-compose.pull.yml` to run everything from pre-built containers

## DEV Prerequisites
- Linux or WSL2 with networkingMode=mirrored
- Docker
- Go
- Node.js and npm
- make

## Source init
Run `make prepare` after cloning to install frontend and Go dependencies.

## DEV launch
Run `make dev` 
* It will create .env file out of .env.example if there is none, you can redact it if needed
* It will run Air for Go hot realoading and Vite dev server for web hot reloading
* It will launch an additional helper container to inspect PostgreSQL
* The UI will be available at `http://localhost:5173`

Development dependencies are installed into a Docker volume and refreshed only when
`package-lock.json` changes. Caddy remains the development proxy so request paths match production.

## Clean-schema changes

This project intentionally has no legacy schema migrations. After pulling a schema redesign,
recreate the local PostgreSQL volume before starting the application:

```sh
docker compose up -d db
docker compose exec -T db psql -U booru -d booru -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
make dev
```

This deletes database metadata and tags but preserves the media and model-cache volumes.
Use Settings → Regenerate after startup to rebuild media metadata from the originals.

## Prod source launch
Run `make prod`
* It will create .env file out of .env.example if there is none, you can redact it if needed
* It will build everything and launch in a way it supposed to be used

## Embedding models
Erabooru no longer ships ONNX model binaries in the repository. The application
downloads the required weights on startup using the settings below:

| Variable | Description | Default |
| --- | --- | --- |
| `MODEL_CACHE_DIR` | Directory for cached models (mount this as a persistent volume in Docker) | `/cache/models` |
| `MODEL_NAME` | Sub-directory inside the cache | `Siglip2_FP16` |
| `MODEL_REPOSITORY` | Hugging Face repository or a full base URL | `onnx-community/siglip2-base-patch16-224-ONNX` |
| `MODEL_REVISION` | Hugging Face revision/branch to resolve | `main` |
| `MODEL_FILES` | Optional override for the files to download (`local|remote|sha`, comma separated) | *(built-in defaults)* |

Media processing uses separate concurrency controls:

| Variable | Description | Default |
| --- | --- | --- |
| `INGEST_CONCURRENCY` | Concurrent metadata and preview jobs | `2` |
| `EMBEDDING_CONCURRENCY` | Concurrent embedding jobs, including preprocessing | `2` |
| `INFERENCE_CONCURRENCY` | Concurrent ONNX model executions | `1` |

If you set `MODEL_DIR` the application skips downloading and loads models directly from the
provided path (useful for local development with pre-downloaded weights).

### Runtime variants

`make prod` builds the CPU application. `make prod-gpu` uses the CUDA 12 application
override and requests all available NVIDIA GPUs. Prebuilt quickstart images use the CPU variant.

