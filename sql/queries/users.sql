-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
    gen_random_uuid(),
    now(),
    now(),
    $1,
    $2
)
RETURNING *;

-- name: RemoveUsers :exec
DELETE FROM users
RETURNING *;

-- name: GetUserID :one
SELECT id FROM users WHERE email = $1;

-- name: GetUserPassword :one
SELECT * FROM users WHERE email = $1;