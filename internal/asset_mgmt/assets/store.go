package assets

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// ===== master =====

// 1) 仮INSERT（created_at は DB時刻）
func (s *Store) InsertMasterTmp(ctx context.Context, in CreateAssetMasterRequest, tmpMng string) (uint64, error) {
	const q = `
	INSERT INTO assets_master
	(management_number, name, management_category_id, genre_id, manufacturer, model, created_at)
	VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`
	res, err := s.db.ExecContext(ctx, q, tmpMng, in.Name, in.ManagementCategoryID, in.GenreID, in.Manufacturer, in.Model)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return uint64(id), nil
}

// 2) 確定番号に置換（DBの created_at / genres.genre_code 利用）
func (s *Store) UpdateMngToFinal(ctx context.Context, id uint64, tmpMng string, pad int) error {
	q := fmt.Sprintf(`
	UPDATE assets_master m
	JOIN asset_genres g ON g.genre_id = m.genre_id
	SET m.management_number = CONCAT(g.genre_code, '-', DATE_FORMAT(m.created_at, '%%Y%%m%%d'), '-', LPAD(m.asset_master_id, %d, '0'))
	WHERE m.asset_master_id = ? AND m.management_number = ?`, pad)

	res, err := s.db.ExecContext(ctx, q, id, tmpMng)
	if err != nil {
		return err
	}
	if aff, _ := res.RowsAffected(); aff != 1 {
		return ErrConflict("no row updated")
	}
	return nil
}

// 3) 取得
func (s *Store) GetMasterByID(ctx context.Context, id uint64) (*AssetMasterResponse, error) {
	const q = `
	SELECT asset_master_id, management_number, name, management_category_id, genre_id, manufacturer, model, created_at
	FROM assets_master WHERE asset_master_id = ?`
	var out AssetMasterResponse
	if err := s.db.QueryRowContext(ctx, q, id).Scan(
		&out.AssetMasterID, &out.ManagementNumber, &out.Name, &out.ManagementCategoryID,
		&out.GenreID, &out.Manufacturer, &out.Model, &out.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &out, nil
}

func (s *Store) GetMasterByMng(ctx context.Context, mng string) (*AssetMasterResponse, error) {
	const q = `
	SELECT asset_master_id, management_number, name, management_category_id, genre_id, manufacturer, model, created_at
	FROM assets_master WHERE management_number = ?`
	var r AssetMasterResponse
	if err := s.db.QueryRowContext(ctx, q, mng).Scan(
		&r.AssetMasterID, &r.ManagementNumber, &r.Name, &r.ManagementCategoryID, &r.GenreID,
		&r.Manufacturer, &r.Model, &r.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, err
	}
	return &r, nil
}

func (s *Store) GetMasterIDByMng(ctx context.Context, mng string) (uint64, error) {
	const q = `SELECT asset_master_id FROM assets_master WHERE management_number = ?`
	var id uint64
	if err := s.db.QueryRowContext(ctx, q, mng).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func (s *Store) UpdateMasterByMng(ctx context.Context, mng string, in UpdateAssetMasterRequest) (*AssetMasterResponse, error) {
	// 動的アップデート
	sets := []string{}
	args := []any{}
	if in.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *in.Name)
	}
	if in.ManagementCategoryID != nil {
		sets = append(sets, "management_category_id = ?")
		args = append(args, *in.ManagementCategoryID)
	}
	if in.GenreID != nil {
		sets = append(sets, "genre_id = ?")
		args = append(args, *in.GenreID)
	}
	if in.Manufacturer != nil {
		sets = append(sets, "manufacturer = ?")
		args = append(args, *in.Manufacturer)
	}
	if in.Model != nil {
		sets = append(sets, "model = ?")
		args = append(args, *in.Model)
	}
	if len(sets) == 0 {
		// 変更なしでも現行値を返す
		return s.GetMasterByMng(ctx, mng)
	}
	args = append(args, mng)
	q := fmt.Sprintf(`UPDATE assets_master SET %s WHERE management_number = ?`, strings.Join(sets, ", "))

	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	if aff, _ := res.RowsAffected(); aff == 0 {
		return nil, sql.ErrNoRows
	}
	return s.GetMasterByMng(ctx, mng)
}

