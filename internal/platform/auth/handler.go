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

// ===== API Responses for Swagger =====

// TokenResponse represents a successful login response with a JWT token.
type TokenResponse struct {
	Token   string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5c..."`
	Message string `json:"message" example:"Login successful"`
}

// MessageResponse represents a generic success message.
type MessageResponse struct {
	Message string `json:"message" example:"success"`
}

// ErrorResponse represents an error response.
type ErrorResponse struct {
	Error string `json:"error" example:"error message"`
}

type LoginRequest struct {
	ID       string `json:"id" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// @Summary      Login user
// @Description  Authenticates a user and returns a JWT token.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body LoginRequest true "Login credentials"
// @Success      200 {object} TokenResponse "Login successful"
// @Failure      400 {object} ErrorResponse "Invalid request"
// @Failure      401 {object} ErrorResponse "IDまたはパスワードが間違っています"
// @Router       /login [post]
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

/*
テスト用ユーザー
{
    "id":"sys-admin",
    "password":"4mH36",
    "role":"admin"
}
*/

// @Summary      Register a new user
// @Description  Registers a new account.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        request body RegisterRequest true "Registration details"
// @Success      201 {object} MessageResponse "Registered successfully"
// @Failure      400 {object} ErrorResponse "Invalid request"
// @Failure      409 {object} ErrorResponse "ID already exists"
// @Failure      500 {object} ErrorResponse "register failed"
// @Router       /register [post]
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

// @Summary      Delete an account
// @Description  Deletes an account by its ID.
// @Tags         auth
// @Produce      json
// @Param        id path string true "Account ID"
// @Success      200 {object} MessageResponse "Deleted successfully"
// @Failure      404 {object} ErrorResponse "Not found"
// @Failure      500 {object} ErrorResponse "delete failed"
// @Router       /accounts/{id} [delete]
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

// @Summary      Change username (ID)
// @Description  Changes the ID of an existing account.
// @Tags         auth
// @Accept       json
// @Produce      json
// @Param        id path string true "Current Account ID"
// @Param        request body ChangeUsernameRequest true "New ID details"
// @Success      200 {object} MessageResponse "Username changed successfully"
// @Failure      400 {object} ErrorResponse "Invalid request"
// @Failure      404 {object} ErrorResponse "Not found"
// @Failure      409 {object} ErrorResponse "New ID already exists"
// @Failure      500 {object} ErrorResponse "change id failed"
// @Router       /accounts/{id} [patch]
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
