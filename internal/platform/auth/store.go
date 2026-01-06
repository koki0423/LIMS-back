package auth

import (
	"context"
	"database/sql"
	"errors"
)

type Account struct {
	ID           string
	PasswordHash string
	Role         string
	IsDisabled   bool
	CreatedAt    string
}

type AccountStore interface {
	GetByID(ctx context.Context, id string) (*Account, error)
	Create(ctx context.Context, a *Account) error
	Delete(ctx context.Context, id string) (int64, error)
	UpdateID(ctx context.Context, oldID, newID string) (int64, error)
}

type Store struct{ db *sql.DB }

// type sqlAccountStore struct {
// 	db *sql.DB
// }

func NewStore(db *sql.DB) AccountStore {
	return &Store{db: db}
}

func (s *Store) GetByID(ctx context.Context, id string) (*Account, error) {
	const q = `
SELECT id, password_hash, role, is_disabled, created_at
FROM auth_accounts
WHERE id = ?
LIMIT 1
`
	var a Account
	var isDisabledInt int
	err := s.db.QueryRowContext(ctx, q, id).Scan(
		&a.ID,
		&a.PasswordHash,
		&a.Role,
		&isDisabledInt,
		&a.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if isDisabledInt != 0 {
		a.IsDisabled = true
	}
	return &a, nil
}

func (s *Store) Create(ctx context.Context, a *Account) error {
	const q = `
INSERT INTO auth_accounts (id, password_hash, role, is_disabled, created_at)
VALUES (?, ?, ?, 0, NOW(6))
`
	_, err := s.db.ExecContext(ctx, q, a.ID, a.PasswordHash, a.Role)
	return err
}

func (s *Store) Delete(ctx context.Context, id string) (int64, error) {
	const q = `DELETE FROM auth_accounts WHERE id = ?`
	res, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Store) UpdateID(ctx context.Context, oldID, newID string) (int64, error) {
	// PK変更なので競合を避けたければトランザクションでもOK
	const q = `UPDATE auth_accounts SET id = ? WHERE id = ?`
	res, err := s.db.ExecContext(ctx, q, newID, oldID)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return n, nil
}
