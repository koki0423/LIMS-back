package lends_new

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// ResolveMasterID: management_number -> asset_master_id
func (s *Store) ResolveMasterID(ctx context.Context, managementNumber string) (uint64, error) {
	const q = `SELECT asset_master_id FROM assets_master WHERE management_number = ?`
	var id uint64
	if err := s.db.QueryRowContext(ctx, q, managementNumber).Scan(&id); err != nil {
		if err == sql.ErrNoRows {
			return 0, ErrNotFound("assets_master not found")
		}
		return 0, err
	}
	return id, nil
}

// GetManagementNumber resolves management_number from asset_master_id
func (s *Store) GetManagementNumber(ctx context.Context, masterID uint64) (string, error) {
	const q = `SELECT management_number FROM assets_master WHERE asset_master_id=?`
	var mng string
	if err := s.db.QueryRowContext(ctx, q, masterID).Scan(&mng); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return mng, nil
}

// lock inventory row (assets) by master id
func (s *Store) lockAssetRow(ctx context.Context, tx *sql.Tx, masterID uint64) (assetID uint64, quantity uint, err error) {
	const q = `SELECT asset_id, quantity FROM assets WHERE asset_master_id = ? LIMIT 1 FOR UPDATE`
	row := tx.QueryRowContext(ctx, q, masterID)
	if err = row.Scan(&assetID, &quantity); err != nil {
		if err == sql.ErrNoRows {
			return 0, 0, ErrNotFound("asset row not found")
		}
		return 0, 0, err
	}
	return assetID, quantity, nil
}

func (s *Store) updateAssetQuantity(ctx context.Context, tx *sql.Tx, assetID uint64, delta int) error {
	const q = `UPDATE assets SET quantity = quantity + ? WHERE asset_id = ?`
	res, err := tx.ExecContext(ctx, q, delta, assetID)
	if err != nil {
		return err
	}
	aff, _ := res.RowsAffected()
	if aff != 1 {
		return ErrInternal("failed to update assets.quantity")
	}
	return nil
}

// updateAssetsStatus updates status_id if different
func (s *Store) updateAssetsStatus(ctx context.Context, tx *sql.Tx, masterID uint64, statusID int) error {
	const q = `
		UPDATE assets
		SET status_id = ?
		WHERE asset_master_id = ?
		AND status_id <> ?`
	// エラーは返すが、RowsAffected=0は許容する（ステータスが変わらない場合があるため）
	res, err := tx.ExecContext(ctx, q, statusID, masterID, statusID)
	if err != nil {
		return err
	}
	_, _ = res.RowsAffected()
	return nil
}

func (s *Store) updateAssetOnLend(ctx context.Context, tx *sql.Tx, borrowerID string, assetId uint64) error {
	const q = `
		UPDATE assets
		SET location = ?, last_checked_at = ?
		WHERE asset_id = ?`
	res, err := tx.ExecContext(ctx, q, borrowerID, time.Now(), assetId)
	if err != nil {
		return err
	}
	aff, _ := res.RowsAffected()
	if aff != 1 {
		return ErrInternal("failed to update assets.location")
	}
	return nil
}

// ---- Transactional Methods ----

// ExecCreateLend handles the full transaction flow for creating a lend
func (s *Store) ExecCreateLend(ctx context.Context, m *Lend) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// 1. Lock asset row
	assetID, qty, err := s.lockAssetRow(ctx, tx, m.AssetMasterID)
	if err != nil {
		return err
	}

	// 2. Stock check
	if int(qty)-int(m.Quantity) < 0 {
		err = ErrConflict("insufficient stock")
		return err
	}
	// 3. Decrement stock
	if err = s.updateAssetQuantity(ctx, tx, assetID, -int(m.Quantity)); err != nil {
		return err
	}

	// 4. Insert lend
	const q = `
	INSERT INTO lends
	(lend_ulid, asset_master_id, quantity, borrower_id, due_on, lent_by_id, lent_at, note)
	VALUES
	(?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?)`

	res, err := tx.ExecContext(ctx, q,
		m.LendULID,
		m.AssetMasterID,
		// m.ManagementNumber,
		m.Quantity,
		m.BorrowerID,
		m.DueOn,
		nullStrOrNil(m.LentByID),
		nullStrOrNil(m.Note),
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	m.LendID = uint64(id)

	// 5. Update Asset Status (4: 貸出中)
	if err := s.updateAssetsStatus(ctx, tx, m.AssetMasterID, 4); err != nil {
		log.Printf("failed to update assets.status: %v", err)
	}

	// 6. Update location
	if err := s.updateAssetOnLend(ctx, tx, m.BorrowerID, assetID); err != nil {
		log.Printf("failed to update assets.location: %v", err)
	}

	return tx.Commit()
}