func (s *Store) ListMasters(ctx context.Context, p Page, q AssetSearchQuery) ([]AssetMasterResponse, int64, error) {
	// --- 1. 動的クエリ構築のための準備 ---
	var sb strings.Builder
	args := []any{}

	// --- 2. SELECT句の構築 ---
	sb.WriteString(`
	SELECT asset_master_id, management_number, name, management_category_id, genre_id, manufacturer, model, created_at
	FROM assets_master
	WHERE 1=1
	`)

	// --- 3. WHERE句（フィルタ条件）の動的な追加 ---
	if q.GenreID != nil && *q.GenreID != 0 {
		sb.WriteString(" AND genre_id = ?")
		args = append(args, *q.GenreID)
	}
	// 他のフィルタ条件もここに追加できます
	// if q.Name != nil { ... }

	// --- 4. ORDER BY句の安全な構築 ---
	order := "DESC"
	if strings.ToLower(p.Order) == "asc" {
		order = "ASC"
	}
	// Sprintfを使わず、変数を直接埋め込むことでSQLインジェクションを防ぐ
	sb.WriteString(" ORDER BY created_at " + order)

	// --- 5. LIMIT / OFFSET句の構築 ---
	if p.Limit <= 0 {
		p.Limit = 50
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	sb.WriteString(" LIMIT ? OFFSET ?")
	args = append(args, p.Limit, p.Offset)

	// --- 6. クエリの実行 ---
	query := sb.String()
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	// --- 7. 結果のスキャン (変更なし) ---
	list := []AssetMasterResponse{}
	for rows.Next() {
		var r AssetMasterResponse
		if err := rows.Scan(
			&r.AssetMasterID, &r.ManagementNumber, &r.Name, &r.ManagementCategoryID, &r.GenreID,
			&r.Manufacturer, &r.Model, &r.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		list = append(list, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// --- 8. 総件数取得クエリの構築（フィルタを反映） ---
	var cb strings.Builder
	countArgs := []any{}
	cb.WriteString("SELECT COUNT(*) FROM assets_master WHERE 1=1")

	if q.GenreID != nil && *q.GenreID != 0 {
		cb.WriteString(" AND genre_id = ?")
		countArgs = append(countArgs, *q.GenreID)
	}
	// 他のフィルタ条件もここに追加...

	var total int64
	if err := s.db.QueryRowContext(ctx, cb.String(), countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return list, total, nil
}

// ===== assets =====

type assetRow struct {
	AssetID          uint64
	AssetMasterID    uint64
	ManagementNumber string
	Serial           sqlNullString
	Quantity         uint
	PurchasedAt      time.Time
	StatusID         uint
	Owner            string
	DefaultLocation  string
	Location         sqlNullString
	LastCheckedAt    sqlNullTime
	LastCheckedBy    sqlNullString
	Notes            sqlNullString
}

type sqlNullString struct{ sql.NullString }
type sqlNullTime struct{ sql.NullTime }


func (s *Store) CreateAssetTx(
    ctx context.Context,
    in CreateAssetRequest,
    masterID uint64,
) (assetID uint64, managementNumber string, err error) {

    tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
    if err != nil {
        return 0, "", err
    }
    // 失敗時は必ずRollback
    defer func() {
        if err != nil {
            _ = tx.Rollback()
        }
    }()

    const qIns = `
        INSERT INTO assets
          (asset_master_id, serial, quantity, purchased_at, status_id, owner, default_location,
           location, last_checked_at, last_checked_by, notes)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, UTC_TIMESTAMP(), ?, ?)`

    res, err := tx.ExecContext(ctx, qIns,
        masterID,
        in.Serial,
        in.Quantity,
        in.PurchasedAt,
        in.StatusID,
        in.Owner,
        in.DefaultLocation,
        in.Location,
        in.LastCheckedBy,
        in.Notes,
    )
    if err != nil {
        return 0, "", err
    }

    id64, err := res.LastInsertId()
    if err != nil {
        return 0, "", err
    }
    assetID = uint64(id64)

    const qMgmt = `SELECT management_number FROM assets_master WHERE asset_master_id = ?`
    if err = tx.QueryRowContext(ctx, qMgmt, masterID).Scan(&managementNumber); err != nil {
        log.Printf("Failed to resolve management_number for masterID=%d: %v", masterID, err)
        return 0, "", err
    }

    if err = tx.Commit(); err != nil {
        return 0, "", err
    }
    return assetID, managementNumber, nil
}

func (s *Store) GetAssetByID(ctx context.Context, id uint64) (*AssetResponse, error) {
	const q = `
	SELECT a.asset_id, a.asset_master_id, m.management_number, a.serial, a.quantity, a.purchased_at, a.status_id,
		a.owner, a.default_location, a.location, a.last_checked_at, a.last_checked_by, a.notes
	FROM assets a
	JOIN assets_master m ON m.asset_master_id = a.asset_master_id
	WHERE a.asset_master_id = ?`
	var r AssetResponse
	var serial, loc, lcb, notes sql.NullString
	var lct sql.NullTime
	if err := s.db.QueryRowContext(ctx, q, id).Scan(
		&r.AssetID, &r.AssetMasterID, &r.ManagementNumber, &serial, &r.Quantity, &r.PurchasedAt, &r.StatusID,
		&r.Owner, &r.DefaultLocation, &loc, &lct, &lcb, &notes,
	); err != nil {
		return nil, err
	}
	if serial.Valid {
		v := serial.String
		r.Serial = &v
	}
	if loc.Valid {
		v := loc.String
		r.Location = &v
	}
	if lct.Valid {
		v := lct.Time
		r.LastCheckedAt = &v
	}
	if lcb.Valid {
		v := lcb.String
		r.LastCheckedBy = &v
	}
	if notes.Valid {
		v := notes.String
		r.Notes = &v
	}
	return &r, nil
}

func (s *Store) UpdateAssetByID(ctx context.Context, id uint64, in UpdateAssetRequest) (*AssetResponse, error) {
	sets := []string{}
	args := []any{}
	if in.Serial != nil {
		sets = append(sets, "serial = ?")
		args = append(args, *in.Serial)
	}
	if in.Quantity != nil {
		sets = append(sets, "quantity = ?")
		args = append(args, *in.Quantity)
	}
	if in.PurchasedAt != nil {
		sets = append(sets, "purchased_at = ?")
		args = append(args, *in.PurchasedAt)
	}
	if in.StatusID != nil {
		sets = append(sets, "status_id = ?")
		args = append(args, *in.StatusID)
	}
	if in.Owner != nil {
		sets = append(sets, "owner = ?")
		args = append(args, *in.Owner)
	}
	if in.DefaultLocation != nil {
		sets = append(sets, "default_location = ?")
		args = append(args, *in.DefaultLocation)
	}
	if in.Location != nil {
		sets = append(sets, "location = ?")
		args = append(args, *in.Location)
	}
	if in.LastCheckedAt != nil {
		sets = append(sets, "last_checked_at = ?")
		args = append(args, *in.LastCheckedAt)
	}
	if in.LastCheckedBy != nil {
		sets = append(sets, "last_checked_by = ?")
		args = append(args, *in.LastCheckedBy)
	}
	if in.Notes != nil {
		sets = append(sets, "notes = ?")
		args = append(args, *in.Notes)
	}

	if len(sets) == 0 {
		return s.GetAssetByID(ctx, id)
	}

	args = append(args, id)
	q := fmt.Sprintf(`UPDATE assets SET %s WHERE asset_id = ?`, strings.Join(sets, ", "))
	res, err := s.db.ExecContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	if aff, _ := res.RowsAffected(); aff == 0 {
		return nil, sql.ErrNoRows
	}
	return s.GetAssetByID(ctx, id)
}

func (s *Store) ListAssets(ctx context.Context, q AssetSearchQuery, p Page) ([]AssetResponse, int64, error) {
	// 安全な order
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

	// ベース句（SELECT と COUNT で共通）
	baseFrom := `
	FROM assets a
	JOIN assets_master m ON m.asset_master_id = a.asset_master_id
	`

	// WHERE 句と args を共通で作る
	where := "WHERE 1=1"
	args := []any{}
	if q.ManagementNumber != nil {
		where += " AND m.management_number = ?"
		args = append(args, *q.ManagementNumber)
	}
	if q.AssetMasterID != nil {
		where += " AND a.asset_master_id = ?"
		args = append(args, *q.AssetMasterID)
	}
	if q.StatusID != nil {
		where += " AND a.status_id = ?"
		args = append(args, *q.StatusID)
	}
	if q.Owner != nil {
		where += " AND a.owner = ?"
		args = append(args, *q.Owner)
	}
	if q.Location != nil {
		where += " AND a.location = ?"
		args = append(args, *q.Location)
	}
	if q.PurchasedFrom != nil {
		where += " AND a.purchased_at >= ?"
		args = append(args, *q.PurchasedFrom)
	}
	if q.PurchasedTo != nil {
		where += " AND a.purchased_at < ?"
		args = append(args, *q.PurchasedTo)
	}

	// 一覧取得用 SQL
	selectSQL := `
	SELECT a.asset_id, a.asset_master_id, m.management_number, a.serial, a.quantity, a.purchased_at, a.status_id,
		a.owner, a.default_location, a.location, a.last_checked_at, a.last_checked_by, a.notes
	` + baseFrom + `
	` + where + `
	ORDER BY a.purchased_at ` + order + `, a.asset_id ` + order + `
	LIMIT ? OFFSET ?`

	// 実行用引数（where用 + limit/offset）
	queryArgs := append(append([]any{}, args...), p.Limit, p.Offset)

	rows, err := s.db.QueryContext(ctx, selectSQL, queryArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	out := []AssetResponse{}
	for rows.Next() {
		var r AssetResponse
		var serial, loc, lcb, notes sql.NullString
		var lct sql.NullTime
		if err := rows.Scan(
			&r.AssetID, &r.AssetMasterID, &r.ManagementNumber, &serial, &r.Quantity, &r.PurchasedAt, &r.StatusID,
			&r.Owner, &r.DefaultLocation, &loc, &lct, &lcb, &notes,
		); err != nil {
			return nil, 0, err
		}
		if serial.Valid {
			v := serial.String
			r.Serial = &v
		}
		if loc.Valid {
			v := loc.String
			r.Location = &v
		}
		if lct.Valid {
			v := lct.Time
			r.LastCheckedAt = &v
		}
		if lcb.Valid {
			v := lcb.String
			r.LastCheckedBy = &v
		}
		if notes.Valid {
			v := notes.String
			r.Notes = &v
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// カウント用 SQL（LIMIT/OFFSETなし・同じWHERE）
	countSQL := `SELECT COUNT(*) ` + baseFrom + "\n" + where
	var total int64
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	return out, total, nil
}
