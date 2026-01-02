package auth

import (
	"github.com/gin-gonic/gin"
	"github.com/perber/wiki/internal/core/auth"
)

func InjectPublicEditor(authDisabled bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if authDisabled {
			if _, exists := c.Get("user"); !exists {
				c.Set("user", &auth.User{
					ID:       "public-editor",
					Username: "public-editor",
					Role:     auth.RoleEditor,
				})
			}
		}
		c.Next()
	}
}
