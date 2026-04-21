package database

import (
	"context"
	"database/sql"
	"fmt"
)

type TxFunc func(ctx context.Context, tx *sql.Tx) error

func (m *Manager) WithTx(ctx context.Context, opts *sql.TxOptions, fn TxFunc) error {
	db, _, err := m.DB(ctx)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}

	if err := fn(ctx, tx); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("transaction failed: %w; rollback failed: %v", err, rollbackErr)
		}
		return err
	}

	return tx.Commit()
}
