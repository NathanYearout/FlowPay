-- name: CreateAccount :one
INSERT INTO accounts (owner, asset_type, account_type)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetAccount :one
SELECT * FROM accounts
WHERE id = $1;

-- name: ListAccountsByOwner :many
SELECT * FROM accounts
WHERE owner = $1
ORDER BY created_at;

-- name: GetBalance :one
SELECT COALESCE(SUM(amount), 0)::NUMERIC(28, 18) AS balance
FROM ledger_entries
WHERE account_id = $1;
