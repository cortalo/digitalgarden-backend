package authhandler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const userIDContextKey = "userID"

// TokenVerifier is defined here, by the consumer, so *authtoken.Issuer
// satisfies it implicitly.
type TokenVerifier interface {
	Verify(tokenString string) (int64, error)
}

// RequireAuth reads "Authorization: Bearer <token>", verifies it, and
// stores the user ID in the gin context for downstream handlers to read
// via UserID. Requests without a valid token are aborted with 401 before
// reaching the route handler.
//
// No route uses this yet — publish (the next feature) will be the first.
// It's added now, alongside login, because it's the other half of the
// same auth package and has no meaningful dependency on publish existing.
func RequireAuth(verifier TokenVerifier) gin.HandlerFunc {
	const prefix = "Bearer "

	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}

		token := strings.TrimPrefix(header, prefix)
		userID, err := verifier.Verify(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.Set(userIDContextKey, userID)
		c.Next()
	}
}

// UserID returns the authenticated user's ID set by RequireAuth. Only
// call this on routes registered behind RequireAuth.
func UserID(c *gin.Context) (int64, bool) {
	v, ok := c.Get(userIDContextKey)
	if !ok {
		return 0, false
	}
	userID, ok := v.(int64)
	return userID, ok
}
