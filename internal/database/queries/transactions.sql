-- name: CreateTransaction :one
INSERT INTO transactions (idempotency_key, type, status, metadata)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetTransactionByIdempotencyKey :one
SELECT * FROM transactions
WHERE idempotency_key = $1;

-- name: GetTransaction :one
SELECT * FROM transactions
WHERE id = $1;

-- name: UpdateTransactionStatus :exec
UPDATE transactions
SET status = $2, updated_at = NOW()
WHERE id = $1;
