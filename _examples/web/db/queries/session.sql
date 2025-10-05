-- name: GetSessionByToken :one
SELECT * FROM sessions
WHERE token = $1
  AND expires_at > CURRENT_TIMESTAMP;

-- name: GetSessionByID :one
SELECT * FROM sessions
WHERE id = $1
  AND expires_at > CURRENT_TIMESTAMP;

-- name: UpsertSession :one
INSERT INTO sessions (id, token, fingerprint, ip_address, user_agent, user_id, data, expires_at, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
ON CONFLICT (id) DO UPDATE
SET token = EXCLUDED.token,
    fingerprint = EXCLUDED.fingerprint,
    ip_address = EXCLUDED.ip_address,
    user_agent = EXCLUDED.user_agent,
    user_id = EXCLUDED.user_id,
    data = EXCLUDED.data,
    expires_at = EXCLUDED.expires_at,
    updated_at = EXCLUDED.updated_at
RETURNING *;

-- name: DeleteSessionByID :exec
DELETE FROM sessions
WHERE id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE expires_at <= CURRENT_TIMESTAMP;
