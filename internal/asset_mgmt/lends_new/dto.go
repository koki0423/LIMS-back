package lends_new

import "time"

// 貸出登録リクエスト
type CreateLendRequest struct {
	// フロントからは原則 management_number だけ来る想定
	// 内部バッチ等で asset_master_id を直接指定したい場合用に残しておく
	AssetMasterID    int64   `json:"asset_master_id" `
	ManagementNumber *string `json:"management_number" binding:"required"`
	Quantity         int     `json:"quantity" binding:"required"`
	BorrowerID       string  `json:"borrower_id" binding:"required"`
	// "2006-01-02" 形式の文字列を想定（DATE）
	DueOn    *string `json:"due_on,omitempty"`
	LentByID *string `json:"lent_by_id,omitempty"`
	Note     *string `json:"note,omitempty"`
}

// 返却登録リクエスト
type CreateReturnRequest struct {
	LendID        int64   `json:"lend_id"`
	Quantity      int     `json:"quantity"`
	ProcessedByID *string `json:"processed_by_id,omitempty"`
	Note          *string `json:"note,omitempty"`
}

// 貸出レスポンス
type LendResponse struct {
	LendID           int64      `json:"lend_id"`
	LendULID         string     `json:"lend_ulid"`
	AssetMasterID    int64      `json:"asset_master_id"`
	ManagementNumber *string    `json:"management_number,omitempty"`
	Quantity         int        `json:"quantity"`
	BorrowerID       string     `json:"borrower_id"`
	DueOn            *time.Time `json:"due_on,omitempty"`
	LentByID         *string    `json:"lent_by_id,omitempty"`
	LentAt           time.Time  `json:"lent_at"`
	Note             *string    `json:"note,omitempty"`
	Returned         bool       `json:"returned"`
	ReturnedQuantity int        `json:"returned_quantity"`
}

// 返却レスポンス
type ReturnResponse struct {
	ReturnID      int64     `json:"return_id"`
	ReturnULID    string    `json:"return_ulid"`
	LendID        int64     `json:"lend_id"`
	Quantity      int       `json:"quantity"`
	ProcessedByID *string   `json:"processed_by_id,omitempty"`
	ReturnedAt    time.Time `json:"returned_at"`
	Note          *string   `json:"note,omitempty"`
	// 返却元の貸出情報を一部返したいならここに追加
}
