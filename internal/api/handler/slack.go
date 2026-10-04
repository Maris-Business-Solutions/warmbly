// Slack app endpoints: the two request URLs Slack calls (verified against
// SLACK_SIGNING_SECRET before anything is parsed) and the dashboard's Slack
// panel (status, channels, settings, member links).
package handler

import (
	"context"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/app/slackapp"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// slackMaxBody caps what is read from Slack before the signature is checked.
const slackMaxBody = 1 << 20

// slackIngress reads the capped raw body, verifies Slack's signature over it,
// then hands it to serve. Nothing is parsed before verification.
func (h *Handler) slackIngress(c *gin.Context, serve func(ctx context.Context, body []byte) (any, error)) {
	if h.SlackService == nil || !h.SlackService.Interactive() {
		errx.JSON(c, slackapp.ErrSlackNotConfigured)
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, slackMaxBody+1))
	if err != nil || len(body) > slackMaxBody {
		errx.JSON(c, errx.New(errx.BadRequest, "Request body is missing or too large."))
		return
	}
	if err := h.SlackService.Verify(c.GetHeader("X-Slack-Request-Timestamp"), c.GetHeader("X-Slack-Signature"), body); err != nil {
		errx.JSON(c, errx.New(errx.Unauthorized, "Invalid Slack request signature."))
		return
	}
	out, err := serve(c.Request.Context(), body)
	if err != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "Unreadable Slack payload."))
		return
	}
	if out == nil {
		c.Status(http.StatusOK)
		return
	}
	c.JSON(http.StatusOK, out)
}

// SlackEvents — POST /api/v1/integrations/slack/events
func (h *Handler) SlackEvents(c *gin.Context) {
	h.slackIngress(c, func(ctx context.Context, body []byte) (any, error) {
		return h.SlackService.HandleEvents(ctx, body)
	})
}

// SlackInteractivity — POST /api/v1/integrations/slack/interactivity
func (h *Handler) SlackInteractivity(c *gin.Context) {
	h.slackIngress(c, func(ctx context.Context, body []byte) (any, error) {
		return h.SlackService.HandleInteractivity(ctx, body)
	})
}

// slackCaller resolves the signed-in user and, when required, the org.
func (h *Handler) slackCaller(c *gin.Context, needOrg bool) (uuid.UUID, uuid.UUID, bool) {
	if h.SlackService == nil {
		errx.JSON(c, slackapp.ErrSlackNotConfigured)
		return uuid.Nil, uuid.Nil, false
	}
	userID, err := middleware.GetUserUUID(c)
	if err != nil {
		errx.JSON(c, errx.ErrUnauthorized)
		return uuid.Nil, uuid.Nil, false
	}
	if !needOrg {
		return uuid.Nil, userID, true
	}
	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.JSON(c, errx.New(errx.BadRequest, "no organization selected"))
		return uuid.Nil, uuid.Nil, false
	}
	return *orgID, userID, true
}

// GetSlackStatus — GET /v1/integrations/slack/status
func (h *Handler) GetSlackStatus(c *gin.Context) {
	orgID, userID, ok := h.slackCaller(c, true)
	if !ok {
		return
	}
	member, xerr := h.OrganizationService.GetMembership(c.Request.Context(), orgID, userID)
	if xerr != nil || member == nil {
		errx.JSON(c, errx.New(errx.Forbidden, "not a member of this organization"))
		return
	}
	access := slackapp.StatusOwnLink
	switch {
	case member.Permissions.HasPermission(models.PermManageSettings):
		access = slackapp.StatusManage
	case member.Permissions.HasPermission(models.PermUseIntegrations):
		access = slackapp.StatusWorkspace
	}
	st, xerr := h.SlackService.Status(c.Request.Context(), orgID, userID, access)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.JSON(http.StatusOK, st)
}

