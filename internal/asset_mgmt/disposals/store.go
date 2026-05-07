package disposals

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"IRIS-backend/internal/asset_mgmt/inventory"
	platformdb "IRIS-backend/internal/platform/db"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// --- assets_master / assets 参照 ---

func (s *Store) ResolveMasterID(ctx context.Context, managementNumber string) (uint64, error) {
	return s.resolveMasterID(ctx, s.db, managementNumber)
}

func (s *Store) ResolveMasterIDTx(ctx context.Context, tx *sql.Tx, managementNumber string) (uint64, error) {
	return s.resolveMasterID(ctx, tx, managementNumber)
}

func (s *Store) resolveMasterID(ctx context.Context, q platformdb.DBTX, managementNumber string) (uint64, error) {
	id, err := inventory.ResolveMasterID(ctx, q, managementNumber)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound("assets_master not found")
	}
	if err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) LockAssetRows(ctx context.Context, tx *sql.Tx, masterID uint64) ([]inventory.LockedAssetRow, error) {
	rows, err := inventory.LockAssetRowsByMasterID(ctx, tx, int64(masterID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound("asset row not found")
	}
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *Store) ApplyQuantityAdjustments(ctx context.Context, tx *sql.Tx, adjustments []inventory.QuantityAdjustment) error {
	if err := inventory.ApplyQuantityAdjustments(ctx, tx, adjustments); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound("asset row not found")
		}
		return err
	}
	return nil
}

func (s *Store) ReconcileAssetStatus(ctx context.Context, tx *sql.Tx, masterID uint64) error {
	return inventory.ReconcileAssetStatus(ctx, tx, int64(masterID))
}

// --- disposals ---

func (s *Store) InsertDisposal(ctx context.Context, tx *sql.Tx, m *Disposal) (uint64, error) {
	const q = `
	INSERT INTO disposals
	(disposal_ulid, management_number, quantity, disposed_at, reason, processed_by_id)
	VALUES
	(?, ?, ?, UTC_TIMESTAMP(), ?, ?)`
	res, err := tx.ExecContext(ctx, q,
		m.DisposalULID, m.ManagementNumber, m.Quantity,
		nullStrOrNil(m.Reason), nullStrOrNil(m.ProcessedByID),
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return uint64(id), nil
}

func (s *Store) GetByULID(ctx context.Context, ul string) (*Disposal, error) {
	const q = `
	SELECT disposal_id, disposal_ulid, management_number, quantity, disposed_at, reason, processed_by_id
	FROM disposals WHERE disposal_ulid = ?`
	var m Disposal
	if err := s.db.QueryRowContext(ctx, q, ul).Scan(
		&m.DisposalID, &m.DisposalULID, &m.ManagementNumber, &m.Quantity,
		&m.DisposedAt, &m.Reason, &m.ProcessedByID,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound("disposal not found")
		}
		return nil, err
	}
	return &m, nil
}

func (s *Store) List(ctx context.Context, f DisposalFilter, p Page) ([]Disposal, int64, error) {
	sb := strings.Builder{}
	sb.WriteString(`
	SELECT disposal_id, disposal_ulid, management_number, quantity, disposed_at, reason, processed_by_id
	FROM disposals WHERE 1=1`)

	args := []any{}
	if f.ManagementNumber != nil {
		sb.WriteString(` AND management_number = ?`)
		args = append(args, *f.ManagementNumber)
	}
	if f.ProcessedByID != nil {
		sb.WriteString(` AND processed_by_id = ?`)
		args = append(args, *f.ProcessedByID)
	}
	if f.From != nil {
		sb.WriteString(` AND disposed_at >= ?`)
		args = append(args, *f.From)
	}
	if f.To != nil {
		sb.WriteString(` AND disposed_at < ?`)
		args = append(args, *f.To)
	}

	order := "DESC"
	if strings.ToLower(p.Order) == "asc" {
		order = "ASC"
	}
	if p.Limit <= 0 {
		p.Limit = 50
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	sb.WriteString(fmt.Sprintf(` ORDER BY disposed_at %s LIMIT ? OFFSET ?`, order))
	args = append(args, p.Limit, p.Offset)

	rows, err := s.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []Disposal
	for rows.Next() {
		var m Disposal
		if err := rows.Scan(&m.DisposalID, &m.DisposalULID, &m.ManagementNumber, &m.Quantity, &m.DisposedAt, &m.Reason, &m.ProcessedByID); err != nil {
			return nil, 0, err
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// count
	cb := strings.Builder{}
	cb.WriteString(`SELECT COUNT(*) FROM disposals WHERE 1=1`)
	argsC := []any{}
	if f.ManagementNumber != nil {
		cb.WriteString(` AND management_number = ?`)
		argsC = append(argsC, *f.ManagementNumber)
	}
	if f.ProcessedByID != nil {
		cb.WriteString(` AND processed_by_id = ?`)
		argsC = append(argsC, *f.ProcessedByID)
	}
	if f.From != nil {
		cb.WriteString(` AND disposed_at >= ?`)
		argsC = append(argsC, *f.From)
	}
	if f.To != nil {
		cb.WriteString(` AND disposed_at < ?`)
		argsC = append(argsC, *f.To)
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, cb.String(), argsC...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func nullStrOrNil(ns sql.NullString) any {
	if ns.Valid {
		return ns.String
	}
	return nil
}
