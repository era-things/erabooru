# User metadata export and import

Settings exports a gzip-compressed NDJSON archive through
`GET /api/admin/export-tags`; restore it with `POST /api/admin/import-tags`.
The existing filename and endpoints are retained for compatibility.

Version 3 starts with a header containing `version` and `createdAt`. Each following
record contains only source metadata:

- `id`: content hash used to match existing media.
- `original_name`: original uploaded filename, which cannot be recovered from the hash.
- `tags`: assigned tag names, including the editable `tagme` marker if present.
- `negative_tags`: explicitly rejected tag names; an empty array means no rejections.
- `dates`: named calendar dates in `YYYY-MM-DD` format.
- `uploaded_at`: upload timestamp with full database precision, retained separately
  from the calendar date.

All media records are included, even when processing is queued or has failed.
The archive contains no media bytes, previews, dimensions, embeddings, statistics,
classifier weights, search indexes, or processing jobs. Those can be regenerated
from the media and source metadata. Global application settings are not part of
this per-media archive.

Import requires the media to exist already and skips unknown content hashes. It
replaces each matching item's assigned tags, rejected tags, dates, and supplied
original filename in one transaction. Other items are untouched. Tag statistics
are updated and affected classifiers marked for retraining through the ordinary
metadata-edit hooks; importing derived data is unnecessary. Repeating an import
does not duplicate feedback or its statistical contribution.

Versions 1 and 2 remain readable, including their date-only values. These older
archives do not replace existing rejected feedback or filenames; assigning a
positive tag still clears any contradictory rejection. Version 3 rejects missing
rejection lists, contradictory positive/negative tags, malformed dates, and
duplicate date names. Unsupported versions are rejected.

Each record is atomic, rather than the entire archive: if a later record is
invalid or the stream is damaged, earlier records may already have been restored.
Correct the archive and retry it to finish the import.
