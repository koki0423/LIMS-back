package attendance

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func RegisterRoutes(r gin.IRoutes, svc *Service) {

	r.POST("/attendances", handleCreateAttendance(svc))
	r.GET("/attendances", handleListAttendances(svc))
	r.GET("/attendances/stats", handleStats(svc))

	//なぜかHEADがうまく動かないのでv2.0ではコメントアウト
	// g.HEAD("/attendances", handleHeadAttendance(svc))

}

// @Summary      Create or update attendance
// @Description  Upserts an attendance record for a student on a specific date.
// @Tags         attendance
// @Accept       json
// @Produce      json
// @Param        request body CreateAttendanceRequest true "Attendance details"
// @Success      200 {object} AttendanceResponse "Updated successfully"
// @Success      201 {object} AttendanceResponse "Created successfully"
// @Failure      400 {object} ErrorResponse "Invalid input"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /attendances [post]
func handleCreateAttendance(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateAttendanceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			writeErr(c, ErrInvalid("invalid json: "+err.Error()))
			return
		}
		res, created, err := svc.UpsertAttendance(c.Request.Context(), req)
		if err != nil {
			writeErr(c, err)
			return
		}
		if created {
			c.Header("Location", "/attendances/"+strconv.FormatUint(res.AttendanceID, 10))
			c.JSON(http.StatusCreated, res)
			return
		}
		c.JSON(http.StatusOK, res)
	}
}

// @Summary      Check attendance existence
// @Description  Checks if an attendance record exists for a specific user and date.
// @Tags         attendance
// @Param        user_id query string false "User ID (Student Number)"
// @Param        on query string false "Date (YYYY-MM-DD or 'today')"
// @Success      200 "Record exists"
// @Failure      400 {object} ErrorResponse "Invalid input"
// @Failure      404 "Record does not exist"
// @Router       /attendances [head]
func handleHeadAttendance(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := c.Query("user_id")
		on := c.Query("on")
		if on == "" {
			on = "today"
		}
		ok, err := svc.Exists(c.Request.Context(), user, on)
		if err != nil {
			writeErr(c, err)
			return
		}
		if ok {
			c.Status(http.StatusOK)
			return
		}
		c.Status(http.StatusNotFound)
	}
}

// @Summary      List attendances
// @Description  Retrieves a paginated list of attendance records.
// @Tags         attendance
// @Produce      json
// @Param        user_id query string false "Filter by User ID"
// @Param        on query string false "Filter by exact date (YYYY-MM-DD or 'today')"
// @Param        from query string false "Filter by start date (YYYY-MM-DD)"
// @Param        to query string false "Filter by end date (YYYY-MM-DD)"
// @Param        limit query int false "Number of items to return" default(50)
// @Param        offset query int false "Offset for pagination" default(0)
// @Param        sort query string false "Sort order" Enums(clocked_at_desc, clocked_at_asc, attended_on_desc, attended_on_asc) default(clocked_at_desc)
// @Param        tz query string false "Timezone" default(UTC)
// @Success      200 {object} ListAttendanceResponse
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /attendances [get]
func handleListAttendances(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		q := ListQuery{
			Limit:  atoiDefault(c.Query("limit"), DefaultPageLimit),
			Offset: atoiDefault(c.Query("offset"), 0),
			Sort:   strDefault(c.Query("sort"), DefaultSort),
			TZ:     strDefault(c.Query("tz"), DefaultTZ),
		}
		if v := c.Query("user_id"); v != "" {
			q.StudentNumber = &[]string{v}[0]
		}
		if v := c.Query("on"); v != "" {
			q.On = &[]string{v}[0]
		}
		if v := c.Query("from"); v != "" {
			q.From = &[]string{v}[0]
		}
		if v := c.Query("to"); v != "" {
			q.To = &[]string{v}[0]
		}

		rows, total, err := svc.List(c.Request.Context(), q)
		if err != nil {
			writeErr(c, err)
			return
		}
		c.Header("X-Total-Count", strconv.FormatInt(total, 10))
		c.JSON(http.StatusOK, gin.H{
			"items": rows,
			"page": gin.H{
				"limit":  q.Limit,
				"offset": q.Offset,
				"total":  total,
			},
		})
	}
}

// @Summary      Get attendance statistics
// @Description  Retrieves top attendance counts grouped by users within a specified period.
// @Tags         attendance
// @Produce      json
// @Param        from query string true "Start date (YYYY-MM-DD)"
// @Param        to query string true "End date (YYYY-MM-DD)"
// @Param        limit query int false "Number of items to return" default(10)
// @Success      200 {object} map[string]interface{}
// @Failure      400 {object} ErrorResponse "Invalid input"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /attendances/stats [get]
func handleStats(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		req := StatsRequest{
			From:  c.Query("from"),
			To:    c.Query("to"),
			Limit: atoiDefault(c.Query("limit"), 10),
		}
		if req.From == "" || req.To == "" {
			writeErr(c, ErrInvalid("from/to are required (YYYY-MM-DD)"))
			return
		}
		rows, err := svc.Stats(c.Request.Context(), req)
		if err != nil {
			writeErr(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"period": gin.H{"from": req.From, "to": req.To},
			"result": rows,
		})
	}
}

func writeErr(c *gin.Context, err error) {
	status := toHTTPStatus(err)
	switch e := err.(type) {
	case *APIError:
		c.JSON(status, gin.H{"error": e})
	default:
		c.JSON(500, gin.H{"error": APIError{Code: CodeInternal, Message: e.Error()}})
	}
}

func atoiDefault(s string, d int) int {
	if s == "" {
		return d
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return d
	}
	return n
}

func strDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
