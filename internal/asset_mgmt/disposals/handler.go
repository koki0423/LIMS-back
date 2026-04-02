package disposals

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func RegisterRoutes(r gin.IRoutes, svc *Service) {
	h := &Handler{svc: svc}
	// 登録
	r.POST("/assets/:management_number/disposals", h.CreateDisposal) //OK
	// 参照
	r.GET("/disposals", h.ListDisposals)              //OK
	r.GET("/disposals/:disposal_ulid", h.GetDisposal) //OK
}

// @Summary      Create a disposal record
// @Description  Creates a disposal record for an asset by its management number and updates the inventory.
// @Tags         disposals
// @Accept       json
// @Produce      json
// @Param        management_number path string true "Management Number"
// @Param        disposal body CreateDisposalRequest true "Disposal to create"
// @Success      201 {object} DisposalResponse
// @Failure      400 {object} ErrorResponse "Invalid input"
// @Failure      404 {object} ErrorResponse "Asset not found"
// @Failure      409 {object} ErrorResponse "Insufficient stock"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /assets/{management_number}/disposals [post]
func (h *Handler) CreateDisposal(c *gin.Context) {
	mng := c.Param("management_number")
	var req CreateDisposalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorBody(CodeInvalidArgument, "invalid json"))
		return
	}

	log.Printf("CreateDisposal called with management_number: %s, quantity: %d,  reason: %+v, processed_by_id: %+v",
		mng, req.Quantity, req.Reason, req.ProcessedByID)

	res, err := h.svc.CreateDisposal(c.Request.Context(), mng, req)
	if err != nil {
		c.JSON(ToHTTPStatus(err), errorFromErr(err))
		return
	}
	c.Header("Location", "/disposals/"+res.DisposalULID)
	c.JSON(http.StatusCreated, res)
}

// @Summary      Get a disposal record
// @Description  Get details of a disposal record by its ULID.
// @Tags         disposals
// @Produce      json
// @Param        disposal_ulid path string true "Disposal ULID"
// @Success      200 {object} DisposalResponse
// @Failure      404 {object} ErrorResponse "Disposal not found"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /disposals/{disposal_ulid} [get]
func (h *Handler) GetDisposal(c *gin.Context) {
	ul := c.Param("disposal_ulid")
	res, err := h.svc.GetDisposalByULID(c.Request.Context(), ul)
	if err != nil {
		c.JSON(ToHTTPStatus(err), errorFromErr(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

// @Summary      List disposal records
// @Description  Get a paginated list of disposal records with optional filtering.
// @Tags         disposals
// @Produce      json
// @Param        management_number query string false "Filter by management number"
// @Param        processed_by_id   query string false "Filter by processed user ID"
// @Param        from              query string false "Filter by date from (RFC3339)" Format(dateTime)
// @Param        to                query string false "Filter by date to (RFC3339)" Format(dateTime)
// @Param        limit             query int false "Number of items to return" default(50)
// @Param        offset            query int false "Offset for pagination"
// @Param        order             query string false "Sort order ('asc' or 'desc')" Enums(asc, desc) default(desc)
// @Success      200 {object} ListDisposalsResponse
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /disposals [get]
func (h *Handler) ListDisposals(c *gin.Context) {
	f := DisposalFilter{}
	if v := c.Query("management_number"); v != "" {
		f.ManagementNumber = &v
	}
	if v := c.Query("processed_by_id"); v != "" {
		f.ProcessedByID = &v
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = &t
		}
	}
	p := Page{
		Limit:  parseIntDefault(c.Query("limit"), 50),
		Offset: parseIntDefault(c.Query("offset"), 0),
		Order:  c.DefaultQuery("order", "desc"),
	}
	res, err := h.svc.ListDisposals(c.Request.Context(), f, p)
	if err != nil {
		c.JSON(ToHTTPStatus(err), errorFromErr(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

// ---- helpers ----

func parseIntDefault(s string, d int) int {
	if s == "" {
		return d
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return d
	}
	return v
}

type errorDTO struct {
	Error struct {
		Code    Code   `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func errorBody(code Code, msg string) errorDTO {
	var e errorDTO
	e.Error.Code = code
	e.Error.Message = msg
	return e
}
func errorFromErr(err error) errorDTO {
	msg := err.Error()
	if api, ok := err.(*APIError); ok {
		return errorBody(api.Code, api.Message)
	}
	return errorBody(CodeInternal, msg)
}
