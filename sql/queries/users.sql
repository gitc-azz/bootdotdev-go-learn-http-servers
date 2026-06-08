-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    $1
)
RETURNING *;


-- name: EmptyUsers :exec
DELETE FROM users
WHERE created_at < NOW();
