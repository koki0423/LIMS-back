package printLabels

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func RegisterRoutes(r gin.IRoutes, svc *Service) {
	h := &Handler{svc: svc}
	r.POST("/assets/print", h.PrintLabels)
	r.POST("/assets/print/batch", h.HandlePrintBatch)
	r.GET("/assets/print/templates/download", h.DownloadTemplate)
}

// @Summary      Print a single label
// @Description  Print a single label using the specified configuration and data.
// @Tags         print
// @Accept       json
// @Produce      json
// @Param        request body PrintRequest true "Print request details"
// @Success      201 {object} PrintResponse
// @Failure      400 {object} ErrorResponse "Invalid input"
// @Failure      404 {object} ErrorResponse "Template not found"
// @Failure      409 {object} ErrorResponse "Tape size not matched"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /assets/print [post]
func (h *Handler) PrintLabels(c *gin.Context) {
	var req PrintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, newErrDTO(ErrInvalid("invalid json")))
		return
	}

	res, err := h.svc.PrintLabels(c.Request.Context(), req)
	if err != nil {
		c.JSON(toHTTPStatus(err), newErrDTO(err))
		return
	}
	c.JSON(http.StatusCreated, res)
}

// @Summary      Print multiple labels
// @Description  Print multiple labels in a batch using the specified configuration and data.
// @Tags         print
// @Accept       json
// @Produce      json
// @Param        request body BatchPrintRequest true "Batch print request details"
// @Success      201 {object} PrintResponse
// @Failure      400 {object} ErrorResponse "Invalid input"
// @Failure      404 {object} ErrorResponse "Template not found"
// @Failure      409 {object} ErrorResponse "Tape size not matched"
// @Failure      500 {object} ErrorResponse "Internal server error"
// @Router       /assets/print/batch [post]
func (h *Handler) HandlePrintBatch(c *gin.Context) {
	var req BatchPrintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "JSONのデータ構造が正しくありません",
			"details": err.Error(),
		})
		return
	}

	res, err := h.svc.PrintLabelsBatch(c.Request.Context(), req)
	log.Printf("Batch print request processed: %+v, %v", req, err)
	if err != nil {
		c.JSON(toHTTPStatus(err), newErrDTO(err))
		return
	}
	c.JSON(http.StatusCreated, res)
}

func (h *Handler) DownloadTemplate(c *gin.Context) {
	var q TemplateDownloadQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		c.JSON(http.StatusBadRequest, newErrDTO(ErrInvalid("invalid query")))
		return
	}

	fullpath, filename, err := h.svc.ResolveTemplatePath(c.Request.Context(), q.Width, q.Type)
	if err != nil {
		c.JSON(toHTTPStatus(err), newErrDTO(err))
		return
	}

	c.FileAttachment(fullpath, filename)
}

// ===== helpers =====
type errDTO struct {
	Error *APIError `json:"error"`
}

func newErrDTO(err error) errDTO {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return errDTO{Error: apiErr}
	}

	return errDTO{Error: ErrInternal(err.Error())}
}
