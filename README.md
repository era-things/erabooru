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

If you set `MODEL_DIR` the application skips downloading and loads models directly from the
provided path (useful for local development with pre-downloaded weights).

### Runtime variants

`make prod` builds the CPU application. `make prod-gpu` uses the CUDA 12 application
override and requests all available NVIDIA GPUs. Prebuilt quickstart images use the CPU variant.

