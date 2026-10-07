package middleware

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
)

// UUIDParams validates only the named resource parameters present on this route.
func UUIDParams(names ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, name := range names {
			value, present := c.Params.Get(name)
			if !present {
				continue
			}
			id, err := uuid.Parse(value)
			if err != nil {
				c.Abort()
				errx.JSON(c, errx.ErrUuid)
				return
			}
			for i := range c.Params {
				if c.Params[i].Key == name {
					c.Params[i].Value = id.String()
				}
			}
		}
		c.Next()
	}
}
