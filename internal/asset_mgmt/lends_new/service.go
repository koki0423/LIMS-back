package lends_new

import (
	"context"
	"crypto/rand"
	"database/sql"
	"strconv"
	"time"

	"github.com/oklog/ulid/v2"
)

// ===== インターフェース群 =====

type Clock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

type IDGen interface {
	New() (string, error)
}

type ulidGen struct{}

func (ulidGen) New() (string, error) {
	t := time.Now().UTC()
	entropy := ulid.Monotonic(rand.Reader, 0)
	id, err := ulid.New(ulid.Timestamp(t), entropy)
	if err != nil {
		return "", err
	}
	return id.String(), nil
}

// ===== Service本体 =====

type Service struct {
	db    *sql.DB
	store *Store
	clock Clock
	id    IDGen
}

func NewService(db *sql.DB) *Service {
	return &Service{
		db:    db,
		store: NewStore(db),
		clock: realClock{},
		id:    ulidGen{},
	}
}

// 貸出登録
func (s *Service) CreateLend(ctx context.Context, req CreateLendRequest) (*LendResponse, error) {
	if req.Quantity <= 0 {
		return nil, NewInvalidArgumentError("quantity must be > 0")
	}
	if req.BorrowerID == "" {
		return nil, NewInvalidArgumentError("borrower_id is required")
	}

	var assetMasterID int64

	if req.AssetMasterID > 0 {
		assetMasterID = req.AssetMasterID
	} else {
		if req.ManagementNumber == nil || *req.ManagementNumber == "" {
			return nil, NewInvalidArgumentError("either asset_master_id or management_number is required")
		}

		id, err := s.store.ResolveMasterID(ctx, *req.ManagementNumber)
		if err != nil {
			return nil, err
		}
		assetMasterID = id
	}

	// 状態ステータス確認(正常，貸出中以外は貸出不可)
	statusID, err := s.store.GetAssetStatusByMasterID(ctx, assetMasterID)
	if err != nil {
		return nil, err
	}

	if statusID != 1 && statusID != 4 {
		return nil, NewConflictError("only assets with status normal or lent can be lent")
	}

	// 貸出管理カテゴリIDを取得（貸出中のステータス更新に必要）
	managementCategoryID, err := s.store.GetManagementCategoryIDByMasterID(ctx, assetMasterID)
	if err != nil {
		return nil, err
	}

	// 在庫数を取得して貸出数量と比較
	availableQty, err := s.store.GetAvailableQuantityByMasterID(ctx, assetMasterID)
	if err != nil {
		return nil, err
	}

	if req.Quantity > availableQty {
		return nil, NewConflictError("lend quantity exceeds available stock")
	}

	idStr, err := s.id.New()
	if err != nil {
		return nil, err
	}

	now := s.clock.Now()

	var dueOnTime time.Time
	var dueOnValid bool
	if req.DueOn != nil && *req.DueOn != "" {
		parsed, err := time.Parse("2006-01-02", *req.DueOn)
		if err != nil {
			return nil, NewInvalidArgumentError("invalid due_on format, expected YYYY-MM-DD")
		}
		dueOnTime = parsed
		dueOnValid = true
	}

	lend := &Lend{
		LendULID:      idStr,
		AssetMasterID: assetMasterID,
		Quantity:      req.Quantity,
		BorrowerID:    req.BorrowerID,
		LentAt:        now,
		Returned:      false,
	}

	if req.ManagementNumber != nil && *req.ManagementNumber != "" {
		lend.ManagementNumber.String = *req.ManagementNumber
		lend.ManagementNumber.Valid = true
	}

	if dueOnValid {
		lend.DueOn.Time = dueOnTime
		lend.DueOn.Valid = true
	}
	if req.LentByID != nil && *req.LentByID != "" {
		lend.LentByID.String = *req.LentByID
		lend.LentByID.Valid = true
	}
	if req.Note != nil && *req.Note != "" {
		lend.Note.String = *req.Note
		lend.Note.Valid = true
	}

	err = s.store.InsertLend(ctx, lend)
	if err != nil {
		return nil, err
	}

	if managementCategoryID == 1 {
		err = s.store.UpdateAssetStatusInLend(ctx, assetMasterID, 4) // 4: 貸出中
		if err != nil {
			return nil, err
		}
	}

	resp := buildLendResponse(lend, 0)
	return &resp, nil
}

