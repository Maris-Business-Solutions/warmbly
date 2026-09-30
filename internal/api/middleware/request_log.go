package middleware

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger is gin's standard access log with every credential removed.
//
// gin.Logger() prints the full request URI, and several routes carry a
// credential there because the provider or the mail client puts it there:
// OAuth `code` and `state` on the callback bouncers, the team invitation token
// on the preview lookup, the socket ticket on /v1/getaway's reply, the form
// prefill ticket. Others carry one in the path: inbound webhook secrets,
// unsubscribe tokens, device and warmup tokens. None of them may reach stdout,
// the container log, or whatever aggregates it.
//
// So the query is dropped and a path parameter named in credentialParams is
// logged as its name. A request that has to be traced has X-Request-Id.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		end := time.Now()
		fmt.Fprintf(gin.DefaultWriter, "[GIN] %v | %3d | %13v | %15s | %-7s %#v\n",
			end.Format("2006/01/02 - 15:04:05"),
			c.Writer.Status(),
			end.Sub(start),
			c.ClientIP(),
			c.Request.Method,
			loggedPath(c),
		)
	}
}

// credentialParams are the route parameter names whose value is a credential.
var credentialParams = map[string]bool{"secret": true, "token": true}

// loggedPath is the request path with every credential parameter replaced by
// its name, rebuilt from the matched route so no value is guessed at.
func loggedPath(c *gin.Context) string {
	path := c.Request.URL.Path
	route := c.FullPath()
	if route == "" {
		return path
	}
	redact := false
	for _, p := range c.Params {
		if credentialParams[p.Key] {
			redact = true
			break
		}
	}
	if !redact {
		return path
	}

	segs := strings.Split(route, "/")
	for i, seg := range segs {
		switch {
		case strings.HasPrefix(seg, ":"):
			if name := seg[1:]; !credentialParams[name] {
				segs[i] = c.Param(name)
			}
		case strings.HasPrefix(seg, "*"):
			segs[i] = strings.TrimPrefix(c.Param(seg[1:]), "/")
		}
	}
	return strings.Join(segs, "/")
}
