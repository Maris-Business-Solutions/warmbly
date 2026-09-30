package middleware

import (
	"strings"
	"time"

	"github.com/MicahParks/keyfunc/v3"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/warmbly/warmbly/internal/errx"
)

const (
	googleIssuer = "https://accounts.google.com"
)

type OidcHandler struct {
	ServiceAccount string
	KeySet         keyfunc.Keyfunc
	AppEnv         string
	// Audience is the webhook URL Cloud Tasks names in the token it mints.
	// Required: an empty one refuses every request.
	Audience string
}

func (h *OidcHandler) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.AppEnv == "dev" {
			c.Next()
			return
		}

		// No key set means the GCP Cloud Tasks OIDC path isn't configured (the
		// default local dispatcher calls handlers in-process). Fail closed
		// rather than deref a nil key set if a webhook request slips through.
		if h.KeySet == nil || h.Audience == "" {
			deny(c)
			return
		}

		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			deny(c)
			return
		}

		tokenStr := strings.TrimPrefix(auth, "Bearer ")

		opts := []jwt.ParserOption{
			jwt.WithLeeway(10 * time.Second),
			jwt.WithValidMethods([]string{"RS256"}),
			jwt.WithExpirationRequired(),
			jwt.WithAudience(h.Audience),
		}

		token, err := jwt.Parse(tokenStr, h.KeySet.Keyfunc, opts...)
		if err != nil {
			deny(c)
			return
		}

		if !token.Valid {
			deny(c)
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			deny(c)
			return
		}

		if iss, _ := claims.GetIssuer(); iss != googleIssuer {
			deny(c)
			return
		}

		if sAccount, _ := claims.GetSubject(); sAccount != h.ServiceAccount {
			deny(c)
			return
		}

		c.Next()
	}
}

// deny answers 403 and stops the chain; errx.Handle alone lets the handler run.
func deny(c *gin.Context) {
	errx.Handle(c, errx.ErrForbidden)
	c.Abort()
}