// 返却登録（部分返却対応）
func (s *Service) CreateReturn(ctx context.Context, req CreateReturnRequest) (*ReturnResponse, error) {
	if req.Quantity <= 0 {
		return nil, NewInvalidArgumentError("quantity must be > 0")
	}
	if req.LendID <= 0 {
		return nil, NewInvalidArgumentError("lend_id must be > 0")
	}

	idStr, err := s.id.New()
	if err != nil {
		return nil, err
	}

	tx, err := s.store.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	lend, err := GetLendByIDTx(ctx, tx, req.LendID)
	if err != nil {
		return nil, err
	}

	// 貸出に紐づく資産の管理区分IDを取得（貸出中のステータス更新に必要）
	managementCategoryID, err := s.store.GetManagementCategoryIDByMasterID(ctx, lend.AssetMasterID)
	if err != nil {
		return nil, err
	}

	totalReturned, err := GetTotalReturnedQuantityTx(ctx, tx, req.LendID)
	if err != nil {
		return nil, err
	}

	newTotal := totalReturned + req.Quantity
	if newTotal > lend.Quantity {
		return nil, NewQuantityOverReturnError()
	}

	now := s.clock.Now()
	ret := &Return{
		ReturnULID: idStr,
		LendID:     req.LendID,
		Quantity:   req.Quantity,
		ReturnedAt: now,
	}

	if req.ProcessedByID != nil && *req.ProcessedByID != "" {
		ret.ProcessedByID.String = *req.ProcessedByID
		ret.ProcessedByID.Valid = true
	}
	if req.Note != nil && *req.Note != "" {
		ret.Note.String = *req.Note
		ret.Note.Valid = true
	}

	err = InsertReturnTx(ctx, tx, ret)
	if err != nil {
		return nil, err
	}

	if newTotal == lend.Quantity && !lend.Returned {
		err = UpdateLendReturnedFlagTx(ctx, tx, lend.LendID, true)
		if err != nil {
			return nil, err
		}
	}

	if managementCategoryID == 1 {
		err = s.store.UpdateAssetStatusInReturn(ctx, lend.AssetMasterID, 1) // 1: 正常
		if err != nil {
			return nil, err
		}
	}

	err = tx.Commit()
	if err != nil {
		return nil, err
	}

	resp := ReturnResponse{
		ReturnID:   ret.ReturnID,
		ReturnULID: ret.ReturnULID,
		LendID:     ret.LendID,
		Quantity:   ret.Quantity,
	}

	if ret.ProcessedByID.Valid {
		val := ret.ProcessedByID.String
		resp.ProcessedByID = &val
	}
	resp.ReturnedAt = ret.ReturnedAt
	if ret.Note.Valid {
		val := ret.Note.String
		resp.Note = &val
	}

	return &resp, nil
}

// 貸出単一取得
func (s *Service) GetLend(ctx context.Context, lendID int64) (*LendResponse, error) {
	lend, err := s.store.GetLendByID(ctx, lendID)
	if err != nil {
		return nil, err
	}
	totalReturned, err := s.store.GetTotalReturnedQuantity(ctx, lendID)
	if err != nil {
		return nil, err
	}
	resp := buildLendResponse(lend, totalReturned)
	return &resp, nil
}

// 貸出単一取得（ID or ULID）
func (s *Service) GetLendByKey(ctx context.Context, key string) (*LendResponse, error) {
	if key == "" {
		return nil, NewInvalidArgumentError("id or ulid is required")
	}

	// 数値として解釈できればID検索
	if id, err := strconv.ParseInt(key, 10, 64); err == nil && id > 0 {
		return s.GetLend(ctx, id)
	}

	// それ以外は lend_ulid とみなして検索
	lend, err := s.store.GetLendByULID(ctx, key)
	if err != nil {
		return nil, err
	}
	totalReturned, err := s.store.GetTotalReturnedQuantity(ctx, lend.LendID)
	if err != nil {
		return nil, err
	}
	resp := buildLendResponse(lend, totalReturned)
	return &resp, nil
}

