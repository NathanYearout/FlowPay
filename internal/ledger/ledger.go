package ledger

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "flowpay/internal/database/db"
)

var (
	ErrEntriesUnbalanced = errors.New("ledger entries must sum to zero")
	ErrMixedAssets       = errors.New("all entries must reference accounts of the same asset type")
	ErrDuplicateRequest  = errors.New("duplicate idempotency key")
	ErrAccountNotFound   = errors.New("account not found")
)

type EntryInput struct {
	AccountID string `json:"account_id"`
	Amount    string `json:"amount"`
}

type Ledger struct {
	pool    *pgxpool.Pool
	queries *db.Queries
}

func New(pool *pgxpool.Pool) *Ledger {
	return &Ledger{
		pool:    pool,
		queries: db.New(pool),
	}
}

func (l *Ledger) CreateAccount(ctx context.Context, owner, assetType, accountType string) (db.Account, error) {
	return l.queries.CreateAccount(ctx, db.CreateAccountParams{
		Owner:       owner,
		AssetType:   assetType,
		AccountType: accountType,
	})
}

func (l *Ledger) GetAccount(ctx context.Context, id pgtype.UUID) (db.Account, error) {
	return l.queries.GetAccount(ctx, id)
}

func (l *Ledger) GetBalance(ctx context.Context, accountID pgtype.UUID) (pgtype.Numeric, error) {
	return l.queries.GetBalance(ctx, accountID)
}

func (l *Ledger) CreateTransaction(
	ctx context.Context,
	idempotencyKey string,
	txnType string,
	metadata []byte,
	entries []EntryInput,
) (db.Transaction, []db.LedgerEntry, error) {

	// Idempotency check
	existing, err := l.queries.GetTransactionByIdempotencyKey(ctx, idempotencyKey)
	if err == nil {
		existingEntries, _ := l.queries.GetEntriesByTransaction(ctx, existing.ID)
		return existing, existingEntries, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return db.Transaction{}, nil, fmt.Errorf("idempotency check failed: %w", err)
	}

	// Parse amounts and validate they sum to zero
	parsedAmounts := make([]pgtype.Numeric, len(entries))
	sum := new(big.Float)
	for i, e := range entries {
		n := pgtype.Numeric{}
		if err := n.Scan(e.Amount); err != nil {
			return db.Transaction{}, nil, fmt.Errorf("invalid amount %q: %w", e.Amount, err)
		}
		parsedAmounts[i] = n

		f, _, err := new(big.Float).Parse(e.Amount, 10)
		if err != nil {
			return db.Transaction{}, nil, fmt.Errorf("invalid amount %q: %w", e.Amount, err)
		}
		sum.Add(sum, f)
	}

	if sum.Cmp(new(big.Float)) != 0 {
		return db.Transaction{}, nil, ErrEntriesUnbalanced
	}

	// Parse account IDs
	accountIDs := make([]pgtype.UUID, len(entries))
	for i, e := range entries {
		uid := pgtype.UUID{}
		if err := uid.Scan(e.AccountID); err != nil {
			return db.Transaction{}, nil, fmt.Errorf("invalid account_id %q: %w", e.AccountID, err)
		}
		accountIDs[i] = uid
	}

	// Begin Postgres transaction
	pgTx, err := l.pool.Begin(ctx)
	if err != nil {
		return db.Transaction{}, nil, fmt.Errorf("begin tx: %w", err)
	}
	defer pgTx.Rollback(ctx)

	qtx := l.queries.WithTx(pgTx)

	// Validate all accounts exist and share the same asset type
	var assetType string
	for i, uid := range accountIDs {
		acct, err := qtx.GetAccount(ctx, uid)
		if err != nil {
			return db.Transaction{}, nil, fmt.Errorf("account %s: %w", entries[i].AccountID, ErrAccountNotFound)
		}
		if i == 0 {
			assetType = acct.AssetType
		} else if acct.AssetType != assetType {
			return db.Transaction{}, nil, ErrMixedAssets
		}
	}

	// Insert the transaction record
	txn, err := qtx.CreateTransaction(ctx, db.CreateTransactionParams{
		IdempotencyKey: idempotencyKey,
		Type:           txnType,
		Status:         "completed",
		Metadata:       metadata,
	})
	if err != nil {
		return db.Transaction{}, nil, fmt.Errorf("create transaction: %w", err)
	}

	// Insert ledger entries
	ledgerEntries := make([]db.LedgerEntry, len(entries))
	for i := range entries {
		entry, err := qtx.CreateLedgerEntry(ctx, db.CreateLedgerEntryParams{
			TransactionID: txn.ID,
			AccountID:     accountIDs[i],
			Amount:        parsedAmounts[i],
		})
		if err != nil {
			return db.Transaction{}, nil, fmt.Errorf("create ledger entry: %w", err)
		}
		ledgerEntries[i] = entry
	}

	// Commit !
	if err := pgTx.Commit(ctx); err != nil {
		return db.Transaction{}, nil, fmt.Errorf("commit: %w", err)
	}

	return txn, ledgerEntries, nil
}
