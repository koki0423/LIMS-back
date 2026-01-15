package lends_new

import (
	"database/sql"
	"time"
)

type Lend struct {
	LendID           uint64
	LendULID         string
	AssetMasterID    uint64
	// ManagementNumber string // <- 重複するため削除
	Quantity         uint
	BorrowerID       string
	DueOn            sql.NullString // DATEを文字列で扱う（"2006-01-02"）
	LentByID         sql.NullString
	LentAt           time.Time
	Note             sql.NullString
	Returned         bool // tinyint(1) -> bool
}

type Return struct {
	ReturnID      uint64
	ReturnULID    string
	LendID        uint64
	Quantity      uint
	ProcessedByID sql.NullString
	ReturnedAt    time.Time
	Note          sql.NullString
}