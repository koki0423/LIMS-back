package lends_new

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type LendHandler struct {
	svc *Service
}

func RegisterRoutes(r gin.IRoutes, svc *Service) {
	h := &LendHandler{svc: svc}
	// 貸出登録
	r.POST("/lends", h.CreateLend)
	// 貸出単一取得
	r.GET("/lends/:lend_id", h.GetLend)
	// 貸出履歴リスト
	r.GET("/lends", h.ListLends)
	// 返却登録
	r.POST("/returns", h.CreateReturn)
	// 返却単一取得
	r.GET("/returns/:return_id", h.GetReturn)
	// 返却履歴リスト
	r.GET("/returns", h.ListReturns)
}

// POST /api/v2/lends
func (h *LendHandler) CreateLend(c *gin.Context) {
	var req CreateLendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    ErrCodeInvalidArgument,
			"message": err.Error(),
		})
		return
	}

	resp, err := h.svc.CreateLend(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// POST /api/v2/returns
func (h *LendHandler) CreateReturn(c *gin.Context) {
	var req CreateReturnRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    ErrCodeInvalidArgument,
			"message": err.Error(),
		})
		return
	}

	resp, err := h.svc.CreateReturn(c.Request.Context(), req)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

// GET /api/v2/lends/:lend_id
func (h *LendHandler) GetLend(c *gin.Context) {
	key := c.Param("lend_id")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    ErrCodeInvalidArgument,
			"message": "lend_id (id or ulid) is required",
		})
		return
	}

	resp, err := h.svc.GetLendByKey(c.Request.Context(), key)
	if err != nil {
		writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GET /api/v2/lends?borrower_id=&asset_master_id=&management_number=&returned=
func (h *LendHandler) ListLends(c *gin.Context) {
	filter := LendFilter{
		BorrowerID:       c.Query("borrower_id"),
		ManagementNumber: c.Query("management_number"),
	}

	assetMasterIDStr := c.Query("asset_master_id")
	if assetMasterIDStr != "" {
		id, err := strconv.ParseInt(assetMasterIDStr, 10, 64)
		if err == nil && id > 0 {
			filter.AssetMasterID = &id
		}
	}

	returnedStr := c.Query("returned")
	if returnedStr != "" {
		if returnedStr == "true" || returnedStr == "1" {
			val := true
			filter.Returned = &val
		} else if returnedStr == "false" || returnedStr == "0" {
			val := false
			filter.Returned = &val
		}
	}

	limitStr := c.Query("limit")
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			filter.Limit = v
		}
	}
	offsetStr := c.Query("offset")
	if offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			filter.Offset = v
		}
	}

	resp, err := h.svc.ListLends(c.Request.Context(), filter)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GET /api/v2/returns/:return_id
func (h *LendHandler) GetReturn(c *gin.Context) {
	key := c.Param("return_id")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    ErrCodeInvalidArgument,
			"message": "return_id (id or ulid) is required",
		})
		return
	}

	resp, err := h.svc.GetReturnByKey(c.Request.Context(), key)
	if err != nil {
		writeError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GET /api/v2/returns?borrower_id=&asset_master_id=&lend_id=
func (h *LendHandler) ListReturns(c *gin.Context) {
	filter := ReturnFilter{
		BorrowerID: c.Query("borrower_id"),
	}

	assetMasterIDStr := c.Query("asset_master_id")
	if assetMasterIDStr != "" {
		id, err := strconv.ParseInt(assetMasterIDStr, 10, 64)
		if err == nil && id > 0 {
			filter.AssetMasterID = &id
		}
	}

	lendIDStr := c.Query("lend_id")
	if lendIDStr != "" {
		id, err := strconv.ParseInt(lendIDStr, 10, 64)
		if err == nil && id > 0 {
			filter.LendID = &id
		}
	}

	limitStr := c.Query("limit")
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			filter.Limit = v
		}
	}
	offsetStr := c.Query("offset")
	if offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil && v >= 0 {
			filter.Offset = v
		}
	}

	resp, err := h.svc.ListReturns(c.Request.Context(), filter)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// エラーハンドリング共通化
func writeError(c *gin.Context, err error) {
	var dErr *DomainError
	if errors.As(err, &dErr) {
		status := http.StatusInternalServerError
		switch dErr.Code {
		case ErrCodeNotFound:
			status = http.StatusNotFound
		case ErrCodeInvalidArgument:
			status = http.StatusBadRequest
		case ErrCodeConflict, ErrCodeQuantityOverReturn:
			status = http.StatusConflict
		default:
			status = http.StatusInternalServerError
		}
		c.JSON(status, gin.H{
			"code":    dErr.Code,
			"message": dErr.Message,
		})
		return
	}

	// 想定外エラー
	c.JSON(http.StatusInternalServerError, gin.H{
		"code":    ErrCodeInternal,
		"message": err.Error(),
	})
}
