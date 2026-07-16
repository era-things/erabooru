# Erabooru Architecture Redesign

This document is the implementation checklist for the clean-slate redesign. There is no compatibility or data-migration requirement: every phase may replace schemas, APIs, volumes, and deployment topology outright.

## Target

Erabooru will run as one Go application backed by PostgreSQL and a mounted filesystem:

- The Go process serves the API, embedded Svelte application, originals, and previews.
- Uploads stream to a temporary file while being hashed, then atomically move into content-addressed storage.
- PostgreSQL is the only metadata, tag-search, settings, vector-search, and durable-job source.
- River remains only where durable background execution is useful (media ingestion/derivation), not as an RPC mechanism.
- libvips/FFmpeg produce metadata and previews; ONNX Runtime produces image, video, and text embeddings.
- Media processing is idempotent and controlled by database constraints and a small processing state, not defensive branches spread across handlers.

## Explicit removals

- Bleve, its volume, hooks, index worker, index queue, rebuild UI/API, and explorer.
- MinIO, buckets, notifications, presigned URLs, Caddy storage proxying, and MinIO recovery scans.
- Separate media-worker/server wiring and dummy River worker registrations.
- Ent, Atlas, `lib/pq`, and redundant database pools in favor of pgx and a small SQL schema/query layer.
- Eventually Gin in favor of Go's standard HTTP router and typed JSON handlers.

## Phases

### 1. PostgreSQL-only search

- [x] Parse the existing tag/field query language into PostgreSQL predicates.
- [x] Run ordinary listing, hidden filters, tag filters, and vector filters in PostgreSQL.
- [x] Remove Bleve indexing and synchronization.
- [x] Remove the index queue and worker.
- [x] Remove Bleve deployment/configuration and its direct dependency. Transitive module cleanup will happen with the persistence rewrite.
- [x] Replace “regenerate index” with a storage repair/reprocess scan.

### 2. Filesystem media storage

- [x] Define mounted paths for originals, previews, and atomic upload temporaries.
- [x] Stream uploads through Go while computing the content hash.
- [x] Atomically install content-addressed originals and enqueue a unique ingestion job.
- [x] Serve originals/previews through Go's range-capable file server and set cache policy at the proxy.
- [x] Remove MinIO and notification-based ingestion.
- [x] Make deletion database-first and storage cleanup idempotent.

### 3. Process and queue consolidation

- [x] Register the real media worker in the primary application.
- [x] Remove the separate media-worker binary, image, service, and test wiring.
- [x] Replace dummy job registrations with real worker registrations.
- [x] Remove the remaining inference client-type queue configuration.
- [x] Call text embedding directly instead of inserting and polling a River job.
- [x] Establish one explicit media/inference concurrency budget.
- [x] Make HTTP and River shutdown context-aware; FFmpeg receives job contexts and ONNX resources terminate with the process.

### 4. Persistence simplification

- [x] Replace Ent and generated entities with pgx queries and one shared pool.
- [x] Introduce a clean schema for media, tags, tag relations, settings, and fixed-dimension embeddings.
- [x] Store size, upload timestamp, and duration directly; River owns durable processing state.
- [x] Use database constraints/upserts for idempotency and invariants.
- [x] Add a 768-dimension inner-product HNSW index for the configured SigLIP model.
- [x] Replace generic settings indirection with a singleton row and direct foreign key.

### 5. Media pipeline performance

- [x] Generate bounded previews for images as well as videos.
- [x] Probe/decode each local file without repeated object-store downloads.
- [x] Keep preview extraction and embedding samples at their separate durable-job boundaries; combining them would require persisting transient frames or making successful ingestion retry when inference fails.
- [x] Replace duration-proportional sampling with an adaptive 5–16 frame budget.
- [x] Batch video-frame ONNX inference in one model invocation.
- [x] Enable full graph optimization and explicit intra/inter-op threading.

### 6. Deferred work

Frontend component simplification, npm audit remediation, broad dependency upgrades, and the lower-priority edge-case list are intentionally deferred until the architecture stops moving.

## Progress log

- 2026-07-16: Review completed; target architecture accepted. Phase 1 started.
- 2026-07-16: PostgreSQL became the sole search source; Bleve infrastructure was removed.
- 2026-07-16: MinIO was replaced by mounted filesystem storage; uploads, previews, repair scans, deletion, and integration helpers now use local files.
- 2026-07-16: Durable media processing moved into the primary server and the separate media-worker deployment was removed. The native embedding process remains as an explicit CPU/GPU boundary.
- 2026-07-16: Native inference moved into the primary application. River-as-RPC, the embedding worker binary/service, and client-type queue configuration were removed; text search now calls inference directly.
- 2026-07-16: Ent and its generated tree were replaced by one pgx pool and a compact constrained schema; the obsolete dependency graph was removed.
- 2026-07-16: Image previews, adaptive video sampling, batched frame inference, HNSW vector search, and explicit ONNX threading were added.
- 2026-07-16: Uploads became a single server-hashed streaming operation with atomic content-addressed installation; browser-side whole-file hashing and its dependency were removed.
- 2026-07-16: The clean-schema lifecycle passed live pgvector integration tests and the native CPU production image built and linked successfully.
