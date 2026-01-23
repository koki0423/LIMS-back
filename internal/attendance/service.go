package attendance

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// ===== Error model (assets/disposals/lends と同型) =====
type Code string

const (
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
	CodeNotFound        Code = "NOT_FOUND"
	CodeConflict        Code = "CONFLICT"
	CodeInternal        Code = "INTERNAL"
)

type APIError struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
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

// ===== Service =====

type Service struct {
	db    *sql.DB
	store *Store
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db, store: NewStore(db)}
}

// POST /attendances
func (s *Service) UpsertAttendance(ctx context.Context, in CreateAttendanceRequest) (AttendanceResponse, bool, error) {
	if in.StudentNumber == "" {
		return AttendanceResponse{}, false, ErrInvalid("user_id is required")
	}
	var on *time.Time
	if in.AttendedOn != nil && *in.AttendedOn != "" {
		parsed, err := parseOn(*in.AttendedOn)
		if err != nil {
			return AttendanceResponse{}, false, ErrInvalid("attended_on must be YYYY-MM-DD or 'today'")
		}
		on = &parsed
	}

	row, created, err := s.store.Upsert(ctx, in.StudentNumber, on, in.Note)
	if err != nil {
		return AttendanceResponse{}, false, err
	}
	return row.toDTO(), created, nil
}

// HEAD /attendances?user_id=&on=
func (s *Service) Exists(ctx context.Context, userID string, onStr string) (bool, error) {
	if userID == "" {
		return false, ErrInvalid("user_id is required")
	}
	on, err := parseOn(onStr)
	if err != nil {
		return false, ErrInvalid("on must be YYYY-MM-DD or 'today'")
	}
	return s.store.Exists(ctx, userID, on)
}

// GET /attendances
func (s *Service) List(ctx context.Context, q ListQuery) ([]AttendanceResponse, int64, error) {
	if q.Sort == "" {
		q.Sort = DefaultSort
	}
	if q.Limit <= 0 {
		q.Limit = DefaultPageLimit
	}
	if q.Limit > MaxPageLimit {
		q.Limit = MaxPageLimit
	}

	rows, total, err := s.store.List(ctx, q)
	if err != nil {
		return nil, 0, err
	}
	out := make([]AttendanceResponse, 0, len(rows))
	for i := 0; i < len(rows); i++ {
		out = append(out, rows[i].toDTO())
	}
	return out, total, nil
}

// GET /attendances/stats
func (s *Service) Stats(ctx context.Context, req StatsRequest) ([]StatsRow, error) {
	from, err := time.ParseInLocation(DateLayout, req.From, time.UTC)
	if err != nil {
		return nil, ErrInvalid("from must be YYYY-MM-DD")
	}
	to, err := time.ParseInLocation(DateLayout, req.To, time.UTC)
	if err != nil {
		return nil, ErrInvalid("to must be YYYY-MM-DD")
	}
	if to.Before(from) {
		return nil, ErrInvalid("to must be >= from")
	}
	return s.store.Stats(ctx, from, to, req.Limit)
}

func parseOn(s string) (time.Time, error) {
	n := normalizeDateString(s)
	return time.ParseInLocation(DateLayout, n, time.UTC)
}
