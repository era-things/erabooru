# Erabooru Development Guide

This repository contains a Go backend with a SvelteKit frontend. During development you can run the UI with the Vite dev server for hot reloading while using the existing backend services. Bleve is used for search indexing, and an ent hook keeps the index synchronized with the Postgres metadata.

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
Run `make prepare` after cloning to init node, go, and generate Ent ORM schema

## DEV launch
Run `make dev` 
* It will create .env file out of .env.example if there is none, you can redact it if needed
* It will run Air for Go hot realoading and Vite dev server for web hot reloading
* It will launch additional helper container to inspect postgre and bleve db
* The UI will be available at `http://localhost:5173`

## Prod source launch
Run `make prod`
* It will create .env file out of .env.example if there is none, you can redact it if needed
* It will build everything and launch in a way it supposed to be used

## Embedding models
Erabooru no longer ships ONNX model binaries in the repository. The image embed worker
downloads the required weights on startup using the settings below:

| Variable | Description | Default |
| --- | --- | --- |
| `MODEL_CACHE_DIR` | Directory for cached models (mount this as a persistent volume in Docker) | `/cache/models` |
| `MODEL_NAME` | Sub-directory inside the cache | `Siglip2_FP16` |
| `MODEL_REPOSITORY` | Hugging Face repository or a full base URL | `onnx-community/siglip2-base-patch16-224-ONNX` |
| `MODEL_REVISION` | Hugging Face revision/branch to resolve | `main` |
| `MODEL_FILES` | Optional override for the files to download (`local|remote|sha`, comma separated) | *(built-in defaults)* |

If you set `MODEL_DIR` the embed worker skips downloading and loads models directly from the
provided path (useful for local development with pre-downloaded weights).

## Video hardware acceleration

The image embedding worker samples video frames with `ffmpeg`. By default it
searches for a supported GPU decoder and enables it automatically when
available. You can override the selection or disable the feature entirely with
the following variables in `.env`:

| Variable | Description | Example |
| --- | --- | --- |
| `VIDEO_HWACCEL` | Hardware acceleration profile. Supported value: `cuda`. Leave empty to auto-detect. | `cuda` |
| `VIDEO_HW_OUTPUT_FORMAT` | Optional override for `-hwaccel_output_format`. Defaults to the profile's recommendation. | `cuda` |
| `VIDEO_HW_DEVICE` | Device selector passed to ffmpeg (for CUDA this is the GPU index). Leave empty to use the default device. | `0` |
| `VIDEO_HWACCEL_DISABLE` | Force ffmpeg to use CPU decoding even if a supported accelerator is detected. | `true` |

When no accelerator is detected (or you set `VIDEO_HWACCEL_DISABLE=true`) the
worker keeps its existing CPU-only behavior.
If you enable CUDA acceleration inside Docker you must run the container with
the NVIDIA runtime (`NVIDIA_VISIBLE_DEVICES`, `NVIDIA_DRIVER_CAPABILITIES`) so
that `/dev/nvidia*` devices are available, as shown in the provided compose
files.

