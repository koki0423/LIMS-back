package attendance

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type DBTX interface {
	ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row
}

type Store struct{ db DBTX }

func NewStore(db DBTX) *Store { return &Store{db: db} }

// Upsert: student_number + attended_on（UNIQUE）でINSERTまたはUPDATE。
// 返り値: 確定行（id含む）、created=true（新規）/false（更新）
func (s *Store) Upsert(ctx context.Context, student string, attendedOn *time.Time, note *string) (Attendance, bool, error) {
	// INSERT ... ON DUPLICATE KEY UPDATE
	// - 新規: RowsAffected = 1
	// - 既存更新: RowsAffected = 2
	var q = `
	INSERT INTO attendances (student_number, attended_on, clocked_at, note)
	VALUES (?, COALESCE(?, UTC_DATE()), UTC_TIMESTAMP(), ?)
	ON DUPLICATE KEY UPDATE
	clocked_at = VALUES(clocked_at),
	note       = VALUES(note)`

	attOn := any(nil)
	if attendedOn != nil {
		attOn = attendedOn.Format(DateLayout)
	}
	res, err := s.db.ExecContext(ctx, q, student, attOn, noteOrNil(note))
	if err != nil {
		return Attendance{}, false, err
	}
	aff, _ := res.RowsAffected()
	created := (aff == 1)

	// 最終行を取得（UNIQUEキーで）
	row := s.db.QueryRowContext(ctx, `
	SELECT attendance_id, student_number, DATE_FORMAT(attended_on, '%Y-%m-%d') as attended_on, clocked_at, note
	FROM attendances
	WHERE student_number = ?
	AND attended_on = COALESCE(?, UTC_DATE())`,
		student, attOn,
	)
	var r attendanceRow
	if err := row.Scan(&r.AttendanceID, &r.StudentNumber, &r.AttendedOn, &r.ClockedAt, &r.Note); err != nil {
		if err == sql.ErrNoRows {
			return Attendance{}, created, ErrInternal("inserted but not found")
		}
		return Attendance{}, created, err
	}
	return r.toModel(), created, nil
}

// Exists: 指定ユーザが指定日(=on)に存在するか
func (s *Store) Exists(ctx context.Context, student string, on time.Time) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, `
	SELECT 1 FROM attendances
	WHERE student_number = ? AND attended_on = ? LIMIT 1`, student, on.Format(DateLayout),
	).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// List: 条件に応じて動的WHERE + ORDER + LIMIT/OFFSET
func (s *Store) List(ctx context.Context, q ListQuery) ([]Attendance, int64, error) {
	var (
		buf    bytes.Buffer
		args   []any
		wheres []string
	)

	buf.WriteString(`
	SELECT attendance_id, student_number, DATE_FORMAT(attended_on, '%Y-%m-%d') AS attended_on, clocked_at, note
	FROM attendances
	`)
	// WHERE
	if q.StudentNumber != nil && *q.StudentNumber != "" {
		wheres = append(wheres, "student_number = ?")
		args = append(args, *q.StudentNumber)
	}
	if q.On != nil && *q.On != "" {
		wheres = append(wheres, "attended_on = ?")
		args = append(args, normalizeDateString(*q.On))
	} else {
		if q.From != nil && *q.From != "" {
			wheres = append(wheres, "attended_on >= ?")
			args = append(args, mustDate(*q.From))
		}
		if q.To != nil && *q.To != "" {
			wheres = append(wheres, "attended_on <= ?")
			args = append(args, mustDate(*q.To))
		}
	}
	if len(wheres) > 0 {
		buf.WriteString(" WHERE " + strings.Join(wheres, " AND "))
	}

	// ORDER
	switch q.Sort {
	case SortClockedAtAsc:
		buf.WriteString(" ORDER BY clocked_at ASC, attendance_id ASC")
	case SortAttendedOnDesc:
		buf.WriteString(" ORDER BY attended_on DESC, clocked_at DESC, attendance_id DESC")
	case SortAttendedOnAsc:
		buf.WriteString(" ORDER BY attended_on ASC, clocked_at ASC, attendance_id ASC")
	default:
		buf.WriteString(" ORDER BY clocked_at DESC, attendance_id DESC")
	}

	// LIMIT/OFFSET
	limit := q.Limit
	if limit <= 0 {
		limit = DefaultPageLimit
	}
	if limit > MaxPageLimit {
		limit = MaxPageLimit
	}
	offset := q.Offset
	buf.WriteString(fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset))

	// 実行
	rows, err := s.db.QueryContext(ctx, buf.String(), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []Attendance
	for rows.Next() {
		var r attendanceRow
		if err := rows.Scan(&r.AttendanceID, &r.StudentNumber, &r.AttendedOn, &r.ClockedAt, &r.Note); err != nil {
			return nil, 0, err
		}
		out = append(out, r.toModel())
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// COUNT（ORDER BY より前までを再構築）
	var cntBuf bytes.Buffer
	cntBuf.WriteString("SELECT COUNT(*) FROM attendances")
	if len(wheres) > 0 {
		cntBuf.WriteString(" WHERE " + strings.Join(wheres, " AND "))
	}
	var total int64
	if err := s.db.QueryRowContext(ctx, cntBuf.String(), args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// Stats: 期間の出席数をユーザ別合計（TOP N）
func (s *Store) Stats(ctx context.Context, from, to time.Time, limit int) ([]StatsRow, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := s.db.QueryContext(ctx, `
	SELECT student_number, COUNT(*) AS cnt
	FROM attendances
	WHERE attended_on BETWEEN ? AND ?
	GROUP BY student_number
	ORDER BY cnt DESC, student_number ASC
	LIMIT ?`, from.Format(DateLayout), to.Format(DateLayout), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []StatsRow
	for rows.Next() {
		var row StatsRow
		if err := rows.Scan(&row.StudentNumber, &row.Count); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ===== helpers =====

func noteOrNil(s *string) any {
	if s == nil {
		return nil
	}
	if *s == "" {
		return nil
	}
	return *s
}

func normalizeDateString(v string) string {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "today" {
		return time.Now().UTC().Format(DateLayout)
	}
	// assume YYYY-MM-DD
	return v
}

func mustDate(s string) string {
	s = normalizeDateString(s)
	// 軽い検証（失敗時もそのまま返す→DBで弾かせる）
	if _, err := time.ParseInLocation(DateLayout, s, time.UTC); err != nil {
		return s
	}
	return s
}

var cachedLoc *time.Location

func tzLoc() *time.Location {
	// All date/time handling in this package is standardized to UTC.
	if cachedLoc != nil {
		return cachedLoc
	}
	loc, err := time.LoadLocation(DefaultTZ)
	if err != nil {
		cachedLoc = time.UTC
		return cachedLoc
	}
	cachedLoc = loc
	return cachedLoc
}
