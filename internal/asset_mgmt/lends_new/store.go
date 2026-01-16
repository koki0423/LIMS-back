package lends_new

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	// "time"
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// 貸出INSERT
func (s *Store) InsertLend(ctx context.Context, lend *Lend) error {
	query := `
	INSERT INTO lends
	(lend_ulid, asset_master_id, management_number, quantity, borrower_id,
	due_on, lent_by_id, lent_at, note, returned)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	var managementNumber interface{}
	if lend.ManagementNumber.Valid {
		managementNumber = lend.ManagementNumber.String
	} else {
		managementNumber = nil
	}

	var dueOn interface{}
	if lend.DueOn.Valid {
		// DATE型へのマッピングは driver に依存するが time.Time をそのまま渡してOKなことが多い
		dueOn = lend.DueOn.Time
	} else {
		dueOn = nil
	}

	var lentByID interface{}
	if lend.LentByID.Valid {
		lentByID = lend.LentByID.String
	} else {
		lentByID = nil
	}

	var note interface{}
	if lend.Note.Valid {
		note = lend.Note.String
	} else {
		note = nil
	}

	res, err := s.db.ExecContext(ctx, query,
		lend.LendULID,
		lend.AssetMasterID,
		managementNumber,
		lend.Quantity,
		lend.BorrowerID,
		dueOn,
		lentByID,
		lend.LentAt,
		note,
		lend.Returned,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	lend.LendID = id
	return nil
}

// 貸出1件取得
func (s *Store) GetLendByID(ctx context.Context, lendID int64) (*Lend, error) {
	query := `
	SELECT lend_id, lend_ulid, asset_master_id, management_number, quantity,
		borrower_id, due_on, lent_by_id, lent_at, note, returned
	FROM lends
	WHERE lend_id = ?
	`
	row := s.db.QueryRowContext(ctx, query, lendID)
	var lend Lend
	var returnedInt int

	err := row.Scan(
		&lend.LendID,
		&lend.LendULID,
		&lend.AssetMasterID,
		&lend.ManagementNumber,
		&lend.Quantity,
		&lend.BorrowerID,
		&lend.DueOn,
		&lend.LentByID,
		&lend.LentAt,
		&lend.Note,
		&returnedInt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewNotFoundError("lend not found")
	}
	if err != nil {
		return nil, err
	}
	lend.Returned = returnedInt != 0
	return &lend, nil
}

// ULIDで貸出1件取得
func (s *Store) GetLendByULID(ctx context.Context, lendULID string) (*Lend, error) {
	query := `
	SELECT lend_id, lend_ulid, asset_master_id, management_number, quantity,
		borrower_id, due_on, lent_by_id, lent_at, note, returned
	FROM lends
	WHERE lend_ulid = ?
	LIMIT 1
	`
	row := s.db.QueryRowContext(ctx, query, lendULID)

	var lend Lend
	var returnedInt int
	err := row.Scan(
		&lend.LendID,
		&lend.LendULID,
		&lend.AssetMasterID,
		&lend.ManagementNumber,
		&lend.Quantity,
		&lend.BorrowerID,
		&lend.DueOn,
		&lend.LentByID,
		&lend.LentAt,
		&lend.Note,
		&returnedInt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewNotFoundError("lend not found")
	}
	if err != nil {
		return nil, err
	}
	lend.Returned = returnedInt != 0
	return &lend, nil
}

// 貸出リスト取得
func (s *Store) ListLends(ctx context.Context, filter LendFilter) ([]*Lend, error) {
	query := `
	SELECT lend_id, lend_ulid, asset_master_id, management_number, quantity,
		borrower_id, due_on, lent_by_id, lent_at, note, returned
	FROM lends
	WHERE 1 = 1
	`
	conds := []string{}
	args := []interface{}{}

	if filter.BorrowerID != "" {
		conds = append(conds, "borrower_id = ?")
		args = append(args, filter.BorrowerID)
	}
	if filter.AssetMasterID != nil {
		conds = append(conds, "asset_master_id = ?")
		args = append(args, *filter.AssetMasterID)
	}
	if filter.ManagementNumber != "" {
		conds = append(conds, "management_number = ?")
		args = append(args, filter.ManagementNumber)
	}
	if filter.Returned != nil {
		conds = append(conds, "returned = ?")
		var r int
		if *filter.Returned {
			r = 1
		} else {
			r = 0
		}
		args = append(args, r)
	}

	if len(conds) > 0 {
		query = query + " AND " + strings.Join(conds, " AND ")
	}
	query = query + " ORDER BY lent_at DESC"

	if filter.Limit > 0 {
		query = query + fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query = query + fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lends []*Lend
	for rows.Next() {
		var lend Lend
		var returnedInt int
		err := rows.Scan(
			&lend.LendID,
			&lend.LendULID,
			&lend.AssetMasterID,
			&lend.ManagementNumber,
			&lend.Quantity,
			&lend.BorrowerID,
			&lend.DueOn,
			&lend.LentByID,
			&lend.LentAt,
			&lend.Note,
			&returnedInt,
		)
		if err != nil {
			return nil, err
		}
		lend.Returned = returnedInt != 0
		lends = append(lends, &lend)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return lends, nil
}

// 返却 INSERT
func (s *Store) InsertReturn(ctx context.Context, ret *Return) error {
	query := `
	INSERT INTO returns
	(return_ulid, lend_id, quantity, processed_by_id, returned_at, note)
	VALUES (?, ?, ?, ?, ?, ?)
	`
	var processedByID interface{}
	if ret.ProcessedByID.Valid {
		processedByID = ret.ProcessedByID.String
	} else {
		processedByID = nil
	}

	var note interface{}
	if ret.Note.Valid {
		note = ret.Note.String
	} else {
		note = nil
	}

	res, err := s.db.ExecContext(ctx, query,
		ret.ReturnULID,
		ret.LendID,
		ret.Quantity,
		processedByID,
		ret.ReturnedAt,
		note,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	ret.ReturnID = id
	return nil
}

// 返却1件取得
func (s *Store) GetReturnByID(ctx context.Context, returnID int64) (*Return, error) {
	query := `
	SELECT return_id, return_ulid, lend_id, quantity, processed_by_id, returned_at, note
	FROM returns
	WHERE return_id = ?
	`
	row := s.db.QueryRowContext(ctx, query, returnID)
	var ret Return
	err := row.Scan(
		&ret.ReturnID,
		&ret.ReturnULID,
		&ret.LendID,
		&ret.Quantity,
		&ret.ProcessedByID,
		&ret.ReturnedAt,
		&ret.Note,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewNotFoundError("return not found")
	}
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

// ULIDで返却1件取得
func (s *Store) GetReturnByULID(ctx context.Context, returnULID string) (*Return, error) {
	query := `
	SELECT return_id, return_ulid, lend_id, quantity,
		processed_by_id, returned_at, note
	FROM returns
	WHERE return_ulid = ?
	LIMIT 1
`
	row := s.db.QueryRowContext(ctx, query, returnULID)

	var ret Return
	err := row.Scan(
		&ret.ReturnID,
		&ret.ReturnULID,
		&ret.LendID,
		&ret.Quantity,
		&ret.ProcessedByID,
		&ret.ReturnedAt,
		&ret.Note,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewNotFoundError("return not found")
	}
	if err != nil {
		return nil, err
	}
	return &ret, nil
}

// 返却リスト取得
func (s *Store) ListReturns(ctx context.Context, filter ReturnFilter) ([]*Return, error) {
	// lends テーブルと JOIN して borrower_id / asset_master_id で絞れるようにする
	query := `
	SELECT r.return_id, r.return_ulid, r.lend_id, r.quantity,
		r.processed_by_id, r.returned_at, r.note
	FROM returns r
	JOIN lends l ON r.lend_id = l.lend_id
	WHERE 1 = 1
	`
	conds := []string{}
	args := []interface{}{}

	if filter.BorrowerID != "" {
		conds = append(conds, "l.borrower_id = ?")
		args = append(args, filter.BorrowerID)
	}
	if filter.AssetMasterID != nil {
		conds = append(conds, "l.asset_master_id = ?")
		args = append(args, *filter.AssetMasterID)
	}
	if filter.LendID != nil {
		conds = append(conds, "r.lend_id = ?")
		args = append(args, *filter.LendID)
	}

	if len(conds) > 0 {
		query = query + " AND " + strings.Join(conds, " AND ")
	}
	query = query + " ORDER BY r.returned_at DESC"

	if filter.Limit > 0 {
		query = query + fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query = query + fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var returns []*Return
	for rows.Next() {
		var ret Return
		err := rows.Scan(
			&ret.ReturnID,
			&ret.ReturnULID,
			&ret.LendID,
			&ret.Quantity,
			&ret.ProcessedByID,
			&ret.ReturnedAt,
			&ret.Note,
		)
		if err != nil {
			return nil, err
		}
		returns = append(returns, &ret)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return returns, nil
}

// ある lend に対する返却済み数量合計
func (s *Store) GetTotalReturnedQuantity(ctx context.Context, lendID int64) (int, error) {
	query := `
	SELECT COALESCE(SUM(quantity), 0) FROM returns WHERE lend_id = ?
	`
	row := s.db.QueryRowContext(ctx, query, lendID)
	var total int
	err := row.Scan(&total)
	if err != nil {
		return 0, err
	}
	return total, nil
}

// lend の returned フラグ更新
func (s *Store) UpdateLendReturnedFlag(ctx context.Context, lendID int64, returned bool) error {
	query := `
	UPDATE lends
	SET returned = ?
	WHERE lend_id = ?
	`
	var r int
	if returned {
		r = 1
	} else {
		r = 0
	}
	_, err := s.db.ExecContext(ctx, query, r, lendID)
	return err
}

// トランザクション開始（必要ならサービスから使う）
func (s *Store) BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, opts)
}

// トランザクション内で使う total returned の取得
func GetTotalReturnedQuantityTx(ctx context.Context, tx *sql.Tx, lendID int64) (int, error) {
	query := `SELECT COALESCE(SUM(quantity), 0) FROM returns WHERE lend_id = ?`
	row := tx.QueryRowContext(ctx, query, lendID)
	var total int
	err := row.Scan(&total)
	if err != nil {
		return 0, err
	}
	return total, nil
}

// トランザクション内で returns INSERT
func InsertReturnTx(ctx context.Context, tx *sql.Tx, ret *Return) error {
	query := `
	INSERT INTO returns
	(return_ulid, lend_id, quantity, processed_by_id, returned_at, note)
	VALUES (?, ?, ?, ?, ?, ?)
	`
	var processedByID interface{}
	if ret.ProcessedByID.Valid {
		processedByID = ret.ProcessedByID.String
	} else {
		processedByID = nil
	}

	var note interface{}
	if ret.Note.Valid {
		note = ret.Note.String
	} else {
		note = nil
	}

	res, err := tx.ExecContext(ctx, query,
		ret.ReturnULID,
		ret.LendID,
		ret.Quantity,
		processedByID,
		ret.ReturnedAt,
		note,
	)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	ret.ReturnID = id
	return nil
}

// トランザクション内で lends.returned 更新
func UpdateLendReturnedFlagTx(ctx context.Context, tx *sql.Tx, lendID int64, returned bool) error {
	query := `UPDATE lends SET returned = ? WHERE lend_id = ?`
	var r int
	if returned {
		r = 1
	} else {
		r = 0
	}
	_, err := tx.ExecContext(ctx, query, r, lendID)
	return err
}

// トランザクション内で lend 取得
func GetLendByIDTx(ctx context.Context, tx *sql.Tx, lendID int64) (*Lend, error) {
	query := `
	SELECT lend_id, lend_ulid, asset_master_id, management_number, quantity,
		borrower_id, due_on, lent_by_id, lent_at, note, returned
	FROM lends
	WHERE lend_id = ?
	FOR UPDATE
	`
	row := tx.QueryRowContext(ctx, query, lendID)
	var lend Lend
	var returnedInt int
	err := row.Scan(
		&lend.LendID,
		&lend.LendULID,
		&lend.AssetMasterID,
		&lend.ManagementNumber,
		&lend.Quantity,
		&lend.BorrowerID,
		&lend.DueOn,
		&lend.LentByID,
		&lend.LentAt,
		&lend.Note,
		&returnedInt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, NewNotFoundError("lend not found")
	}
	if err != nil {
		return nil, err
	}
	lend.Returned = returnedInt != 0
	return &lend, nil
}

// 管理番号から asset_master_id を引く
func (s *Store) ResolveMasterID(ctx context.Context, managementNumber string) (int64, error) {
	if managementNumber == "" {
		return 0, NewInvalidArgumentError("management_number is required")
	}

	query := `
	SELECT asset_master_id
	FROM assets_master
	WHERE management_number = ?
	LIMIT 1
	`
	row := s.db.QueryRowContext(ctx, query, managementNumber)

	var assetMasterID int64
	err := row.Scan(&assetMasterID)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, NewNotFoundError("asset master not found for given management_number")
	}
	if err != nil {
		return 0, err
	}
	return assetMasterID, nil
}