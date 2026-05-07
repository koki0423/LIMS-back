package disposals

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"log"
	"strings"
	"time"

	"IRIS-backend/internal/asset_mgmt/inventory"

	ulid "github.com/oklog/ulid/v2"
)

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
	if err != nil {
		return err
	}
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
		masterID, err := s.store.ResolveMasterIDTx(ctx, tx, managementNumber)
		if err != nil {
			return err
		}

		// 在庫ロック & 廃棄計画作成
		lockedRows, err := s.store.LockAssetRows(ctx, tx, masterID)
		if err != nil {
			return err
		}

		adjustments, err := inventory.ComputeDisposalPlan(lockedRows, int(in.Quantity))
		if errors.Is(err, inventory.ErrInsufficientStock) {
			return ErrConflict("insufficient stock")
		}
		if errors.Is(err, inventory.ErrInvalidQuantity) {
			return ErrInvalid("quantity must be > 0")
		}
		if err != nil {
			return err
		}

		if err := s.store.ApplyQuantityAdjustments(ctx, tx, adjustments); err != nil {
			return err
		}

		if err := s.store.ReconcileAssetStatus(ctx, tx, masterID); err != nil {
			log.Printf("Failed to reconcile asset status: %v", err)
			return err
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
	if err != nil {
		return DisposalResponse{}, err
	}
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
	if err != nil {
		return ListResult{}, err
	}
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
	if next >= int(total) {
		next = 0
	}
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
