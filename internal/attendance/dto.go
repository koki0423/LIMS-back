package attendance

import "time"

const (
	SortClockedAtDesc  = "clocked_at_desc"
	SortClockedAtAsc   = "clocked_at_asc"
	SortAttendedOnDesc = "attended_on_desc"
	SortAttendedOnAsc  = "attended_on_asc"
	DefaultPageLimit   = 50
	MaxPageLimit       = 200
	DefaultSort        = SortClockedAtDesc
	DefaultTZ          = "UTC"
	DateLayout         = "2006-01-02"
)

type CreateAttendanceRequest struct {
	StudentNumber string  `json:"user_id" binding:"required"`
	AttendedOn    *string `json:"attended_on,omitempty"` // "YYYY-MM-DD" or "today"
	Note          *string `json:"note,omitempty"`
}

type AttendanceResponse struct {
	AttendanceID  uint64    `json:"attendance_id"`
	StudentNumber string    `json:"user_id"`
	AttendedOn    string    `json:"attended_on"` // YYYY-MM-DD
	ClockedAt     time.Time `json:"clocked_at"`
	Note          *string   `json:"note,omitempty"`
}

type ListQuery struct {
	StudentNumber *string
	On            *string
	From          *string
	To            *string
	Limit         int
	Offset        int
	Sort          string
	ExistsOnly    bool
	TZ            string
}

type StatsRequest struct {
	From  string // YYYY-MM-DD
	To    string // YYYY-MM-DD
	Limit int
}

type StatsRow struct {
	StudentNumber string `json:"user_id"`
	Count         int64  `json:"count"`
}
