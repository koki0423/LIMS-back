package assets

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func RegisterRoutes(r gin.IRoutes, svc *Service) {
	h := &Handler{svc: svc}

	// masters
	r.POST("/assets/masters", h.CreateAssetMaster)
	r.GET("/assets/masters", h.ListAssetMasters)
	r.GET("/assets/masters/:management_number", h.GetAssetMaster)
	r.PUT("/assets/masters/:management_number", h.UpdateAssetMaster)

	// assets
	r.POST("/assets", h.CreateAsset)
	r.GET("/assets", h.ListAssets)
	r.GET("/assets/:asset_id", h.GetAsset)
	r.PUT("/assets/:asset_id", h.UpdateAsset)

	// assest-set
	r.GET("/assets/pair/:management_number", h.GetAssetSet)

}

// ===== masters =====

func (h *Handler) CreateAssetMaster(c *gin.Context) {
	var req CreateAssetMasterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid json"))
		return
	}
	res, err := h.svc.CreateAssetMaster(c.Request.Context(), req)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.Header("Location", "/assets/masters/"+res.ManagementNumber)
	c.JSON(http.StatusCreated, res)
}

func (h *Handler) GetAssetMaster(c *gin.Context) {
	mng := c.Param("management_number")
	res, err := h.svc.GetAssetMaster(c.Request.Context(), mng)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) ListAssetMasters(c *gin.Context) {
	var q AssetSearchQuery
	if v := c.Query("genre"); v != "" {
		id, err := strconv.Atoi(v)
		if err == nil {
			u := uint(id)
			q.GenreID = &u
		}
	}
	p := Page{
		Limit:  atoiDef(c.Query("limit"), 50),
		Offset: atoiDef(c.Query("offset"), 0),
		Order:  strings.ToLower(c.DefaultQuery("order", "desc")),
	}
	items, total, err := h.svc.ListAssetMasters(c.Request.Context(), p, q)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "next_offset": nextOffset(total, p)})
}

func (h *Handler) UpdateAssetMaster(c *gin.Context) {
	mng := c.Param("management_number")
	var req UpdateAssetMasterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid json"))
		return
	}
	res, err := h.svc.UpdateAssetMaster(c.Request.Context(), mng, req)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

// ===== assets =====

func (h *Handler) CreateAsset(c *gin.Context) {
	var req CreateAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("CreateAsset: bind error: %v", err)
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid json"))
		return
	}
	res, err := h.svc.CreateAsset(c.Request.Context(), req)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.Header("Location", "/assets/"+strconv.FormatUint(res.AssetID, 10))
	c.JSON(http.StatusCreated, res)
}

func (h *Handler) GetAsset(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("asset_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "asset_id must be a number"))
		return
	}
	res, err := h.svc.GetAsset(c.Request.Context(), id)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

func (h *Handler) ListAssets(c *gin.Context) {
	var q AssetSearchQuery
	if v := c.Query("management_number"); v != "" {
		q.ManagementNumber = &v
	}
	if v:= c.Query("asmi"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			q.AssetMasterID = &n
		}
	}
	if v := c.Query("status_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			u := uint(n)
			q.StatusID = &u
		}
	}
	if v := c.Query("owner"); v != "" {
		q.Owner = &v
	}
	if v := c.Query("location"); v != "" {
		q.Location = &v
	}
	if v := c.Query("purchased_from"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			q.PurchasedFrom = &t
		}
	}
	if v := c.Query("purchased_to"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			q.PurchasedTo = &t
		}
	}

	p := Page{
		Limit:  atoiDef(c.Query("limit"), 50),
		Offset: atoiDef(c.Query("offset"), 0),
		Order:  strings.ToLower(c.DefaultQuery("order", "desc")),
	}
	items, total, err := h.svc.ListAssets(c.Request.Context(), q, p)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total, "next_offset": nextOffset(total, p)})
}

func (h *Handler) UpdateAsset(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("asset_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid asset_id"))
		return
	}
	var req UpdateAssetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid json"))
		return
	}
	res, err := h.svc.UpdateAsset(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

// ===== asset-set =====
// 将来的にcreateAssetMasterとCreateAssetを廃止してこっちへ移行．ただしAndroidとフロントエンドの対応が終わり次第移行すること．
func (h *Handler) RegisterAsset(c *gin.Context) {
	var req CreateAssetSetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Printf("CreateAssetSet: bind error: %v", err)
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid json"))
		return
	}
	res, err := h.svc.CreateAssetSet(c.Request.Context(), req)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *Handler) GetAssetSet(c *gin.Context) {
	mng := c.Param("management_number")
	res, err := h.svc.GetAssetSet(c.Request.Context(), mng)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, res)
}

// ===== helpers =====

func atoiDef(s string, d int) int {
	if s == "" {
		return d
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return d
	}
	return n
}
func nextOffset(total int64, p Page) int {
	n := p.Offset + p.Limit
	if n >= int(total) {
		return 0
	}
	return n
}

type errDTO struct {
	Error struct {
		Code    Code   `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func apiErr(code Code, msg string) errDTO {
	var e errDTO
	e.Error.Code = code
	e.Error.Message = msg
	return e
}
func apiErrFrom(err error) errDTO {
	if api, ok := err.(*APIError); ok {
		return apiErr(api.Code, api.Message)
	}
	return apiErr(CodeInternal, err.Error())
}
