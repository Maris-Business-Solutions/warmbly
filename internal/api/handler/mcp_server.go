// Warmbly MCP server (server direction). Exposes the shared tool registry over
// the MCP streamable-HTTP transport at POST /api/v1/mcp, authenticated by an API
// key or OAuth token holding AI_AGENT. Each tool is gated by its RequiredAPIPerm
// bits, tools/list reflects only what the key's permission mask allows, and
// send-class tools are never exposed.
package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/app/aitools"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/pkg/generation"
)

const mcpProtocolVersion = "2025-03-26"

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPEndpoint — POST /api/v1/mcp. Handles one JSON-RPC message.
func (h *Handler) MCPEndpoint(c *gin.Context) {
	if h.AITools == nil {
		errx.JSON(c, errx.New(errx.ServiceUnavailable, "MCP is not available"))
		return
	}
	var req jsonRPCRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, rpcError(nil, -32700, "parse error"))
		return
	}

	// Notifications (no id) are acknowledged with no body.
	if len(req.ID) == 0 {
		c.Status(http.StatusAccepted)
		return
	}

	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		c.JSON(http.StatusOK, rpcError(req.ID, -32000, "no organization for this key"))
		return
	}
	if keyMailboxLimited(c) {
		c.JSON(http.StatusOK, rpcError(req.ID, -32000, errToolsMailboxLimited.Message))
		return
	}
	inv := aitools.Invocation{
		OrgID:     *orgID,
		IsAPIKey:  true,
		APIPerms:  middleware.GetAPIKeyPermissions(c),
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	}
	if uid, err := middleware.GetUserUUID(c); err == nil {
		inv.UserID = uid
	}
	bindOAuthMember(c, &inv)
	if !mcpAllowed(inv) {
		c.JSON(http.StatusOK, rpcError(req.ID, -32000, "this credential lacks the AI_AGENT permission, which MCP needs"))
		return
	}

	switch req.Method {
	case "initialize":
		c.JSON(http.StatusOK, rpcResult(req.ID, gin.H{
			"protocolVersion": mcpProtocolVersion,
			"capabilities":    gin.H{"tools": gin.H{}},
			"serverInfo":      gin.H{"name": "warmbly", "version": "1.0"},
		}))
	case "tools/list":
		c.JSON(http.StatusOK, rpcResult(req.ID, gin.H{"tools": h.mcpToolList(inv)}))
	case "tools/call":
		h.mcpToolCall(c, inv, req)
	case "ping":
		c.JSON(http.StatusOK, rpcResult(req.ID, gin.H{}))
	default:
		c.JSON(http.StatusOK, rpcError(req.ID, -32601, "method not found"))
	}
}

// mcpAllowed is the AI_AGENT gate: the key's scope, and for an OAuth token its member's use_ai.
func mcpAllowed(inv aitools.Invocation) bool {
	if !models.HasAPIPermission(inv.APIPerms, models.APIPermAIAgent) {
		return false
	}
	return !inv.ActsForMember || inv.OrgPerms.HasPermission(models.PermUseAI)
}

// mcpToolList reflects only the static tools the key's mask allows, excluding
// send-class tools (never exposed over MCP).
func (h *Handler) mcpToolList(inv aitools.Invocation) []gin.H {
	tools := h.AITools.PermittedTools(inv)
	out := make([]gin.H, 0, len(tools))
	for _, t := range tools {
		if t.Risk == generation.RiskSend {
			continue
		}
		schema := t.InputSchema
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		out = append(out, gin.H{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": schema,
		})
	}
	return out
}

func (h *Handler) mcpToolCall(c *gin.Context, inv aitools.Invocation, req jsonRPCRequest) {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil || params.Name == "" {
		c.JSON(http.StatusOK, rpcError(req.ID, -32602, "invalid params"))
		return
	}

	// Send-class tools are never exposed or callable over MCP.
	if t, ok := h.AITools.Get(params.Name); ok && t.Risk == generation.RiskSend {
		c.JSON(http.StatusOK, rpcError(req.ID, -32601, "tool not available"))
		return
	}

	out, err := h.AITools.Call(c.Request.Context(), inv, params.Name, params.Arguments)
	if err != nil {
		switch {
		case errors.Is(err, aitools.ErrToolNotFound):
			c.JSON(http.StatusOK, rpcError(req.ID, -32601, "tool not found"))
		case errors.Is(err, aitools.ErrToolForbidden):
			c.JSON(http.StatusOK, rpcError(req.ID, -32000, "your API key lacks the permission for this tool"))
		default:
			// A tool error is a normal MCP result with isError, so the client can
			// react rather than treating it as a protocol failure.
			msg, ok := aitools.PublicMessage(err)
			if !ok {
				log.Error().Str("request_id", c.GetString("request_id")).Str("tool", params.Name).Err(err).Msg("mcp tool failed")
				msg = "The tool failed on the server. Try again later."
			}
			c.JSON(http.StatusOK, rpcResult(req.ID, gin.H{
				"content": []gin.H{{"type": "text", "text": msg}},
				"isError": true,
			}))
		}
		return
	}
	c.JSON(http.StatusOK, rpcResult(req.ID, gin.H{
		"content": []gin.H{{"type": "text", "text": out}},
		"isError": false,
	}))
}

func rpcResult(id json.RawMessage, result any) gin.H {
	return gin.H{"jsonrpc": "2.0", "id": rawID(id), "result": result}
}

func rpcError(id json.RawMessage, code int, message string) gin.H {
	return gin.H{"jsonrpc": "2.0", "id": rawID(id), "error": gin.H{"code": code, "message": message}}
}

func rawID(id json.RawMessage) any {
	if len(id) == 0 {
		return nil
	}
	return id
}
