package postgres

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"github.com/greenlab/device-registry/internal/application"
)

// TxRunner implements application.TxRunner using a sqlx.DB connection pool.
type TxRunner struct {
	db *sqlx.DB
}

// NewTxRunner constructs a TxRunner backed by the given connection pool.
func NewTxRunner(db *sqlx.DB) *TxRunner {
	return &TxRunner{db: db}
}

// RunInTx begins a transaction, calls fn with transaction-scoped repos, and commits
// on success or rolls back on any error returned by fn.
func (r *TxRunner) RunInTx(ctx context.Context, fn func(ctx context.Context, tx application.TxRepos) error) (err error) {
	txn, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("TxRunner.Begin: %w", err)
	}
	defer func() {
		if err != nil {
			_ = txn.Rollback()
		}
	}()

	repos := application.TxRepos{
		Devices:  newTxDeviceRepo(txn),
		Channels: newTxChannelRepo(txn),
		Fields:   newTxFieldRepo(txn),
	}

	if err = fn(ctx, repos); err != nil {
		return err
	}
	if err = txn.Commit(); err != nil {
		return fmt.Errorf("TxRunner.Commit: %w", err)
	}
	return nil
}
