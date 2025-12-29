package disposals

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"log"

	ulid "github.com/oklog/ulid/v2"
)

// ---- Error model ----
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

func (e *APIError) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }
func ErrInvalid(msg string) *APIError  { return &APIError{Code: CodeInvalidArgument, Message: msg} }
func ErrNotFound(msg string) *APIError { return &APIError{Code: CodeNotFound, Message: msg} }
func ErrConflict(msg string) *APIError { return &APIError{Code: CodeConflict, Message: msg} }
func ErrInternal(msg string) *APIError { return &APIError{Code: CodeInternal, Message: msg} }

// ---- Clock & ID ----
type Clock interface{ Now() time.Time }
type realClock struct{}
func (realClock) Now() time.Time { return time.Now().UTC() }

type IDGen interface{ NewULID(t time.Time) string }
type ulidGen struct{}
func (ulidGen) NewULID(t time.Time) string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	return ulid.MustNew(ulid.Timestamp(t), entropy).String()
}

// ---- Service ----

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

func (s *Service) withTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil { return err }
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// POST /assets/:management_number/disposals
func (s *Service) CreateDisposal(ctx context.Context, managementNumber string, in CreateDisposalRequest) (DisposalResponse, error) {
	if in.Quantity == 0 {
		return DisposalResponse{}, ErrInvalid("quantity must be > 0")
	}
	now := s.clock.Now()
	duid := s.id.NewULID(now)

	var resp DisposalResponse
	err := s.withTx(ctx, func(tx *sql.Tx) error {
		// master解決
		masterID, err := s.store.ResolveMasterID(ctx, managementNumber)
		if err != nil { return err }

		// 在庫ロック & チェック
		assetID, qty, err := s.store.LockAssetRow(ctx, tx, masterID) // SELECT ... FOR UPDATE
		if err != nil { return err }
		if int(qty) - int(in.Quantity) < 0 {
			return ErrConflict("insufficient stock")
		}
		// log.Printf("Locked assetID: %d with quantity: %d", assetID, qty)

		// 在庫減算
		if err := s.store.UpdateAssetQuantity(ctx, tx, assetID, -int(in.Quantity)); err != nil {
			return err
		}
		log.Printf("Updated assetID: %d quantity by %d", assetID, -int(in.Quantity))

		// 減算後が0ならステータスだけ変更
		newQty := int(qty) - int(in.Quantity)
		if newQty == 0 {
			const StatusZeroStock = 5
			if err := s.store.UpdateAssetStatus(ctx, tx, assetID, StatusZeroStock); err != nil {
				log.Printf("Failed to update asset status: %v", err)
				return err
			}
		}

		// 廃棄挿入
		m := &Disposal{
			DisposalULID:     duid,
			ManagementNumber: managementNumber,
			Quantity:         in.Quantity,
			Reason:           toNullString(in.Reason),
			ProcessedByID:    toNullString(in.ProcessedByID),
		}
		if _, err := s.store.InsertDisposal(ctx, tx, m); err != nil {
			log.Printf("Failed to insert disposal record: %v", err)
			return err
		}

		resp = DisposalResponse{
			DisposalULID:     duid,
			ManagementNumber: managementNumber,
			Quantity:         in.Quantity,
			Reason:           in.Reason,
			ProcessedByID:    in.ProcessedByID,
			DisposedAt:       now,
		}
		return nil
	})
	return resp, err
}


func (s *Service) GetDisposalByULID(ctx context.Context, ul string) (DisposalResponse, error) {
	m, err := s.store.GetByULID(ctx, ul)
	if err != nil { return DisposalResponse{}, err }
	return DisposalResponse{
		DisposalULID:     m.DisposalULID,
		ManagementNumber: m.ManagementNumber,
		Quantity:         m.Quantity,
		Reason:           nullToPtr(m.Reason),
		ProcessedByID:    nullToPtr(m.ProcessedByID),
		DisposedAt:       m.DisposedAt,
	}, nil
}

type ListResult struct {
	Items      []DisposalResponse `json:"items"`
	Total      int64              `json:"total"`
	NextOffset int                `json:"next_offset"`
}

func (s *Service) ListDisposals(ctx context.Context, f DisposalFilter, p Page) (ListResult, error) {
	rows, total, err := s.store.List(ctx, f, p)
	if err != nil { return ListResult{}, err }
	items := make([]DisposalResponse, 0, len(rows))
	for _, m := range rows {
		items = append(items, DisposalResponse{
			DisposalULID:     m.DisposalULID,
			ManagementNumber: m.ManagementNumber,
			Quantity:         m.Quantity,
			Reason:           nullToPtr(m.Reason),
			ProcessedByID:    nullToPtr(m.ProcessedByID),
			DisposedAt:       m.DisposedAt,
		})
	}
	next := p.Offset + p.Limit
	if next >= int(total) { next = 0 }
	return ListResult{Items: items, Total: total, NextOffset: next}, nil
}

// ---- helpers ----
func toNullString(s *string) (ns sql.NullString) {
	if s != nil && strings.TrimSpace(*s) != "" {
		ns.Valid, ns.String = true, *s
	}
	return
}
func nullToPtr(ns sql.NullString) *string {
	if ns.Valid {
		v := ns.String
		return &v
	}
	return nil
}

func ToHTTPStatus(err error) int {
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
