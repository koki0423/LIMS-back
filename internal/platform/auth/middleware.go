package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	CtxUserIDKey = "user_id"
	CtxRoleKey   = "role"
)

// RequireAuth: Authorization: Bearer <token> を検証して context に sub/role を詰める
func RequireAuth(secret []byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if h == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing Authorization header"})
			return
		}

		parts := strings.SplitN(h, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid Authorization header"})
			return
		}

		tokenStr := strings.TrimSpace(parts[1])
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "empty token"})
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
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
			return
		}

		subAny, ok := claims["sub"]
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing sub"})
			return
		}
		sub, ok := subAny.(string)
		if !ok || sub == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid sub"})
			return
		}

		role := ""
		roleAny, hasRole := claims["role"]
		if hasRole {
			roleStr, ok := roleAny.(string)
			if ok {
				role = roleStr
			}
		}

		c.Set(CtxUserIDKey, sub)
		c.Set(CtxRoleKey, role)
		c.Next()
	}
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
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "missing role"})
			return
		}

		role, ok := v.(string)
		if !ok || role == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid role"})
			return
		}

		_, allowed := roleSet[role]
		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}

		c.Next()
	}
}
