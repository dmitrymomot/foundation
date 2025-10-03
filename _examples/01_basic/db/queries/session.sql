-- name: GetSessionByTokenHash :one
SELECT * FROM sessions
WHERE token_hash = $1
  AND expires_at > CURRENT_TIMESTAMP;

-- name: UpsertSession :one
INSERT INTO sessions (id, token_hash, device_id, user_id, data, expires_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE
SET token_hash = EXCLUDED.token_hash,
    device_id = EXCLUDED.device_id,
    user_id = EXCLUDED.user_id,
    data = EXCLUDED.data,
    expires_at = EXCLUDED.expires_at,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: DeleteSessionByID :exec
DELETE FROM sessions
WHERE id = $1;
