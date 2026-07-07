package auth

import (
	"net/http"
	"strings"

	"IRIS-backend/internal/platform/httpx"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	CtxUserIDKey    = "user_id"
	CtxRoleKey      = "role"
	CtxPrincipalKey = "principal"
)

// RequireAuth: Authorization: Bearer <token> を検証して context に sub/role を詰める
func RequireAuth(secret []byte, resolver AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if h == "" {
			httpx.AbortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing Authorization header")
			return
		}

		parts := strings.SplitN(h, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			httpx.AbortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid Authorization header")
			return
		}

		tokenStr := strings.TrimSpace(parts[1])
		if tokenStr == "" {
			httpx.AbortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "empty token")
			return
		}

		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			// alg 固定（none攻撃とか回避）
			if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, jwt.ErrTokenSignatureInvalid
			}
			return secret, nil
		}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
		if err != nil || token == nil || !token.Valid {
			httpx.AbortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			httpx.AbortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid claims")
			return
		}

		subAny, ok := claims["sub"]
		if !ok {
			httpx.AbortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "missing sub")
			return
		}
		sub, ok := subAny.(string)
		if !ok || sub == "" {
			httpx.AbortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "invalid sub")
			return
		}

		principal, err := resolver.GetPrincipal(c.Request.Context(), sub)
		if err != nil || principal == nil {
			httpx.AbortError(c, http.StatusUnauthorized, "UNAUTHORIZED", "account not found")
			return
		}

		role := ""
		if len(principal.Roles) > 0 {
			role = principal.Roles[0]
		}

		c.Set(CtxPrincipalKey, principal)
		c.Set(CtxUserIDKey, sub)
		c.Set(CtxRoleKey, role)
		c.Next()
	}
}

func GetPrincipal(c *gin.Context) (*Principal, bool) {
	value, ok := c.Get(CtxPrincipalKey)
	if !ok {
		return nil, false
	}

	principal, ok := value.(*Principal)
	if !ok || principal == nil {
		return nil, false
	}

	return principal, true
}

// RequireRole: 例) admin のみ許可したい時に追加
func RequireRole(roles ...string) gin.HandlerFunc {
	roleSet := make(map[string]struct{})
	for _, r := range roles {
		if r == "" {
			continue
		}
		roleSet[r] = struct{}{}
	}

	return func(c *gin.Context) {
		v, ok := c.Get(CtxRoleKey)
		if !ok {
			httpx.AbortError(c, http.StatusForbidden, "FORBIDDEN", "missing role")
			return
		}

		role, ok := v.(string)
		if !ok || role == "" {
			httpx.AbortError(c, http.StatusForbidden, "FORBIDDEN", "invalid role")
			return
		}

		_, allowed := roleSet[role]
		if !allowed {
			httpx.AbortError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
			return
		}

		c.Next()
	}
}

func RequireCapability(capability string) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, ok := GetPrincipal(c)
		if !ok {
			httpx.AbortError(c, http.StatusForbidden, "FORBIDDEN", "missing principal")
			return
		}
		if !principal.HasCapability(capability) {
			httpx.AbortError(c, http.StatusForbidden, "FORBIDDEN", "forbidden")
			return
		}

		c.Next()
	}
}
