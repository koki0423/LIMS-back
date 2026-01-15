package lends

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func RegisterRoutes(r gin.IRoutes, svc *Service) {
	h := &Handler{svc: svc}

	/*既存のエンドポイント．後方互換のため残す．フロントエンドとアプリ側が修正し次第削除*/
	// ここから
	// 貸出（管理番号単位）
	r.POST("/assets/:management_number/lends", h.CreateLend) //OK

	// 貸出リソース
	r.GET("/lends", h.ListLends)                                      //複数レスポンスあり
	r.GET("/lends/:lend_ulid", h.GetLendByUlid)                       //特定の貸出取得．１件のみ応答

	// 返却
	r.POST("/lends/:lend_ulid/returns", h.CreateReturn)     //OK
	r.GET("/lends/:lend_ulid/returns", h.ListReturnsByLend) //要修正
	// ここまで
}

// ---------- handlers ----------

func (h *Handler) CreateLend(c *gin.Context) {
	mng := c.Param("management_number")
	var req CreateLendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorBody(CodeInvalidArgument, "invalid json"))
		return
	}
	res, err := h.svc.CreateLend(c.Request.Context(), mng, req)
	if err != nil {
		c.JSON(ToHTTPStatus(err), errorFromErr(err))
		return
	}
	c.Header("Location", "/lends/"+res.LendULID)
	c.JSON(http.StatusCreated, res)
}

func (h *Handler) GetLendByUlid(c *gin.Context) {
	parm := c.Param("lend_ulid")
	res, err := h.svc.GetLendByULID(c.Request.Context(), parm)
	if err != nil {
		c.JSON(ToHTTPStatus(err), errorFromErr(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) ListLends(c *gin.Context) {
	f := LendFilter{}
	if v := c.Query("management_number"); v != "" {
		f.ManagementNumber = &v
	}
	if v := c.Query("returned"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			f.Returned = &b
		}
	}
	if v := c.Query("borrower_id"); v != "" {
		f.BorrowerID = &v
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
	if v := c.Query("only_outstanding"); v == "true" || v == "1" {
		f.OnlyOutstanding = true
	}
	p := Page{
		Limit:  parseIntDefault(c.Query("limit"), 50),
		Offset: parseIntDefault(c.Query("offset"), 0),
		Order:  c.DefaultQuery("order", "desc"),
	}
	res, err := h.svc.ListLends(c.Request.Context(), f, p)
	if err != nil {
		c.JSON(ToHTTPStatus(err), errorFromErr(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) CreateReturn(c *gin.Context) {
	luid := c.Param("lend_ulid")
	var req CreateReturnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorBody(CodeInvalidArgument, "invalid json"))
		return
	}
	res, err := h.svc.CreateReturn(c.Request.Context(), luid, req)
	if err != nil {
		c.JSON(ToHTTPStatus(err), errorFromErr(err))
		return
	}
	c.Header("Location", "/returns/"+res.ReturnULID)
	c.JSON(http.StatusCreated, res)
}

func (h *Handler) ListReturnsByLend(c *gin.Context) {
	luid := c.Param("lend_ulid")
	p := Page{
		Limit:  parseIntDefault(c.Query("limit"), 50),
		Offset: parseIntDefault(c.Query("offset"), 0),
		Order:  c.DefaultQuery("order", "desc"),
	}
	res, err := h.svc.ListReturnsByLend(c.Request.Context(), luid, p)
	if err != nil {
		c.JSON(ToHTTPStatus(err), errorFromErr(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

// ---------- helpers ----------

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
	var msg string
	var code Code = CodeInternal
	if api, ok := err.(*APIError); ok {
		code, msg = api.Code, api.Message
	} else {
		msg = err.Error()
	}
	return errorBody(code, msg)
}
