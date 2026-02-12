-- name: CreateLedgerEntry :one
INSERT INTO ledger_entries (transaction_id, account_id, amount)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetEntriesByTransaction :many
SELECT * FROM ledger_entries
WHERE transaction_id = $1
ORDER BY created_at;

-- name: GetEntriesByAccount :many
SELECT * FROM ledger_entries
WHERE account_id = $1
ORDER BY created_at;
