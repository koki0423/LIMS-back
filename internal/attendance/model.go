package attendance

import "time"

// DB行に対応（スキャン用）
type attendanceRow struct {
	AttendanceID  uint64
	StudentNumber string
	AttendedOn    string    // DATE → "YYYY-MM-DD"
	ClockedAt     time.Time
	Note          *string
}

// Service ↔ Store で使うモデル（必要最小限）
type Attendance struct {
	AttendanceID  uint64
	StudentNumber string
	AttendedOn    string
	ClockedAt     time.Time
	Note          *string
}

func (r attendanceRow) toModel() Attendance {
	return Attendance{
		AttendanceID:  r.AttendanceID,
		StudentNumber: r.StudentNumber,
		AttendedOn:    r.AttendedOn,
		ClockedAt:     r.ClockedAt.UTC(),
		Note:          r.Note,
	}
}

func (a Attendance) toDTO() AttendanceResponse {
	return AttendanceResponse{
		AttendanceID:  a.AttendanceID,
		StudentNumber: a.StudentNumber,
		AttendedOn:    a.AttendedOn,
		ClockedAt:     a.ClockedAt,
		Note:          a.Note,
	}
}
