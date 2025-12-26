package printLabels

import (
	"errors"
	"net/http"
	"log"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func RegisterRoutes(r gin.IRoutes, svc *Service) {
	h := &Handler{svc: svc}
	r.POST("/assets/print", h.PrintLabels)
	r.POST("/assets/print/batch", h.HandlePrintBatch)
}

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

// /print/batch: 複数ラベルの流し込み印刷
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
