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
	Create(ctx context.Context, a *Account, roleCode string) error
	Delete(ctx context.Context, id string) (int64, error)
	UpdateID(ctx context.Context, oldID, newID string) (int64, error)
	ListRoleCodesByID(ctx context.Context, id string) ([]string, error)
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

func (s *Store) Create(ctx context.Context, a *Account, roleCode string) (err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	const insertAccount = `
INSERT INTO auth_accounts (id, password_hash, role, is_disabled, created_at)
VALUES (?, ?, ?, 0, UTC_TIMESTAMP(6))
`
	if _, err = tx.ExecContext(ctx, insertAccount, a.ID, a.PasswordHash, a.Role); err != nil {
		return err
	}

	const selectRoleID = `
SELECT role_id
FROM auth_roles
WHERE role_code = ? AND is_disabled = 0
LIMIT 1
`
	var roleID uint64
	if err = tx.QueryRowContext(ctx, selectRoleID, roleCode).Scan(&roleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrInvalidRole
		}
		return err
	}

	const insertAccountRole = `
INSERT INTO auth_account_roles (account_id, role_id, assigned_at)
VALUES (?, ?, UTC_TIMESTAMP(6))
`
	if _, err = tx.ExecContext(ctx, insertAccountRole, a.ID, roleID); err != nil {
		return err
	}

	err = tx.Commit()
	return err
}

func (s *Store) Delete(ctx context.Context, id string) (_ int64, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `DELETE FROM auth_account_roles WHERE account_id = ?`, id); err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM auth_account_groups WHERE account_id = ?`, id); err != nil {
		return 0, err
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM auth_accounts WHERE id = ?`, id)
	if err != nil {
		return 0, err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Store) UpdateID(ctx context.Context, oldID, newID string) (_ int64, err error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	const cloneAccount = `
INSERT INTO auth_accounts (id, password_hash, role, is_disabled, created_at)
SELECT ?, password_hash, role, is_disabled, created_at
FROM auth_accounts
WHERE id = ?
`
	res, err := tx.ExecContext(ctx, cloneAccount, newID, oldID)
	if err != nil {
		return 0, err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	if n == 0 {
		err = tx.Commit()
		if err != nil {
			return 0, err
		}
		return 0, nil
	}

	if _, err = tx.ExecContext(ctx, `UPDATE auth_account_roles SET account_id = ? WHERE account_id = ?`, newID, oldID); err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE auth_account_groups SET account_id = ? WHERE account_id = ?`, newID, oldID); err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM auth_accounts WHERE id = ?`, oldID); err != nil {
		return 0, err
	}

	err = tx.Commit()
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *Store) ListRoleCodesByID(ctx context.Context, id string) ([]string, error) {
	const q = `
SELECT r.role_code
FROM auth_account_roles ar
JOIN auth_roles r ON ar.role_id = r.role_id
WHERE ar.account_id = ?
  AND r.is_disabled = 0
ORDER BY r.role_id
`

	rows, err := s.db.QueryContext(ctx, q, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roleCodes []string
	for rows.Next() {
		var roleCode string
		if err := rows.Scan(&roleCode); err != nil {
			return nil, err
		}
		roleCodes = append(roleCodes, roleCode)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return roleCodes, nil
}
