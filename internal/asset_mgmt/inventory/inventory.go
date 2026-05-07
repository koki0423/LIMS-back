package inventory

import (
	"context"
	"database/sql"

	platformdb "IRIS-backend/internal/platform/db"
)

const (
	StatusNormal           = 1
	StatusLent             = 4
	StatusZeroStock        = 5
	ManagementCategoryLend = 1
)

type LockedAssetRow struct {
	AssetID  uint64
	Quantity int
}

type QuantityAdjustment struct {
	AssetID uint64
	Delta   int
}

func ResolveMasterID(ctx context.Context, q platformdb.DBTX, managementNumber string) (uint64, error) {
	const query = `
SELECT asset_master_id
FROM assets_master
WHERE management_number = ?
LIMIT 1`

	var id uint64
	if err := q.QueryRowContext(ctx, query, managementNumber).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func GetManagementCategoryIDByMasterID(ctx context.Context, q platformdb.DBTX, assetMasterID int64) (int, error) {
	const query = `
SELECT management_category_id
FROM assets_master
WHERE asset_master_id = ?
LIMIT 1`

	var managementCategoryID int
	if err := q.QueryRowContext(ctx, query, assetMasterID).Scan(&managementCategoryID); err != nil {
		return 0, err
	}
	return managementCategoryID, nil
}

func LockAssetRowsByMasterID(ctx context.Context, q platformdb.DBTX, assetMasterID int64) ([]LockedAssetRow, error) {
	const query = `
SELECT asset_id, quantity
FROM assets
WHERE asset_master_id = ?
ORDER BY asset_id
FOR UPDATE`

	rows, err := q.QueryContext(ctx, query, assetMasterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	locked := make([]LockedAssetRow, 0, 4)
	for rows.Next() {
		var row LockedAssetRow
		if err := rows.Scan(&row.AssetID, &row.Quantity); err != nil {
			return nil, err
		}
		locked = append(locked, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(locked) == 0 {
		return nil, sql.ErrNoRows
	}
	return locked, nil
}

func GetTotalQuantityByMasterID(ctx context.Context, q platformdb.DBTX, assetMasterID int64) (int, error) {
	const query = `
SELECT COALESCE(SUM(quantity), 0)
FROM assets
WHERE asset_master_id = ?`

	var total int
	if err := q.QueryRowContext(ctx, query, assetMasterID).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func GetOutstandingQuantityByMasterID(ctx context.Context, q platformdb.DBTX, assetMasterID int64) (int, error) {
	const query = `
SELECT COALESCE(SUM(l.quantity), 0) - COALESCE(SUM(r.returned_qty), 0) AS outstanding_qty
FROM lends l
LEFT JOIN (
	SELECT lend_id, SUM(quantity) AS returned_qty
	FROM returns
	GROUP BY lend_id
) r
	ON l.lend_id = r.lend_id
WHERE l.asset_master_id = ?`

	var outstanding int
	if err := q.QueryRowContext(ctx, query, assetMasterID).Scan(&outstanding); err != nil {
		return 0, err
	}
	return outstanding, nil
}

func GetAvailableQuantityByMasterID(ctx context.Context, q platformdb.DBTX, assetMasterID int64) (int, error) {
	totalQty, err := GetTotalQuantityByMasterID(ctx, q, assetMasterID)
	if err != nil {
		return 0, err
	}

	outstandingQty, err := GetOutstandingQuantityByMasterID(ctx, q, assetMasterID)
	if err != nil {
		return 0, err
	}

	return totalQty - outstandingQty, nil
}

func ComputeDisposalPlan(rows []LockedAssetRow, requestQty int) ([]QuantityAdjustment, error) {
	if requestQty <= 0 {
		return nil, ErrInvalidQuantity
	}

	remaining := requestQty
	adjustments := make([]QuantityAdjustment, 0, len(rows))
	for _, row := range rows {
		if row.Quantity <= 0 {
			continue
		}

		deduct := row.Quantity
		if deduct > remaining {
			deduct = remaining
		}

		adjustments = append(adjustments, QuantityAdjustment{
			AssetID: row.AssetID,
			Delta:   -deduct,
		})
		remaining -= deduct
		if remaining == 0 {
			return adjustments, nil
		}
	}

	if remaining != 0 {
		return nil, ErrInsufficientStock
	}
	return adjustments, nil
}

func ApplyQuantityAdjustments(ctx context.Context, q platformdb.DBTX, adjustments []QuantityAdjustment) error {
	const query = `
UPDATE assets
SET quantity = quantity + ?
WHERE asset_id = ?`

	for _, adj := range adjustments {
		res, err := q.ExecContext(ctx, query, adj.Delta, adj.AssetID)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return sql.ErrNoRows
		}
	}

	return nil
}

func DetermineStatus(totalQty int, outstandingQty int, managementCategoryID int) int {
	if totalQty <= 0 {
		return StatusZeroStock
	}
	if managementCategoryID == ManagementCategoryLend && outstandingQty > 0 {
		return StatusLent
	}
	return StatusNormal
}

func ReconcileAssetStatus(ctx context.Context, q platformdb.DBTX, assetMasterID int64) error {
	managementCategoryID, err := GetManagementCategoryIDByMasterID(ctx, q, assetMasterID)
	if err != nil {
		return err
	}

	totalQty, err := GetTotalQuantityByMasterID(ctx, q, assetMasterID)
	if err != nil {
		return err
	}

	outstandingQty, err := GetOutstandingQuantityByMasterID(ctx, q, assetMasterID)
	if err != nil {
		return err
	}

	statusID := DetermineStatus(totalQty, outstandingQty, managementCategoryID)
	const query = `
UPDATE assets
SET status_id = ?
WHERE asset_master_id = ?`
	_, err = q.ExecContext(ctx, query, statusID, assetMasterID)
	return err
}
