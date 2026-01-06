package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct{ svc AuthService }

func RegisterRoutes(r gin.IRoutes, svc AuthService) {
	h := &AuthHandler{svc: svc}
	r.POST("/login", h.Login)       // 既存 :contentReference[oaicite:4]{index=4}
	r.POST("/register", h.Register) // 追加
	r.DELETE("/accounts/:id", h.DeleteAccount)
	r.PATCH("/accounts/:id", h.ChangeUsername) // “ユーザー名変更” = id変更
}

type LoginRequest struct {
	ID       string `json:"id" binding:"required"`
	Password string `json:"password" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	token, err := h.svc.Login(c.Request.Context(), req.ID, req.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "IDまたはパスワードが間違っています"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"message": "Login successful",
	})
}

type RegisterRequest struct {
	ID       string  `json:"id" binding:"required"`
	Password string  `json:"password" binding:"required"`
	Role     *string `json:"role,omitempty"` // 未指定なら user
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	role := "user"
	if req.Role != nil && *req.Role != "" {
		role = *req.Role
	}

	if err := h.svc.Register(c.Request.Context(), req.ID, req.Password, role); err != nil {
		if err == ErrAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{"error": "ID already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "register failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "registered"})
}

func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	id := c.Param("id")

	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		if err == ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "delete failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

type ChangeUsernameRequest struct {
	NewID string `json:"new_id" binding:"required"`
}

func (h *AuthHandler) ChangeUsername(c *gin.Context) {
	oldID := c.Param("id")

	var req ChangeUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if err := h.svc.ChangeID(c.Request.Context(), oldID, req.NewID); err != nil {
		if err == ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if err == ErrAlreadyExists {
			c.JSON(http.StatusConflict, gin.H{"error": "new id already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "change id failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "username changed"})
}
