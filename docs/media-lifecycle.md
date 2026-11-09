# Media lifecycle and recovery

`GET /api/media/:id` returns existing media while ingestion is still running.
Its `status` is `queued`, `processing`, `retrying`, `ready`, or `failed`;
`error` describes the latest unsuccessful attempt. `retrying` means River has
attempts remaining. `failed` means processing was cancelled or exhausted its
attempts. Lists and searches include only ready media. Pending media has empty
format and zero dimensions until metadata processing completes.

Embedding entries expose the same statuses and an `error`. The detail response's
`embedding_availability` is `available`, `pending`, `unavailable`, or `missing`.
A ready imported vector remains available even when local inference is disabled.
Clients poll active ingestion and embedding jobs, and stop for terminal failure,
unavailable embeddings, or missing embeddings on settled media.

Deletion returns 204 after a deletion marker and durable cleanup job commit
together. Repeated deletes, including missing IDs, also return 204. File removal
happens in the background with up to 25 attempts. Repair never restores a marked
item; it can retry exhausted cleanup jobs and recover ingestion or embedding rows
without an active River job. Uploading the same content explicitly creates a
fresh generation, protected from older cleanup jobs. Deletion markers remain
after cleanup until that explicit upload.

Publication, preview installation, deletion, repair, and cleanup use a PostgreSQL
transaction advisory lock per content ID. Incoming bytes stay in private staging
files outside the served directories until multipart validation completes and
publication holds that lock.
Repair ignores staging files. Worker completions and errors also check the media
generation and reject changes to terminal states.

Schema version 3 upgrades version 2 in place, adding retry states and deletion
markers. No database reset is needed for this upgrade. Interrupted processes can
leave application status behind River's final job state; the repair action
recovers rows once no active River job remains.

## Verification

`go test ./...` runs unit tests. Set `TEST_DATABASE_DSN` to an isolated PostgreSQL
database with pgvector to enable API and database lifecycle regressions. These
cover retry exhaustion, pending detail responses, terminal-state guards, failed
cleanup, repair, re-upload, and stale cleanup jobs. The media import regression
also needs ffmpeg. Frontend polling tests run with `npm run test` in `web`.