// ExecCreateReturn handles the full transaction flow for creating a return
func (s *Store) ExecCreateReturn(ctx context.Context, m *Return) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	// 1. Get Lend details & validation
	// LendIDはService層でULIDから解決して渡される想定だが、
	// トランザクション内での整合性のため再取得・ロックも検討余地あり。
	// ここでは単純なSELECTでLend情報を取得する。
	lendQ := `SELECT lend_id, lend_ulid, asset_master_id, quantity FROM lends WHERE lend_id = ?`
	var l Lend
	if err = tx.QueryRowContext(ctx, lendQ, m.LendID).Scan(&l.LendID, &l.LendULID, &l.AssetMasterID, &l.Quantity); err != nil {
		return err
	}

	// 2. Check Over Return
	sumQ := `SELECT COALESCE(SUM(quantity),0) FROM returns WHERE lend_id = ?`
	var sum uint
	if err = tx.QueryRowContext(ctx, sumQ, m.LendID).Scan(&sum); err != nil {
		return err
	}
	outstanding := uint(0)
	if l.Quantity > sum {
		outstanding = l.Quantity - sum
	}
	if m.Quantity > outstanding {
		err = ErrConflict("over return")
		return err
	}

	// 3. Lock asset row & Add stock
	assetID, _, err := s.lockAssetRow(ctx, tx, l.AssetMasterID)
	if err != nil {
		return err
	}
	if err = s.updateAssetQuantity(ctx, tx, assetID, int(m.Quantity)); err != nil {
		return err
	}

	// 4. Insert return
	const q = `
	INSERT INTO returns
	(return_ulid, lend_id, quantity, processed_by_id, returned_at, note)
	VALUES
	(?, ?, ?, ?, CURRENT_TIMESTAMP, ?)`
	res, err := tx.ExecContext(ctx, q,
		m.ReturnULID, m.LendID, m.Quantity, nullStrOrNil(m.ProcessedByID), nullStrOrNil(m.Note),
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	m.ReturnID = uint64(id)

	// 5. Update Asset Status (1: 利用可能)
	if err = s.updateAssetsStatus(ctx, tx, l.AssetMasterID, 1); err != nil {
		log.Printf("failed to update assets.status: %v", err)
	}

	// 6. Update Lend Returned Status (returned=1)
	// 全部返却されたかどうかの厳密なチェックは要件によるが、
	// 既存ロジックに倣い、返却アクションがあれば一旦 returned=true (1) にする
	// ※部分返却の仕様がある場合は注意
	const updateLendQ = `UPDATE lends SET returned = ? WHERE lend_id = ?`
	if _, err = tx.ExecContext(ctx, updateLendQ, true, l.LendID); err != nil {
		log.Printf("failed to update lends.returned_status: %v", err)
	}

	// 7. Update Location (Clear)
	if err = s.updateAssetOnLend(ctx, tx, "", assetID); err != nil {
		log.Printf("failed to update assets.location: %v", err)
	}

	return tx.Commit()
}

// ---- Queries ----

// GetActiveLendByManagementNumber: QRスキャン用
// 返却未済(returned=0)の貸出データを1件取得する
func (s *Store) GetActiveLendByManagementNumber(ctx context.Context, managementNumber string) (*Lend, error) {
	const q = `
	SELECT 
		l.lend_id, l.lend_ulid, l.asset_master_id, l.quantity, l.borrower_id, 
		l.due_on, l.lent_by_id, l.lent_at, l.note, l.returned
	FROM lends l
	JOIN assets_master m ON m.asset_master_id = l.asset_master_id -- JOIN追加
	WHERE m.management_number = ?  -- マスタ側のカラムで検索
	  AND l.returned = 0
	LIMIT 1`

	var m Lend
	err := s.db.QueryRowContext(ctx, q, managementNumber).Scan(
		&m.LendID, &m.LendULID, &m.AssetMasterID, &m.Quantity, &m.BorrowerID,
		&m.DueOn, &m.LentByID, &m.LentAt, &m.Note, &m.Returned,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound("active lend not found")
		}
		return nil, err
	}
	return &m, nil
}

func (s *Store) GetLendByULID(ctx context.Context, ulid string) (*Lend, error) {
	const q = `
	SELECT lend_id, lend_ulid, asset_master_id, quantity, borrower_id, due_on, lent_by_id, lent_at, note, returned
	FROM lends WHERE lend_ulid = ?`
	var m Lend
	err := s.db.QueryRowContext(ctx, q, ulid).Scan(
		&m.LendID, &m.LendULID, &m.AssetMasterID, &m.Quantity, &m.BorrowerID,
		&m.DueOn, &m.LentByID, &m.LentAt, &m.Note, &m.Returned,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrNotFound("lend not found")
		}
		return nil, err
	}
	return &m, nil
}

func (s *Store) SumReturned(ctx context.Context, lendID uint64) (uint, error) {
	const q = `SELECT COALESCE(SUM(quantity),0) FROM returns WHERE lend_id = ?`
	var sum uint
	if err := s.db.QueryRowContext(ctx, q, lendID).Scan(&sum); err != nil {
		return 0, err
	}
	return sum, nil
}

type lendRow struct {
	Lend
	ManagementNumber string
	ReturnedSum      uint
}

