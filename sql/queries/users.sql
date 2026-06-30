-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    $1,
    $2
)
RETURNING id, created_at, updated_at, email;


-- name: UserByEmail :one
SELECT *
FROM users
WHERE email = $1;


-- name: EmptyUsers :exec
DELETE FROM users
WHERE created_at < NOW();
