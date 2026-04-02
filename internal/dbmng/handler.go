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

// @Summary      List genres
// @Description  Get a list of asset genres. Set 'all=1' to include disabled genres.
// @Tags         genres
// @Produce      json
// @Param        all query string false "Include disabled genres if '1', 'true', 'yes', or 'all'"
// @Success      200 {array} AssetGenre
// @Failure      500 {object} APIError "Internal server error"
// @Router       /genres [get]
func (h *Handler) ListGenres(c *gin.Context) {
	resp, err := h.svc.ListGenres(c.Request.Context(), c.Query("all"))
	if err != nil {
		c.JSON(toHTTPStatus(err), err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary      Get a genre
// @Description  Get details of an asset genre by its ID.
// @Tags         genres
// @Produce      json
// @Param        id path int true "Genre ID"
// @Success      200 {object} AssetGenre
// @Failure      400 {object} APIError "Invalid ID"
// @Failure      404 {object} APIError "Genre not found"
// @Failure      500 {object} APIError "Internal server error"
// @Router       /genres/{id} [get]
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

// @Summary      Create a new genre
// @Description  Creates a new asset genre.
// @Tags         genres
// @Accept       json
// @Produce      json
// @Param        request body CreateGenreRequest true "Genre details"
// @Success      201 {object} AssetGenre
// @Failure      400 {object} APIError "Invalid input"
// @Failure      409 {object} APIError "Conflict, e.g., genre code already exists"
// @Failure      500 {object} APIError "Internal server error"
// @Router       /genres [post]
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

// @Summary      Update a genre
// @Description  Updates an existing asset genre by its ID.
// @Tags         genres
// @Accept       json
// @Produce      json
// @Param        id path int true "Genre ID"
// @Param        request body UpdateGenreRequest true "Genre update details"
// @Success      200 {object} AssetGenre
// @Failure      400 {object} APIError "Invalid input or ID"
// @Failure      404 {object} APIError "Genre not found"
// @Failure      409 {object} APIError "Conflict, e.g., genre code already exists"
// @Failure      500 {object} APIError "Internal server error"
// @Router       /genres/{id} [put]
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

// @Summary      Delete a genre
// @Description  Soft deletes (disables) an asset genre by its ID.
// @Tags         genres
// @Param        id path int true "Genre ID"
// @Success      204 "No Content"
// @Failure      400 {object} APIError "Invalid ID"
// @Failure      404 {object} APIError "Genre not found"
// @Failure      500 {object} APIError "Internal server error"
// @Router       /genres/{id} [delete]
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