func (s *Store) ListLends(ctx context.Context, f LendFilter, p Page) ([]lendRow, int64, error) {
	sb := strings.Builder{}
	sb.WriteString(`
	SELECT
	l.lend_id, l.lend_ulid, l.asset_master_id, l.quantity, l.borrower_id, l.due_on, l.lent_by_id, l.lent_at, l.note, l.returned,
	m.management_number,
	COALESCE(r.sum_qty,0) AS returned_sum
	FROM lends l
	JOIN assets_master m ON m.asset_master_id = l.asset_master_id
	LEFT JOIN (
	SELECT lend_id, SUM(quantity) AS sum_qty FROM returns GROUP BY lend_id
	) r ON r.lend_id = l.lend_id
	WHERE 1=1
`)

	args := []any{}
	if f.ManagementNumber != nil {
		sb.WriteString(` AND m.management_number = ?`)
		args = append(args, *f.ManagementNumber)
	}
	if f.BorrowerID != nil {
		sb.WriteString(` AND l.borrower_id = ?`)
		args = append(args, *f.BorrowerID)
	}
	if f.From != nil {
		sb.WriteString(` AND l.lent_at >= ?`)
		args = append(args, *f.From)
	}
	if f.To != nil {
		sb.WriteString(` AND l.lent_at < ?`)
		args = append(args, *f.To)
	}
	if f.OnlyOutstanding {
		sb.WriteString(` AND COALESCE(r.sum_qty,0) < l.quantity`)
	}
	if f.Returned != nil {
		sb.WriteString(` AND l.returned = ?`)
		args = append(args, *f.Returned)
	}
	order := "DESC"
	if strings.ToLower(p.Order) == "asc" {
		order = "ASC"
	}
	sb.WriteString(fmt.Sprintf(` ORDER BY l.lent_at %s`, order))
	if p.Limit <= 0 {
		p.Limit = 50
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	sb.WriteString(` LIMIT ? OFFSET ?`)
	args = append(args, p.Limit, p.Offset)

	rows, err := s.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []lendRow
	for rows.Next() {
		var r lendRow
		if err := rows.Scan(
			&r.Lend.LendID, &r.Lend.LendULID, &r.Lend.AssetMasterID, &r.Lend.Quantity, &r.Lend.BorrowerID,
			&r.Lend.DueOn, &r.Lend.LentByID, &r.Lend.LentAt, &r.Lend.Note, &r.Lend.Returned,
			&r.ManagementNumber, &r.ReturnedSum,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Total count query
	cb := strings.Builder{}
	cb.WriteString(`SELECT COUNT(*) FROM lends l JOIN assets_master m ON m.asset_master_id=l.asset_master_id LEFT JOIN (SELECT lend_id, SUM(quantity) sum_qty FROM returns GROUP BY lend_id) r ON r.lend_id=l.lend_id WHERE 1=1`)
	// Reuse args from WHERE clause (need to rebuild or separate logic, simplified here)
	// For production, extracting filter logic is recommended.
	// 簡易実装のためargsを再利用できる構造を想定するか、フィルタロジックを関数化すべきですが、
	// ここでは実装を省略せず、同じ条件を追加します。
	argsCnt := []any{}
	if f.ManagementNumber != nil {
		cb.WriteString(` AND m.management_number = ?`)
		argsCnt = append(argsCnt, *f.ManagementNumber)
	}
	if f.BorrowerID != nil {
		cb.WriteString(` AND l.borrower_id = ?`)
		argsCnt = append(argsCnt, *f.BorrowerID)
	}
	if f.From != nil {
		cb.WriteString(` AND l.lent_at >= ?`)
		argsCnt = append(argsCnt, *f.From)
	}
	if f.To != nil {
		cb.WriteString(` AND l.lent_at < ?`)
		argsCnt = append(argsCnt, *f.To)
	}
	if f.OnlyOutstanding {
		cb.WriteString(` AND COALESCE(r.sum_qty,0) < l.quantity`)
	}
	if f.Returned != nil {
		cb.WriteString(` AND l.returned = ?`)
		argsCnt = append(argsCnt, *f.Returned)
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, cb.String(), argsCnt...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return out, total, nil
}

func (s *Store) ListReturnsByLend(ctx context.Context, lendID uint64, p Page) ([]Return, int64, error) {
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
	q := fmt.Sprintf(`
	SELECT return_id, return_ulid, lend_id, quantity, processed_by_id, returned_at, note
	FROM returns WHERE lend_id = ? ORDER BY returned_at %s LIMIT ? OFFSET ?`, order)

	rows, err := s.db.QueryContext(ctx, q, lendID, p.Limit, p.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []Return
	for rows.Next() {
		var m Return
		if err := rows.Scan(&m.ReturnID, &m.ReturnULID, &m.LendID, &m.Quantity, &m.ProcessedByID, &m.ReturnedAt, &m.Note); err != nil {
			return nil, 0, err
		}
		items = append(items, m)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM returns WHERE lend_id = ?`, lendID).Scan(&total); err != nil {
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