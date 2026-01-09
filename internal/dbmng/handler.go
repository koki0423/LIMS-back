// handler.go
package dbmng

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handler struct{ svc *Service }

func RegisterRoutes(r gin.IRoutes, svc *Service) {
	h := &Handler{svc: svc}
	r.POST("/genres", h.CreateGenre)
	r.GET("/genres", h.ListGenres)
	r.GET("/genres/:id", h.GetGenre)
	r.PUT("/genres/:id", h.UpdateGenre)
	r.DELETE("/genres/:id", h.DeleteGenre)
}

func (h *Handler) ListGenres(c *gin.Context) {
	resp, err := h.svc.ListGenres(c.Request.Context(), c.Query("all"))
	if err != nil {
		c.JSON(toHTTPStatus(err), err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) GetGenre(c *gin.Context) {
	idU64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || idU64 == 0 {
		c.JSON(http.StatusBadRequest, ErrInvalid("invalid id"))
		return
	}
	resp, err := h.svc.GetGenre(c.Request.Context(), uint(idU64))
	if err != nil {
		c.JSON(toHTTPStatus(err), err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) CreateGenre(c *gin.Context) {
	var req CreateGenreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrInvalid(err.Error()))
		return
	}
	resp, err := h.svc.CreateGenre(c.Request.Context(), req.GenreName, req.GenreCode)
	if err != nil {
		c.JSON(toHTTPStatus(err), err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) UpdateGenre(c *gin.Context) {
	idU64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || idU64 == 0 {
		c.JSON(http.StatusBadRequest, ErrInvalid("invalid id"))
		return
	}
	var req UpdateGenreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrInvalid(err.Error()))
		return
	}
	resp, err := h.svc.UpdateGenre(c.Request.Context(), uint(idU64), req.GenreName, req.GenreCode, req.IsDisabled)
	if err != nil {
		c.JSON(toHTTPStatus(err), err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) DeleteGenre(c *gin.Context) {
	idU64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || idU64 == 0 {
		c.JSON(http.StatusBadRequest, ErrInvalid("invalid id"))
		return
	}
	err = h.svc.DeleteGenre(c.Request.Context(), uint(idU64))
	if err != nil {
		c.JSON(toHTTPStatus(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}
