-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password, is_chirpy_red)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    $1,
    $2,
    false
)
RETURNING id, created_at, updated_at, email, is_chirpy_red;


-- name: UserByEmail :one
SELECT *
FROM users
WHERE email = $1;


-- name: EmptyUsers :exec
DELETE FROM users
WHERE created_at < NOW();


-- name: UpdateUsers :exec
UPDATE users
SET
    email = $2,
    hashed_password = $3,
    updated_at = NOW()
WHERE
    id = $1;


-- name: UpgradeUserToRed :exec
UPDATE users
SET
    is_chirpy_red = 'true'
WHERE
    id = $1;
