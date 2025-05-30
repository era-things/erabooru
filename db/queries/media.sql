-- name: InsertMedia :one
INSERT INTO media (hash, mime_type, width, height, duration)
VALUES ($1, $2, $3, $4, $5)
RETURNING id;

-- name: GetMediaByHash :one
SELECT * FROM media WHERE hash = $1 LIMIT 1; 