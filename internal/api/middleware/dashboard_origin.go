package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/warmbly/warmbly/internal/config"
)

func DashboardOriginMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := config.WithDashboardOrigin(c.Request.Context(), c.GetHeader("Origin"))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
