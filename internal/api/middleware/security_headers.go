package middleware

import (
	"net"
	"strings"

	"github.com/gin-gonic/gin"
)

// SecurityHeaders sets the response headers that tell a browser what it may do
// with an API response, on every response the API serves.
//
// The API answers JSON, so the policy can be the strictest one there is. It
// loads nothing, frames nothing, and may not be framed. That matters because
// the same origin also serves /public (customer uploads) and the OAuth bouncer
// pages, and those are the responses an attacker would want rendered.
//
// HSTS is only emitted on a request that actually arrived over TLS. Sending it
// over plain HTTP is ignored by browsers anyway, and a self-hosted instance
// deliberately running on a LAN without TLS must not be pinned to a scheme it
// does not serve.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()

		// A browser must never guess a content type here: /public serves
		// customer uploads from this origin.
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-site")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), interest-cohort=()")

		// default-src 'none' suits a JSON API. The handful of routes that
		// return HTML (the OAuth bouncers, the unsubscribe page) set their own
		// policy over this one, and /public sets an image sandbox.
		if _, already := h["Content-Security-Policy"]; !already {
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		}

		if requestIsHTTPS(c) {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		c.Next()
	}
}

// PageHeaders is the part of SecurityHeaders a service serving framable pages
// can carry (the forms service): no content sniffing, a referrer policy, and
// HSTS over TLS on ownHost only, never on a customer's domain. Each page sets its own CSP.
func PageHeaders(ownHost string) gin.HandlerFunc {
	ownHost = strings.ToLower(ownHost)
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if ownHost != "" && requestIsHTTPS(c) && strings.EqualFold(requestHostname(c.Request.Host), ownHost) {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		c.Next()
	}
}

// requestIsHTTPS reports whether the client reached us over TLS. Termination is
// always upstream (Railway, Caddy, nginx), so the forwarded header is the
// signal.
//
// The header is read directly: gin's trusted-proxy handling covers ClientIP,
// not arbitrary headers. That is acceptable here because the only thing it
// decides is whether to send HSTS, a browser cannot set the header on a request
// it makes, and a non-browser client that sets it receives a header it ignores.
// Nothing is authorized on this value.
func requestIsHTTPS(c *gin.Context) bool {
	if c.Request.TLS != nil {
		return true
	}
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		return strings.EqualFold(strings.TrimSpace(strings.Split(proto, ",")[0]), "https")
	}
	return false
}

// requestHostname is the request's Host without a port.
func requestHostname(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}
