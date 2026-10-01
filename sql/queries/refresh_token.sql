-- name: CreateRefreshToken :one
INSERT INTO refresh_token(token, created_at, updated_at, user_id, expires_at, revoked_at)
VALUES (
    $1,
    NOW(),
    NOW(),
    $2,
    $3,
    NULL
) RETURNING *;


-- name: GetRefreshToken :one
SELECT user_id, revoked_at, expires_at
FROM refresh_token
WHERE token = $1;


-- name: RevokeRefreshToken :exec
UPDATE refresh_token
SET
    updated_at = NOW(),
    revoked_at = NOW()
WHERE
    token = $1;
