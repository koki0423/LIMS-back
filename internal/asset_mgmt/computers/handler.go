package computers

import (
	"net/http"
	"strconv"

	"log"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc *Service
}

func RegisterRoutes(r gin.IRoutes, svc *Service) {
	h := &Handler{svc: svc}

	r.POST("/computer-details", h.CreateComputerDetail)
	r.GET("/computer-details/:asset_master_id", h.GetComputerDetail)
	r.PUT("/computer-details/:asset_master_id", h.UpdateComputerDetail)

	r.POST("/computer-parts", h.CreateComputerPart)
	r.GET("/computer-parts/:asset_master_id", h.GetComputerPart)
	r.PUT("/computer-parts/:asset_master_id", h.UpdateComputerPart)

	r.POST("/computer-configurations", h.CreateComputerConfiguration)
	r.PUT("/computer-configurations/:computer_configuration_id", h.UpdateComputerConfiguration)
	r.GET("/computers/:computer_asset_master_id/configurations", h.ListComputerConfigurations)

	r.GET("/part-types", h.ListPartTypes)
	r.GET("/usage-statuses", h.ListUsageStatuses)
}

// @Summary      Create a computer detail
// @Description  Creates a detail record linked to an existing asset master.
// @Tags         computers-details
// @Accept       json
// @Produce      json
// @Param        computerDetail body CreateComputerDetailRequest true "Computer detail"
// @Success      201 {object} ComputerDetailResponse
// @Failure      400 {object} ErrorResponse
// @Failure      409 {object} ErrorResponse
// @Failure      500 {object} ErrorResponse
// @Router       /computer-details [post]
func (h *Handler) CreateComputerDetail(c *gin.Context) {
	var req CreateComputerDetailRequest
	log.Println("request body:", c.Request.Body)
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid json"))
		return
	}

	out, err := h.svc.CreateComputerDetail(c.Request.Context(), req)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.Header("Location", "/computer-details/"+strconv.FormatUint(out.AssetMasterID, 10))
	c.JSON(http.StatusCreated, out)
}

// @Summary      Get a computer detail
// @Description  Retrieves a detail record by asset master ID.
// @Tags         computers-details
// @Produce      json
// @Param        asset_master_id path int true "Asset master ID"
// @Success      200 {object} ComputerDetailResponse
// @Failure      400 {object} ErrorResponse
// @Failure      404 {object} ErrorResponse
// @Failure      500 {object} ErrorResponse
// @Router       /computer-details/{asset_master_id} [get]
func (h *Handler) GetComputerDetail(c *gin.Context) {
	assetMasterID, ok := parseUint64Path(c, "asset_master_id")
	if !ok {
		return
	}

	out, err := h.svc.GetComputerDetail(c.Request.Context(), assetMasterID)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, out)
}

// @Summary      Update a computer detail
// @Description  Updates fields on a detail record identified by asset master ID.
// @Tags         computers-details
// @Accept       json
// @Produce      json
// @Param        asset_master_id path int true "Asset master ID"
// @Param        computerDetail body UpdateComputerDetailRequest true "Computer detail patch"
// @Success      200 {object} ComputerDetailResponse
// @Failure      400 {object} ErrorResponse
// @Failure      404 {object} ErrorResponse
// @Failure      500 {object} ErrorResponse
// @Router       /computer-details/{asset_master_id} [put]
func (h *Handler) UpdateComputerDetail(c *gin.Context) {
	assetMasterID, ok := parseUint64Path(c, "asset_master_id")
	if !ok {
		return
	}

	var req UpdateComputerDetailRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid json"))
		return
	}

	out, err := h.svc.UpdateComputerDetail(c.Request.Context(), assetMasterID, req)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, out)
}

// @Summary      Create a computer part
// @Description  Creates a part record linked to an existing asset master.
// @Tags         computers-parts
// @Accept       json
// @Produce      json
// @Param        computerPart body CreateComputerPartRequest true "Computer part"
// @Success      201 {object} ComputerPartResponse
// @Failure      400 {object} ErrorResponse
// @Failure      409 {object} ErrorResponse
// @Failure      500 {object} ErrorResponse
// @Router       /computer-parts [post]
func (h *Handler) CreateComputerPart(c *gin.Context) {
	var req CreateComputerPartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid json"))
		return
	}

	out, err := h.svc.CreateComputerPart(c.Request.Context(), req)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.Header("Location", "/computer-parts/"+strconv.FormatUint(out.AssetMasterID, 10))
	c.JSON(http.StatusCreated, out)
}

// @Summary      Get a computer part
// @Description  Retrieves a part record by asset master ID.
// @Tags         computers-parts
// @Produce      json
// @Param        asset_master_id path int true "Asset master ID"
// @Success      200 {object} ComputerPartResponse
// @Failure      400 {object} ErrorResponse
// @Failure      404 {object} ErrorResponse
// @Failure      500 {object} ErrorResponse
// @Router       /computer-parts/{asset_master_id} [get]
func (h *Handler) GetComputerPart(c *gin.Context) {
	assetMasterID, ok := parseUint64Path(c, "asset_master_id")
	if !ok {
		return
	}

	out, err := h.svc.GetComputerPart(c.Request.Context(), assetMasterID)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, out)
}

