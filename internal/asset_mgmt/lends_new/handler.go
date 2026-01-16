package lends_new

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func RegisterRoutes(r gin.IRoutes, svc *Service) {
	h := &Handler{svc: svc}

	// 1. 貸出リソース
	// POST /lends
	r.POST("/lends", h.CreateLend)

	// 旧バージョンからの移行が終わり次第，コメントアウト解除予定
	// GET /lends (一覧・検索)
	// r.GET("/lends", h.ListLends)

	// アプリケーションからの個別貸出取得用エンドポイント
	// 旧バージョンからの移行が終わり次第，コメントアウト解除予定
	// GET /lends/:lend_ulid (ID指定詳細)
	// r.GET("/lends/:lend_ulid", h.GetLendByUlid)

	// 2. 資産・現物起点 (QR Scan)
	// ターミナル端末（スマホ）からのQRスキャンでの貸出情報取得用エンドポイント
	// GET /assets/active-lend/:management_number
	r.GET("/assets/active-lend/:management_number", h.GetActiveLend)

	// 3. 返却リソース (独立)
	// POST /returns
	r.POST("/returns", h.CreateReturn)
	// GET /returns (履歴)
	r.GET("/returns", h.ListReturns)
}

// ---------- handlers ----------

// POST /lends
func (h *Handler) CreateLend(c *gin.Context) {
	var req CreateLendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorBody(CodeInvalidArgument, "invalid json or missing required fields"))
		return
	}

	res, err := h.svc.CreateLend(c.Request.Context(), req)
	if err != nil {
		c.JSON(ToHTTPStatus(err), errorFromErr(err))
		return
	}

	c.Header("Location", "/lends/"+res.LendULID)
	c.JSON(http.StatusCreated, res)
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

func (h *Handler) GetLendByUlid(c *gin.Context) {
	ulid := c.Param("lend_ulid")
	res, err := h.svc.GetLendByULID(c.Request.Context(), ulid)
	if err != nil {
		c.JSON(ToHTTPStatus(err), errorFromErr(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

// GetActiveLend: QRスキャン用ハンドラ
func (h *Handler) GetActiveLend(c *gin.Context) {
	mng := c.Param("management_number")
	res, err := h.svc.GetActiveLend(c.Request.Context(), mng)
	if err != nil {
		c.JSON(ToHTTPStatus(err), errorFromErr(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) CreateReturn(c *gin.Context) {
	var req CreateReturnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorBody(CodeInvalidArgument, "invalid json"))
		return
	}
	// Service層で LendULID を解決して処理
	res, err := h.svc.CreateReturn(c.Request.Context(), req)
	if err != nil {
		c.JSON(ToHTTPStatus(err), errorFromErr(err))
		return
	}
	c.Header("Location", "/returns/"+res.ReturnULID)
	c.JSON(http.StatusCreated, res)
}

func (h *Handler) ListReturns(c *gin.Context) {
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