// 貸出一覧
func (s *Service) ListLends(ctx context.Context, filter LendFilter) ([]LendResponse, error) {
	lends, err := s.store.ListLends(ctx, filter)
	if err != nil {
		return nil, err
	}

	var result []LendResponse
	for _, lend := range lends {
		totalReturned, err := s.store.GetTotalReturnedQuantity(ctx, lend.LendID)
		if err != nil {
			return nil, err
		}
		resp := buildLendResponse(lend, totalReturned)
		result = append(result, resp)
	}
	return result, nil
}

// 返却単一取得
func (s *Service) GetReturn(ctx context.Context, returnID int64) (*ReturnResponse, error) {
	ret, err := s.store.GetReturnByID(ctx, returnID)
	if err != nil {
		return nil, err
	}

	resp := ReturnResponse{
		ReturnID:   ret.ReturnID,
		ReturnULID: ret.ReturnULID,
		LendID:     ret.LendID,
		Quantity:   ret.Quantity,
	}
	if ret.ProcessedByID.Valid {
		val := ret.ProcessedByID.String
		resp.ProcessedByID = &val
	}
	resp.ReturnedAt = ret.ReturnedAt
	if ret.Note.Valid {
		val := ret.Note.String
		resp.Note = &val
	}
	return &resp, nil
}

// 返却単一取得（ID or ULID）
func (s *Service) GetReturnByKey(ctx context.Context, key string) (*ReturnResponse, error) {
	if key == "" {
		return nil, NewInvalidArgumentError("id or ulid is required")
	}

	// 数値なら return_id
	if id, err := strconv.ParseInt(key, 10, 64); err == nil && id > 0 {
		return s.GetReturn(ctx, id)
	}

	// それ以外は return_ulid
	ret, err := s.store.GetReturnByULID(ctx, key)
	if err != nil {
		return nil, err
	}

	resp := ReturnResponse{
		ReturnID:   ret.ReturnID,
		ReturnULID: ret.ReturnULID,
		LendID:     ret.LendID,
		Quantity:   ret.Quantity,
	}

	if ret.ProcessedByID.Valid {
		v := ret.ProcessedByID.String
		resp.ProcessedByID = &v
	}
	resp.ReturnedAt = ret.ReturnedAt
	if ret.Note.Valid {
		v := ret.Note.String
		resp.Note = &v
	}

	return &resp, nil
}

// 返却一覧
func (s *Service) ListReturns(ctx context.Context, filter ReturnFilter) ([]ReturnResponse, error) {
	returns, err := s.store.ListReturns(ctx, filter)
	if err != nil {
		return nil, err
	}

	var result []ReturnResponse
	for _, ret := range returns {
		resp := ReturnResponse{
			ReturnID:   ret.ReturnID,
			ReturnULID: ret.ReturnULID,
			LendID:     ret.LendID,
			Quantity:   ret.Quantity,
		}
		if ret.ProcessedByID.Valid {
			val := ret.ProcessedByID.String
			resp.ProcessedByID = &val
		}
		resp.ReturnedAt = ret.ReturnedAt
		if ret.Note.Valid {
			val := ret.Note.String
			resp.Note = &val
		}
		result = append(result, resp)
	}
	return result, nil
}

// ヘルパー関数
func buildLendResponse(lend *Lend, returnedQty int) LendResponse {
	resp := LendResponse{
		LendID:           lend.LendID,
		LendULID:         lend.LendULID,
		AssetMasterID:    lend.AssetMasterID,
		Quantity:         lend.Quantity,
		BorrowerID:       lend.BorrowerID,
		LentAt:           lend.LentAt,
		Returned:         lend.Returned,
		ReturnedQuantity: returnedQty,
	}

	if lend.ManagementNumber.Valid {
		val := lend.ManagementNumber.String
		resp.ManagementNumber = &val
	}
	if lend.DueOn.Valid {
		val := lend.DueOn.Time
		resp.DueOn = &val
	}
	if lend.LentByID.Valid {
		val := lend.LentByID.String
		resp.LentByID = &val
	}
	if lend.Note.Valid {
		val := lend.Note.String
		resp.Note = &val
	}
	return resp
}
