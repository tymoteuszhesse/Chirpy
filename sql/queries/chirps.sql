-- name: CreateChirp :one
INSERT INTO chirps (id, created_at, updated_at, body, user_id)
VALUES (
    gen_random_uuid(),
    now(),
    now(),
    $1,
    $2
)
RETURNING *;

-- name: RemoveChirps :exec
DELETE FROM chirps
RETURNING *;

-- name: GetChirps :many
SELECT * FROM chirps ORDER BY created_at;

-- name: GetChirpByID :one
SELECT * FROM chirps WHERE id = $1;