package middleware

import (
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/warmbly/warmbly/internal/errx"
)

func QueryValidation() gin.HandlerFunc {
	return func(c *gin.Context) {
		values, err := url.ParseQuery(c.Request.URL.RawQuery)
		if err == nil {
			err = errx.ValidateText(values)
		}
		if err != nil {
			c.Abort()
			errx.JSON(c, errx.InvalidQuery(err))
			return
		}
		c.Next()
	}
}
