package db

import (
	"context"
	"database/sql"
)

type DBTX interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Txを開始して fn を実行。fn が nil を返せば COMMIT、エラーなら ROLLBACK。
func RunInTx(ctx context.Context, db *sql.DB, opts *sql.TxOptions, fn func(ctx context.Context, tx DBTX) error) error {
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}

	if err := fn(ctx, tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// 読み取り専用Tx
func ReadOnly(ctx context.Context, db *sql.DB, fn func(ctx context.Context, tx DBTX) error) error {
	return RunInTx(ctx, db, &sql.TxOptions{ReadOnly: true}, fn)
}
