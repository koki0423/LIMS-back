package assets

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	mysql "github.com/go-sql-driver/mysql"
	ulid "github.com/oklog/ulid/v2"
)

// ===== Error model =====
type Code string

const (
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
	CodeNotFound        Code = "NOT_FOUND"
	CodeConflict        Code = "CONFLICT"
	CodeInternal        Code = "INTERNAL"
)

type APIError struct {
	Code    Code
	Message string
}

func (e *APIError) Error() string      { return fmt.Sprintf("%s: %s", e.Code, e.Message) }
func ErrInvalid(msg string) *APIError  { return &APIError{Code: CodeInvalidArgument, Message: msg} }
func ErrNotFound(msg string) *APIError { return &APIError{Code: CodeNotFound, Message: msg} }
func ErrConflict(msg string) *APIError { return &APIError{Code: CodeConflict, Message: msg} }
func ErrInternal(msg string) *APIError { return &APIError{Code: CodeInternal, Message: msg} }

func toHTTPStatus(err error) int {
	var api *APIError
	if errors.As(err, &api) {
		switch api.Code {
		case CodeInvalidArgument:
			return 400
		case CodeNotFound:
			return 404
		case CodeConflict:
			return 409
		default:
			return 500
		}
	}
	return 500
}

type Service struct {
	db    *sql.DB
	store *Store
}

func NewService(db *sql.DB) *Service { return &Service{db: db, store: NewStore(db)} }

// ===== Master =====

func (s *Service) CreateAssetMaster(ctx context.Context, in CreateAssetMasterRequest) (AssetMasterResponse, error) {
	if strings.TrimSpace(in.Name) == "" || strings.TrimSpace(in.Manufacturer) == "" ||
		in.ManagementCategoryID == 0 || in.GenreID == 0 {
		return AssetMasterResponse{}, ErrInvalid("name, manufacturer, management_category_id, genre_id are required")
	}

	// 仮管理番号（UNIQUEを満たす）
	tmpMng := "TMP-" + ulid.Make().String()

	// 1) 仮INSERT → PK取得
	id, err := s.store.InsertMasterTmp(ctx, in, tmpMng)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) {
			switch me.Number {
			case 1062: // duplicate key
				return AssetMasterResponse{}, ErrConflict("management_number already exists")
			case 1452: // foreign key constraint fails
				return AssetMasterResponse{}, ErrInvalid("invalid management_category_id or genre_id")
			}
		}
		return AssetMasterResponse{}, err
	}

	// 2) 確定管理番号に置換（DBの created_at と genres.genre_code を使用）
	if err := s.store.UpdateMngToFinal(ctx, id, tmpMng, 5 /*パディング桁*/); err != nil {
		// if errors.Is(err, ErrConflict) {
		// 	return AssetMasterResponse{}, ErrConflict("conflict while finalizing management_number")
		// }
		var ae *APIError
		if errors.As(err, &ae) && ae.Code == CodeConflict {
			return AssetMasterResponse{}, ErrConflict("conflict while finalizing management_number")
		}

		return AssetMasterResponse{}, err
	}

	// 3) IDで取得して返却
	out, err := s.store.GetMasterByID(ctx, id)
	if err != nil {
		return AssetMasterResponse{}, err
	}
	return *out, nil
}

func (s *Service) GetAssetMaster(ctx context.Context, managementNumber string) (AssetMasterResponse, error) {
	out, err := s.store.GetMasterByMng(ctx, managementNumber)
	if err != nil {
		if err == sql.ErrNoRows {
			return AssetMasterResponse{}, ErrNotFound("master not found")
		}
		return AssetMasterResponse{}, err
	}
	return *out, nil
}

func (s *Service) ListAssetMasters(ctx context.Context, p Page, q AssetSearchQuery) ([]AssetMasterResponse, int64, error) {
	items, total, err := s.store.ListMasters(ctx, p, q)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Service) UpdateAssetMaster(ctx context.Context, managementNumber string, in UpdateAssetMasterRequest) (AssetMasterResponse, error) {
	out, err := s.store.UpdateMasterByMng(ctx, managementNumber, in)
	if err != nil {
		if err == sql.ErrNoRows {
			return AssetMasterResponse{}, ErrNotFound("master not found")
		}
		return AssetMasterResponse{}, err
	}
	return *out, nil
}

// ===== Assets =====

func (s *Service) CreateAsset(ctx context.Context, in CreateAssetRequest) (AssetResponse, error) {
	var masterID uint64
	if in.AssetMasterID == nil {
		log.Printf("asset_master_id is required")
		return AssetResponse{}, ErrInvalid("either asset_master_id or management_number is required")
	} else if in.AssetMasterID != nil {
		log.Printf("asset_master_id: %d", *in.AssetMasterID)
		masterID = *in.AssetMasterID
	}

	// quantity >= 0
	if int(in.Quantity) < 0 {
		log.Printf("quantity must be >= 0")
		return AssetResponse{}, ErrInvalid("quantity must be >= 0")
	}
	if strings.TrimSpace(in.Owner) == "" || strings.TrimSpace(in.DefaultLocation) == "" {
		log.Printf("owner/default_location required")
		return AssetResponse{}, ErrInvalid("owner/default_location required")
	}
	if in.PurchasedAt.IsZero() {
		log.Printf("purchased_at required")
		return AssetResponse{}, ErrInvalid("purchased_at required")
	}

	id, mgmt, err := s.store.CreateAssetTx(ctx, in, masterID)
	if err != nil {
		return AssetResponse{}, err
	}

	return AssetResponse{
		AssetID:          id,
		ManagementNumber: mgmt,
		// 必要ならその他の最小項目をここで埋める
	}, nil
}

