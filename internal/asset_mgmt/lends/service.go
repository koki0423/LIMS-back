package lends

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	ulid "github.com/oklog/ulid/v2"
)

// -------------- Error model & mapping --------------

type Code string

const (
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
	CodeNotFound        Code = "NOT_FOUND"
	CodeConflict        Code = "CONFLICT" // 在庫不足・返却過多など
	CodeUnprocessable   Code = "UNPROCESSABLE_ENTITY"
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

// -------------- Clock & ID --------------

type Clock interface{ Now() time.Time }
type realClock struct{}

func (realClock) Now() time.Time { return time.Now().UTC() }

type IDGen interface{ NewULID(t time.Time) string }
type ulidGen struct{}

func (ulidGen) NewULID(t time.Time) string {
	entropy := ulid.Monotonic(rand.Reader, 0)
	return ulid.MustNew(ulid.Timestamp(t), entropy).String()
}

// -------------- Service --------------

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

// POST /assets/:management_number/lends
func (s *Service) CreateLend(ctx context.Context, managementNumber string, in CreateLendRequest) (LendResponse, error) {
	if in.Quantity == 0 {
		return LendResponse{}, ErrInvalid("quantity must be > 0")
	}
	if strings.TrimSpace(in.BorrowerID) == "" {
		return LendResponse{}, ErrInvalid("borrower_id required")
	}

	now := s.clock.Now()
	luid := s.id.NewULID(now)

	// Resolve master first (needed for struct)
	masterID, err := s.store.ResolveMasterID(ctx, managementNumber)
	if err != nil {
		return LendResponse{}, err
	}

	l := &Lend{
		LendULID:         luid,
		AssetMasterID:    masterID,
		ManagementNumber: managementNumber,
		Quantity:         in.Quantity,
		BorrowerID:       in.BorrowerID,
		DueOn:            toNullString(in.DueOn),
		LentByID:         toNullString(in.LentByID),
		Note:             toNullString(in.Note),
	}

	// Delegate transaction and logic to Store
	if err := s.store.ExecCreateLend(ctx, l); err != nil {
		return LendResponse{}, err
	}

	resp := LendResponse{
		LendULID:            luid,
		AssetMasterID:       masterID,
		ManagementNumber:    managementNumber,
		Quantity:            in.Quantity,
		BorrowerID:          in.BorrowerID,
		DueOn:               in.DueOn,
		LentByID:            in.LentByID,
		LentAt:              now,
		ReturnedQuantity:    0,
		OutstandingQuantity: in.Quantity,
		Note:                in.Note,
		Returned:            false,
	}

	return resp, nil
}

func (s *Service) GetLendByULID(ctx context.Context, lendULID string) (LendResponse, error) {
	m, err := s.store.GetLendByULID(ctx, lendULID)
	if err != nil {
		return LendResponse{}, err
	}

	// sum returns
	sum, err := s.store.SumReturned(ctx, m.LendID)
	if err != nil {
		return LendResponse{}, err
	}
	outstanding := uint(0)
	if m.Quantity > sum {
		outstanding = m.Quantity - sum
	}

	// management_number
	mng, err := s.store.GetManagementNumber(ctx, m.AssetMasterID)
	if err != nil {
		return LendResponse{}, err
	}

	return LendResponse{
		LendULID:            m.LendULID,
		AssetMasterID:       m.AssetMasterID,
		ManagementNumber:    mng,
		Quantity:            m.Quantity,
		BorrowerID:          m.BorrowerID,
		DueOn:               nullToPtr(m.DueOn),
		LentByID:            nullToPtr(m.LentByID),
		LentAt:              m.LentAt,
		ReturnedQuantity:    sum,
		OutstandingQuantity: outstanding,
		Note:                nullToPtr(m.Note),
		Returned:            m.Returned,
	}, nil
}

type ListLendsResult struct {
	Items      []LendResponse `json:"items"`
	Total      int64          `json:"total"`
	NextOffset int            `json:"next_offset"`
}

func (s *Service) ListLends(ctx context.Context, f LendFilter, p Page) (ListLendsResult, error) {
	rows, total, err := s.store.ListLends(ctx, f, p)
	if err != nil {
		return ListLendsResult{}, err
	}

	items := make([]LendResponse, 0, len(rows))
	for _, r := range rows {
		outstanding := uint(0)
		if r.Lend.Quantity > r.ReturnedSum {
			outstanding = r.Lend.Quantity - r.ReturnedSum
		}
		items = append(items, LendResponse{
			LendULID:            r.Lend.LendULID,
			AssetMasterID:       r.Lend.AssetMasterID,
			ManagementNumber:    r.ManagementNumber,
			Quantity:            r.Lend.Quantity,
			BorrowerID:          r.Lend.BorrowerID,
			DueOn:               nullToPtr(r.Lend.DueOn),
			LentByID:            nullToPtr(r.Lend.LentByID),
			LentAt:              r.Lend.LentAt,
			ReturnedQuantity:    r.ReturnedSum,
			OutstandingQuantity: outstanding,
			Note:                nullToPtr(r.Lend.Note),
			Returned:            r.Lend.Returned,
		})
	}

	next := p.Offset + p.Limit
	if next >= int(total) {
		next = 0
	} // 0=終端
	return ListLendsResult{Items: items, Total: total, NextOffset: next}, nil
}

type ListReturnsResult struct {
	Items      []ReturnResponse `json:"items"`
	Total      int64            `json:"total"`
	NextOffset int              `json:"next_offset"`
}

func (s *Service) ListReturnsByLend(ctx context.Context, lendULID string, p Page) (ListReturnsResult, error) {
	// resolve lend_id
	l, err := s.store.GetLendByULID(ctx, lendULID)
	if err != nil {
		return ListReturnsResult{}, err
	}

	items, total, err := s.store.ListReturnsByLend(ctx, l.LendID, p)
	if err != nil {
		return ListReturnsResult{}, err
	}

	res := make([]ReturnResponse, 0, len(items))
	for _, it := range items {
		res = append(res, ReturnResponse{
			ReturnULID:    it.ReturnULID,
			LendULID:      lendULID,
			Quantity:      it.Quantity,
			ProcessedByID: nullToPtr(it.ProcessedByID),
			ReturnedAt:    it.ReturnedAt,
			Note:          nullToPtr(it.Note),
		})
	}
	next := p.Offset + p.Limit
	if next >= int(total) {
		next = 0
	}
	return ListReturnsResult{Items: res, Total: total, NextOffset: next}, nil
}

// POST /lends/:lend_ulid/returns
func (s *Service) CreateReturn(ctx context.Context, lendULID string, in CreateReturnRequest) (ReturnResponse, error) {
	if in.Quantity == 0 {
		return ReturnResponse{}, ErrInvalid("quantity must be > 0")
	}
	now := s.clock.Now()
	ruid := s.id.NewULID(now)

	// Get Lend to resolve ID
	l, err := s.store.GetLendByULID(ctx, lendULID)
	if err != nil {
		return ReturnResponse{}, err
	}

	r := &Return{
		ReturnULID:    ruid,
		LendID:        l.LendID,
		Quantity:      in.Quantity,
		ProcessedByID: toNullString(in.ProcessedByID),
		Note:          toNullString(in.Note),
	}

	// Delegate transaction and logic to Store
	if err := s.store.ExecCreateReturn(ctx, r); err != nil {
		return ReturnResponse{}, err
	}

	resp := ReturnResponse{
		ReturnULID:    ruid,
		LendULID:      lendULID,
		Quantity:      in.Quantity,
		ProcessedByID: in.ProcessedByID,
		ReturnedAt:    now,
		Note:          in.Note,
	}

	return resp, nil
}

// helpers

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

// -------------- Error helpers for handler --------------

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
		case CodeUnprocessable:
			return 422
		default:
			return 500
		}
	}
	return 500
}