// ListSlackChannels — GET /v1/integrations/slack/channels?q=
func (h *Handler) ListSlackChannels(c *gin.Context) {
	orgID, _, ok := h.slackCaller(c, true)
	if !ok {
		return
	}
	list, more, xerr := h.SlackService.Channels(c.Request.Context(), orgID, c.Query("q"))
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list, "pagination": gin.H{"next_cursor": nil, "has_more": more}})
}

// UpdateSlackSettings — PUT /v1/integrations/slack/settings
func (h *Handler) UpdateSlackSettings(c *gin.Context) {
	orgID, _, ok := h.slackCaller(c, true)
	if !ok {
		return
	}
	var req models.SlackSettings
	if err := c.ShouldBindJSON(&req); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	st, xerr := h.SlackService.UpdateSettings(c.Request.Context(), orgID, req)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	h.auditOrg(c, models.AuditActionUpdate, models.AuditEntityIntegration, nil, nil, map[string]string{"slack": "settings"})
	c.JSON(http.StatusOK, st)
}

// PreviewSlackLink — GET /v1/integrations/slack/link/:code
func (h *Handler) PreviewSlackLink(c *gin.Context) {
	_, userID, ok := h.slackCaller(c, false)
	if !ok {
		return
	}
	p, xerr := h.SlackService.PreviewLink(c.Request.Context(), userID, c.Param("code"))
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.JSON(http.StatusOK, p)
}

// ConfirmSlackLink — POST /v1/integrations/slack/link
func (h *Handler) ConfirmSlackLink(c *gin.Context) {
	_, userID, ok := h.slackCaller(c, false)
	if !ok {
		return
	}
	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	link, xerr := h.SlackService.ConfirmLink(c.Request.Context(), userID, req.Code)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	// The org comes from the code, so the audit row is written against it.
	if h.AuditService != nil {
		h.AuditService.LogAction(c.Request.Context(), link.OrganizationID, userID, models.AuditActionCreate, models.AuditEntityIntegration,
			&link.ConnectionID, c.ClientIP(), c.Request.UserAgent(), nil, map[string]string{"slack_link": "linked"})
	}
	c.JSON(http.StatusCreated, link)
}

// UpdateMySlackLink — PATCH /v1/integrations/slack/link
func (h *Handler) UpdateMySlackLink(c *gin.Context) {
	orgID, userID, ok := h.slackCaller(c, true)
	if !ok {
		return
	}
	var req struct {
		DMNotifications *bool `json:"dm_notifications" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	link, xerr := h.SlackService.UpdateMyLink(c.Request.Context(), orgID, userID, *req.DMNotifications)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	h.auditOrg(c, models.AuditActionUpdate, models.AuditEntityIntegration, &link.ConnectionID, nil, map[string]string{"slack_link": "dm_notifications"})
	c.JSON(http.StatusOK, link)
}

// DeleteMySlackLink — DELETE /v1/integrations/slack/link
func (h *Handler) DeleteMySlackLink(c *gin.Context) {
	orgID, userID, ok := h.slackCaller(c, true)
	if !ok {
		return
	}
	if xerr := h.SlackService.UnlinkMine(c.Request.Context(), orgID, userID); xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	h.auditOrg(c, models.AuditActionDelete, models.AuditEntityIntegration, nil, nil, map[string]string{"slack_link": "unlinked"})
	c.Status(http.StatusNoContent)
}

// RemoveSlackLink — DELETE /v1/integrations/slack/links/:id
func (h *Handler) RemoveSlackLink(c *gin.Context) {
	orgID, _, ok := h.slackCaller(c, true)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errx.JSON(c, errx.ErrUuid)
		return
	}
	link, xerr := h.SlackService.RemoveLink(c.Request.Context(), orgID, id)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	h.auditOrg(c, models.AuditActionDelete, models.AuditEntityIntegration, &link.ConnectionID, nil, map[string]string{"slack_link": "removed"})
	c.Status(http.StatusNoContent)
}
