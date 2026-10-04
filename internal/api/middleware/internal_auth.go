package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
)

// InternalAuthMiddleware protects the internal routes the edge services
// (tracking, forms) call, with the static INTERNAL_API_TOKEN bearer. It fails
// closed when the token is unset; the compare is constant-time.
func (h *Handler) InternalAuthMiddleware() gin.HandlerFunc {
	return internalAuth
}

var (
	internalTokenOnce sync.Once
	internalToken     []byte
)

func loadInternalToken() {
	if v := os.Getenv("INTERNAL_API_TOKEN"); v != "" {
		internalToken = []byte(v)
	}
}

func internalAuth(c *gin.Context) {
	internalTokenOnce.Do(loadInternalToken)
	if len(internalToken) == 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "internal auth not configured"})
		return
	}
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
		return
	}
	provided := []byte(strings.TrimPrefix(header, "Bearer "))
	if subtle.ConstantTimeCompare(provided, internalToken) != 1 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid bearer token"})
		return
	}
	c.Next()
}

// NodeBrokerAuthMiddleware protects the internal routes only fleet nodes
// call: key material, blob signing, the message map, sync lookups, worker
// config and the node heartbeat.
//
// NODE_BROKER_TOKEN is that credential, kept apart from the one the
// internet-facing tracking and forms services carry. It falls back to
// INTERNAL_API_TOKEN when unset, so a single-token deployment keeps working.
func (h *Handler) NodeBrokerAuthMiddleware() gin.HandlerFunc {
	return nodeBrokerAuth
}

var (
	brokerTokenOnce sync.Once
	brokerToken     []byte
)

func loadBrokerToken() {
	if v := os.Getenv("NODE_BROKER_TOKEN"); v != "" {
		brokerToken = []byte(v)
		return
	}
	brokerToken = []byte(os.Getenv("INTERNAL_API_TOKEN"))
}

func nodeBrokerAuth(c *gin.Context) {
	brokerTokenOnce.Do(loadBrokerToken)
	if len(brokerToken) == 0 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "internal auth not configured"})
		return
	}
	header := c.GetHeader("Authorization")
	if !strings.HasPrefix(header, "Bearer ") {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
		return
	}
	provided := []byte(strings.TrimPrefix(header, "Bearer "))
	if subtle.ConstantTimeCompare(provided, brokerToken) != 1 {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid bearer token"})
		return
	}
	c.Next()
}
