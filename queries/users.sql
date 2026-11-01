
-- name: CreateUser :one
INSERT INTO users (
    username,
    nickname,
    email,
    password_hash,
    favorite_language
)
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5
)
RETURNING
    id,
    username,
    nickname,
    email,
    password_hash,
    favorite_language,
    last_login_at,
    created_at,
    updated_at;

-- name: GetUserByEmail :one
SELECT
    id,
    username,
    nickname,
    email,
    password_hash,
    favorite_language,
    last_login_at,
    created_at,
    updated_at
FROM users
WHERE email = $1
LIMIT 1;

-- name: GetUserByUsername :one
SELECT
    id,
    username,
    nickname,
    email,
    password_hash,
    favorite_language,
    last_login_at,
    created_at,
    updated_at
FROM users
WHERE username = $1
LIMIT 1;

-- name: GetUserByID :one
SELECT
    id,
    username,
    nickname,
    email,
    password_hash,
    favorite_language,
    last_login_at,
    created_at,
    updated_at
FROM users
WHERE id = $1
LIMIT 1;