// @Summary      Update a computer part
// @Description  Updates fields on a part record identified by asset master ID.
// @Tags         computers-parts
// @Accept       json
// @Produce      json
// @Param        asset_master_id path int true "Asset master ID"
// @Param        computerPart body UpdateComputerPartRequest true "Computer part patch"
// @Success      200 {object} ComputerPartResponse
// @Failure      400 {object} ErrorResponse
// @Failure      404 {object} ErrorResponse
// @Failure      500 {object} ErrorResponse
// @Router       /computer-parts/{asset_master_id} [put]
func (h *Handler) UpdateComputerPart(c *gin.Context) {
	assetMasterID, ok := parseUint64Path(c, "asset_master_id")
	if !ok {
		return
	}

	var req UpdateComputerPartRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid json"))
		return
	}

	out, err := h.svc.UpdateComputerPart(c.Request.Context(), assetMasterID, req)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, out)
}

// @Summary      Create a computer configuration
// @Description  Registers a computer-part configuration history record. `installed_at` and `removed_at` use `YYYY-MM-DD`.
// @Tags         computers-configurations
// @Accept       json
// @Produce      json
// @Param        computerConfiguration body CreateComputerConfigurationRequest true "Computer configuration"
// @Success      201 {object} ComputerConfigurationResponse
// @Failure      400 {object} ErrorResponse
// @Failure      409 {object} ErrorResponse
// @Failure      500 {object} ErrorResponse
// @Router       /computer-configurations [post]
func (h *Handler) CreateComputerConfiguration(c *gin.Context) {
	var req CreateComputerConfigurationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid json"))
		return
	}

	out, err := h.svc.CreateComputerConfiguration(c.Request.Context(), req)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.Header("Location", "/computers/"+strconv.FormatUint(out.ComputerAssetMasterID, 10)+"/configurations")
	c.JSON(http.StatusCreated, out)
}

// @Summary      List computer configurations
// @Description  Lists all configuration history rows for a computer asset master.
// @Tags         computers-configurations
// @Produce      json
// @Param        computer_asset_master_id path int true "Computer asset master ID"
// @Success      200 {array} ComputerConfigurationResponse
// @Failure      400 {object} ErrorResponse
// @Failure      404 {object} ErrorResponse
// @Failure      500 {object} ErrorResponse
// @Router       /computers/{computer_asset_master_id}/configurations [get]
func (h *Handler) ListComputerConfigurations(c *gin.Context) {
	computerAssetMasterID, ok := parseUint64Path(c, "computer_asset_master_id")
	if !ok {
		return
	}

	out, err := h.svc.ListComputerConfigurations(c.Request.Context(), computerAssetMasterID)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, out)
}

// @Summary      Update a computer configuration
// @Description  Updates an existing configuration row. `installed_at` and `removed_at` use `YYYY-MM-DD`; an empty string clears the value.
// @Tags         computers-configurations
// @Accept       json
// @Produce      json
// @Param        computer_configuration_id path int true "Computer configuration ID"
// @Param        computerConfiguration body UpdateComputerConfigurationRequest true "Computer configuration patch"
// @Success      200 {object} ComputerConfigurationResponse
// @Failure      400 {object} ErrorResponse
// @Failure      404 {object} ErrorResponse
// @Failure      409 {object} ErrorResponse
// @Failure      500 {object} ErrorResponse
// @Router       /computer-configurations/{computer_configuration_id} [put]
func (h *Handler) UpdateComputerConfiguration(c *gin.Context) {
	computerConfigurationID, ok := parseUint64Path(c, "computer_configuration_id")
	if !ok {
		return
	}

	var req UpdateComputerConfigurationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, "invalid json"))
		return
	}

	out, err := h.svc.UpdateComputerConfiguration(c.Request.Context(), computerConfigurationID, req)
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, out)
}

// @Summary      List part types
// @Description  Lists master data for computer part types.
// @Tags         computers-masters
// @Produce      json
// @Success      200 {array} PartTypeResponse
// @Failure      500 {object} ErrorResponse
// @Router       /part-types [get]
func (h *Handler) ListPartTypes(c *gin.Context) {
	out, err := h.svc.ListPartTypes(c.Request.Context())
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, out)
}

// @Summary      List usage statuses
// @Description  Lists master data for computer part usage statuses.
// @Tags         computers-masters
// @Produce      json
// @Success      200 {array} UsageStatusResponse
// @Failure      500 {object} ErrorResponse
// @Router       /usage-statuses [get]
func (h *Handler) ListUsageStatuses(c *gin.Context) {
	out, err := h.svc.ListUsageStatuses(c.Request.Context())
	if err != nil {
		c.JSON(toHTTPStatus(err), apiErrFrom(err))
		return
	}
	c.JSON(http.StatusOK, out)
}

func parseUint64Path(c *gin.Context, key string) (uint64, bool) {
	value, err := strconv.ParseUint(c.Param(key), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiErr(CodeInvalidArgument, key+" must be uint64"))
		return 0, false
	}
	return value, true
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
