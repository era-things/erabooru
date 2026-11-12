# Checking media before uploading

Before decoding or uploading a file, clients can compute its content ID and call
`HEAD /api/media/<id>`. The ID is the unseeded XXH3-128 hash of the exact file
bytes, encoded as 32 lowercase hexadecimal characters (high 64 bits first).
For example, the bytes `hashed` have ID `afd9fe2e29bcfac47456c5db865d13ef`.
Renaming a file does not change its ID.

The response has no body:

| Status | Meaning |
| --- | --- |
| 200 | An uploaded media record exists; skip decoding and transfer. |
| 404 | No media record exists; upload normally. |
| 400 | Invalid content ID. |
| 500 | Lookup failed; do not treat this as a missing file. |

Responses use `Cache-Control: no-store`. Network errors must also be reported
rather than treated as missing files.

Queued, processing and failed records count as existing, including hidden media.
Server-side processing failures should be retried through repair; uploading the
same bytes again does not restart processing. Deleted media returns 404 even if
background file cleanup has not finished. The check is advisory: uploads still
perform transactional duplicate detection if another client uploads concurrently.

The file watcher hashes the same in-memory bytes it will upload, checks this
endpoint, and returns its normal duplicate result on 200. Duplicate progress,
automatic tags and date rules continue to work. Missing files follow the regular
upload or local GPU import path. Files that failed local preprocessing before
uploading will be missing and therefore retried on the next folder upload.