func (s *Service) GetAsset(ctx context.Context, id uint64) (AssetResponse, error) {
	out, err := s.store.GetAssetByID(ctx, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return AssetResponse{}, ErrNotFound("asset not found")
		}
		return AssetResponse{}, err
	}
	return *out, nil
}

func (s *Service) ListAssets(ctx context.Context, q AssetSearchQuery, p Page) ([]AssetResponse, int64, error) {
	items, total, err := s.store.ListAssets(ctx, q, p)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *Service) UpdateAsset(ctx context.Context, id uint64, in UpdateAssetRequest) (AssetResponse, error) {
	if in.Quantity != nil && int(*in.Quantity) < 0 {
		return AssetResponse{}, ErrInvalid("quantity must be >= 0")
	}
	out, err := s.store.UpdateAssetByID(ctx, id, in)
	if err != nil {
		if err == sql.ErrNoRows {
			return AssetResponse{}, ErrNotFound("asset not found")
		}
		return AssetResponse{}, err
	}
	return *out, nil
}


// ===== Asset Set =====
// 将来的にcreateAssetMasterとCreateAssetを廃止してこっちへ移行．ただしAndroidとフロントエンドの対応が終わり次第移行すること．
func (s *Service) CreateAssetSet(ctx context.Context, req CreateAssetSetRequest) (AssetSetResponse, error) {
	// ---- validate master ----
	if strings.TrimSpace(req.Master.Name) == "" ||
		strings.TrimSpace(req.Master.Manufacturer) == "" ||
		req.Master.ManagementCategoryID == 0 ||
		req.Master.GenreID == 0 {
		return AssetSetResponse{}, ErrInvalid("master.name, master.manufacturer, master.management_category_id, master.genre_id are required")
	}

	// ---- validate asset ----
	if strings.TrimSpace(req.Asset.Owner) == "" || strings.TrimSpace(req.Asset.DefaultLocation) == "" {
		return AssetSetResponse{}, ErrInvalid("asset.owner/default_location required")
	}
	if req.Asset.PurchasedAt.IsZero() {
		return AssetSetResponse{}, ErrInvalid("asset.purchased_at required")
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return AssetSetResponse{}, err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	// 1) master 仮INSERT
	tmpMng := "TMP-" + ulid.Make().String()
	masterID, err := s.store.InsertMasterTmpTx(ctx, tx, req.Master, tmpMng)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) {
			if me.Number == 1452 {
				return AssetSetResponse{}, ErrInvalid("invalid management_category_id or genre_id")
			}
			if me.Number == 1062 {
				return AssetSetResponse{}, ErrConflict("management_number already exists")
			}
		}
		return AssetSetResponse{}, err
	}

	// 2) master 確定番号に置換
	if err := s.store.UpdateMngToFinalTx(ctx, tx, masterID, tmpMng, 5); err != nil {
		var ae *APIError
		if errors.As(err, &ae) && ae.Code == CodeConflict {
			return AssetSetResponse{}, ErrConflict("conflict while finalizing management_number")
		}
		return AssetSetResponse{}, err
	}

	// 3) asset INSERT
	assetID, err := s.store.InsertAssetTx(ctx, tx, req.Asset, masterID)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) {
			if me.Number == 1452 {
				return AssetSetResponse{}, ErrInvalid("invalid foreign key (status_id etc)")
			}
			if me.Number == 1062 {
				return AssetSetResponse{}, ErrConflict("duplicate key")
			}
		}
		return AssetSetResponse{}, err
	}

	if err := tx.Commit(); err != nil {
		return AssetSetResponse{}, err
	}
	committed = true

	// 4) 返却用に取り直し（コミット後、DBからフルDTO）
	m, err := s.store.GetMasterByID(ctx, masterID)
	if err != nil {
		return AssetSetResponse{}, err
	}
	a, err := s.store.GetAssetByID(ctx, assetID)
	if err != nil {
		return AssetSetResponse{}, err
	}

	return AssetSetResponse{Master: *m, Asset: *a}, nil
}

func (s *Service) GetAssetSet(ctx context.Context, managementNumber string) (AssetSetResponse, error) {
	out, err := s.store.GetAssetSetByMng(ctx, managementNumber)
	if err != nil {
		if err == sql.ErrNoRows {
			return AssetSetResponse{}, ErrNotFound("asset set not found")
		}
		return AssetSetResponse{}, err
	}
	return *out, nil
}