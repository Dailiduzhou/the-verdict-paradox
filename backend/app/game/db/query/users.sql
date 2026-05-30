-- name: CreateUser :one
INSERT INTO users (name, password_hash)
VALUES ($1, $2)
RETURNING *;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByName :one
SELECT * FROM users WHERE name = $1;

-- name: UpdateUser :one
UPDATE users
SET name = $2, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users SET password_hash = $2, updated_at = CURRENT_TIMESTAMP WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = $1;
