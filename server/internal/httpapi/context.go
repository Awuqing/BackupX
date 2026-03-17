//go:build ignore

package httpapi

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

const claimsContextKey = "authClaims"

func getUserID(c *gin.Context) (uint, error) {
	value, ok := c.Get(claimsContextKey)
	if !ok {
		return 0, fmt.Errorf("missing auth claims")
	}
	claims, ok := value.(AuthClaims)
	if !ok {
		return 0, fmt.Errorf("invalid auth claims")
	}
	return claims.UserID, nil
}
