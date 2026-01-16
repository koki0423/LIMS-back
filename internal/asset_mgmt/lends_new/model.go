package lends_new

import (
	"database/sql"
	"time"
)

// Lend は lends テーブルの1行を表す
type Lend struct {
	LendID           int64
	LendULID         string
	AssetMasterID    int64
	ManagementNumber sql.NullString
	Quantity         int
	BorrowerID       string
	DueOn            sql.NullTime
	LentByID         sql.NullString
	LentAt           time.Time
	Note             sql.NullString
	Returned         bool
}

// Return は returns テーブルの1行を表す
type Return struct {
	ReturnID      int64
	ReturnULID    string
	LendID        int64
	Quantity      int
	ProcessedByID sql.NullString
	ReturnedAt    time.Time
	Note          sql.NullString
}

// 貸出リスト取得用の検索条件
type LendFilter struct {
	BorrowerID     string
	AssetMasterID  *int64
	ManagementNumber string
	Returned       *bool
	Limit          int
	Offset         int
}

// 返却リスト取得用の検索条件
type ReturnFilter struct {
	BorrowerID     string // joins lends.borrower_id で絞る場合用
	AssetMasterID  *int64
	LendID         *int64
	Limit          int
	Offset         int
}
