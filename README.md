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
* The UI and live reload will be available at `http://localhost` through Caddy. Vite stays inside Docker, so other projects can use host port 5173.

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
Use Settings â†’ Regenerate after startup to rebuild media metadata from the originals.

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
| `INFERENCE_CONCURRENCY` | Inference admission capacity; vision batches run serially | `1` |
| `DECODE_CONCURRENCY` | Shared FFmpeg preview / video extraction jobs | `1` |
| `FFMPEG_THREADS` | Decoder threads per FFmpeg process (filter threads remain 1) | `1` |
| `VIPS_CONCURRENCY` | libvips threads per image preparation operation | `1` |
| `ONNX_INTRA_THREADS` | Threads per ONNX session | `(available CPUs - 1) / inference concurrency`, clamped to 1–3 |
| `EMBEDDING_BATCH_SIZE` | Maximum shared image/video inference batch | `4` |
| `EMBEDDING_BATCH_TARGET_MS` | Adaptive batch execution target, not a hard deadline | `150` |

Text searches take priority over background embeddings. New background inference
waits while searches are queued or running. With inference concurrency above one,
one slot is reserved for searches; background work can use the remaining slots.
At concurrency one, a search waits for the current bounded batch to finish,
then runs before the next background call. Images and video frames share a bounded
batcher with a maximum 5 ms collection window. It starts at one frame, grows after
fast calls, and shrinks after calls exceed the target. Running ONNX calls are not
interrupted, so a slow single frame can still exceed the target. Image preparation
runs outside inference admission, bounded by embedding worker concurrency.

Videos shorter than 60 seconds (or with unknown duration) use sequential sampling.
Longer videos seek to individual sample timestamps instead of decoding through the
whole clip. The number of sampled frames and normalized averaging stay unchanged;
sampled timestamps differ slightly between the two strategies. Seeking performance
depends on codec, keyframe spacing and storage. All FFmpeg frame extraction and
preview jobs share the decode limit. Browser uploads use three concurrent requests
and an independent sequential preview queue.

Start conservatively on an N100: the defaults use one decoder thread per process,
one libvips thread per operation, and at most three ONNX threads per session,
leaving at least one CPU out of the budget on a four-core server.
For CUDA, try increasing `EMBEDDING_CONCURRENCY` and `EMBEDDING_BATCH_SIZE` to 8;
measure before increasing decoding or inference concurrency. These are tuning
candidates, not measured optimal settings.

Each successfully processed media item emits one `benchmark` summary, after
embedding completes (or after ingestion when embeddings were supplied / disabled).
It includes preview, preparation, video decoding, inference and admission-wait
timings in milliseconds, plus frame count, batch count and maximum batch size.
`total_ms` measures wall time from the ingestion attempt's start, including the
wait for its embedding job. Failed attempts emit their summary at the failing
stage with the error. Embedding jobs queued directly, such as admin retries,
measure total time from their embedding attempt's start.

Inference timing counts each batch once per item, even when several frames from
one video share that batch. When a batch contains frames from multiple items,
its execution duration appears in each item's summary; these durations should
not be summed across items. Text searches retain their separate timing line.

### Uploading media and preserving its original name

Clients can [check a content hash before uploading](docs/upload-existence.md) to skip existing media without decoding or transferring it.

`POST /api/media` accepts streaming `multipart/form-data` with exactly one `file` part. The part must include its original filename (including extension) and an `image/*` or `video/*` Content-Type. Raw request-body uploads are no longer accepted.

```sh
curl --fail-with-body http://localhost/api/media -F 'file=@2026-11-09T13-04-05Z.png;type=image/png'
```

The server stores the basename verbatim as `original_name`, including Unicode and timestamp text. Paths are stripped; names are metadata, never storage paths. Detail, list, preview and similarity responses include this field, and the media details screen displays it. Duplicate uploads preserve the first recorded name. Existing media whose name was already lost has an empty name. Ingestion, repair and embedding jobs preserve recorded names. Timestamp names are kept as text without interpreting or changing their dates.

### Importing media with locally computed embeddings (API only)

`GET /api/embeddings/profile` returns the active embedding profile: `id`,
`dimensions` (768), `image_size`, SHA-256 hashes of the vision/text models and
tokenizer, and preprocessing / aggregation identifiers. The ID fingerprints the
actual loaded files and the processing contract, rather than a mutable model
repository name. Both endpoints return 503 if no embedding model is loaded.

`POST /api/media/import` accepts a streaming `multipart/form-data` body with
**exactly two parts in this order**:

1. `embedding`: one JSON document, at most 64 KiB:
   `{"profile":"<id from the profile endpoint>","vector":[<768 numbers>]}`.
2. `file`: the original image or video, with an `image/*` or `video/*` part
   Content-Type. Its original filename is required and saved as `original_name`; it is not used as a storage path.

For example, after saving the metadata as `embedding.json`:

```sh
curl http://localhost/api/embeddings/profile
curl --fail-with-body http://localhost/api/media/import \
  -F 'embedding=@embedding.json;type=application/json' \
  -F 'file=@image.png;type=image/png'
# For a video, use: -F 'file=@video.mp4;type=video/mp4'
```

The vector must contain 768 finite numbers with a nonzero norm; the server L2
normalizes it before storing it. Profile mismatches return 409. Invalid metadata
returns 422; malformed multipart requests / extra parts return 400. The server
checks metadata before streaming the media to disk, without buffering the whole
image/video in memory.

Use the same model weights and preprocessing on the GPU machine. For images,
match preview generation and `PrepareImage` in `internal/embeddings/vision.go`:
center-cropped RGB input at the advertised size, black alpha flattening and
normalization to [-1, 1]. For video, supply **one final vector**, obtained by L2
normalizing each sampled frame embedding, averaging those vectors, then L2
normalizing the mean. Match frame selection / letterboxing in
`internal/workers/embedworker/image_embed_worker.go`. The uploader is responsible
for calculating the supplied vector for the supplied file; the server does not
rerun inference to verify it.

A successful new import returns 201 with `{"id":"...","existed":false}`. The
original, ready vector and durable metadata/preview job are recorded together.
Media becomes browsable/searchable after background metadata and preview
processing; that processing preserves the imported vector and skips embedding
inference. Import retries for the same content return 200 with `existed:true`
and preserve the existing item/vector. This endpoint does not replace embeddings
on existing items. Ordinary `POST /api/media` uploads continue to generate
embeddings on the server. The frontend does not expose the import endpoint.

### Runtime variants

`make prod` builds the CPU application. `make prod-gpu` uses the CUDA 12 application
override and requests all available NVIDIA GPUs. Prebuilt quickstart images use the CPU variant.

