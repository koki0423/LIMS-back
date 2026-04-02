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
	r.POST("/returns/key/:lend_key", h.CreateReturnByLendKey)
	// 返却単一取得
	r.GET("/returns/:return_id", h.GetReturn)
	// 返却履歴リスト
	r.GET("/returns", h.ListReturns)
}

// @Summary      Create a lend record
// @Description  Register a new lend for an asset.
// @Tags         lends
// @Accept       json
// @Produce      json
// @Param        lend body CreateLendRequest true "Lend to create"
// @Success      201 {object} LendResponse
// @Failure      400 {object} ErrorResponse "Invalid input"
// @Failure      409 {object} ErrorResponse "Conflict, e.g., already lent or insufficient stock"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /lends [post]
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

// @Summary      Create a return record
// @Description  Register a return for a specific lend record.
// @Tags         returns
// @Accept       json
// @Produce      json
// @Param        return body CreateReturnRequest true "Return to create"
// @Success      201 {object} ReturnResponse
// @Failure      400 {object} ErrorResponse "Invalid input"
// @Failure      404 {object} ErrorResponse "Lend record not found"
// @Failure      409 {object} ErrorResponse "Return quantity exceeds lent quantity"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /returns [post]
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

// @Summary      Create a return record by lend key
// @Description  Register a return using a lend ID or ULID.
// @Tags         returns
// @Accept       json
// @Produce      json
// @Param        lend_key path string true "Lend ID or ULID"
// @Param        return body CreateReturnByKeyRequest true "Return details"
// @Success      201 {object} ReturnResponse
// @Failure      400 {object} ErrorResponse "Invalid input"
// @Failure      404 {object} ErrorResponse "Lend record not found"
// @Failure      409 {object} ErrorResponse "Return quantity exceeds lent quantity"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /returns/key/{lend_key} [post]
func (h *LendHandler) CreateReturnByLendKey(c *gin.Context) {
	lendKey := c.Param("lend_key")
	if lendKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    ErrCodeInvalidArgument,
			"message": "lend_key (id or ulid) is required",
		})
		return
	}

	var req CreateReturnByKeyRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    ErrCodeInvalidArgument,
			"message": err.Error(),
		})
		return
	}

	if req.Quantity <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    ErrCodeInvalidArgument,
			"message": "quantity must be > 0",
		})
		return
	}

	// lend_key から lend を取得（中に lend_id が入っている）
	lendResp, err := h.svc.GetLendByKey(c.Request.Context(), lendKey)
	if err != nil {
		writeError(c, err)
		return
	}

	// 既存の CreateReturnRequest にマッピング（lend_id だけ埋めればOK）
	createReq := CreateReturnRequest{
		LendID:        lendResp.LendID,
		Quantity:      req.Quantity,
		ProcessedByID: req.ProcessedByID,
		Note:          req.Note,
	}

	resp, err := h.svc.CreateReturn(c.Request.Context(), createReq)
	if err != nil {
		writeError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// @Summary      Get a lend record
// @Description  Get details of a lend record by its ID or ULID.
// @Tags         lends
// @Produce      json
// @Param        lend_id path string true "Lend ID or ULID"
// @Success      200 {object} LendResponse
// @Failure      400 {object} ErrorResponse "Invalid input"
// @Failure      404 {object} ErrorResponse "Lend not found"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /lends/{lend_id} [get]
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

// @Summary      List lend records
// @Description  Get a list of lend records with optional filtering.
// @Tags         lends
// @Produce      json
// @Param        borrower_id query string false "Filter by borrower ID"
// @Param        asset_master_id query int false "Filter by asset master ID"
// @Param        management_number query string false "Filter by management number"
// @Param        returned query bool false "Filter by returned status (true/false)"
// @Param        limit query int false "Number of items to return" default(50)
// @Param        offset query int false "Offset for pagination" default(0)
// @Success      200 {array} LendResponse
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /lends [get]
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

// @Summary      Get a return record
// @Description  Get details of a return record by its ID or ULID.
// @Tags         returns
// @Produce      json
// @Param        return_id path string true "Return ID or ULID"
// @Success      200 {object} ReturnResponse
// @Failure      400 {object} ErrorResponse "Invalid input"
// @Failure      404 {object} ErrorResponse "Return not found"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /returns/{return_id} [get]
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

// @Summary      List return records
// @Description  Get a list of return records with optional filtering.
// @Tags         returns
// @Produce      json
// @Param        borrower_id query string false "Filter by borrower ID"
// @Param        asset_master_id query int false "Filter by asset master ID"
// @Param        lend_id query int false "Filter by lend ID"
// @Param        limit query int false "Number of items to return" default(50)
// @Param        offset query int false "Offset for pagination" default(0)
// @Success      200 {array} ReturnResponse
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /returns [get]
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
