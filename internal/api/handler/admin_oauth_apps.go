package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// Operator moderation of OAuth apps (/admin/oauth-apps) and of who may build
// them (/admin/oauth-developer-blocks).

// AdminListOAuthApps is GET /admin/oauth-apps.
func (h *Handler) AdminListOAuthApps(c *gin.Context) {
	var search models.AdminOAuthAppSearch
	if err := c.ShouldBindQuery(&search); err != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "invalid query parameters"))
		return
	}
	offset, ok := offsetCursor(c)
	if !ok {
		return
	}
	search.Offset = offset
	res, xerr := h.OAuthService.AdminListApps(c.Request.Context(), &search)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.JSON(http.StatusOK, res)
}

// AdminSuspendOAuthApp is POST /admin/oauth-apps/:id/suspend.
func (h *Handler) AdminSuspendOAuthApp(c *gin.Context) {
	adminID, id, ok := h.adminTarget(c)
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	app, xerr := h.OAuthService.AdminSuspend(c.Request.Context(), id, adminID, body.Reason)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	h.audit(c, models.AuditAction("suspend_oauth_app"), models.AuditEntityOAuthApplication, &id, map[string]string{
		"name": app.Name, "organization_id": app.OrganizationID.String(), "reason": body.Reason,
	})
	c.JSON(http.StatusOK, app)
}

// AdminUnsuspendOAuthApp is POST /admin/oauth-apps/:id/unsuspend.
func (h *Handler) AdminUnsuspendOAuthApp(c *gin.Context) {
	_, id, ok := h.adminTarget(c)
	if !ok {
		return
	}
	app, xerr := h.OAuthService.AdminUnsuspend(c.Request.Context(), id)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	h.audit(c, models.AuditAction("unsuspend_oauth_app"), models.AuditEntityOAuthApplication, &id, map[string]string{
		"name": app.Name, "organization_id": app.OrganizationID.String(),
	})
	c.JSON(http.StatusOK, app)
}

// AdminRevokeOAuthAppGrants is POST /admin/oauth-apps/:id/revoke-grants.
func (h *Handler) AdminRevokeOAuthAppGrants(c *gin.Context) {
	_, id, ok := h.adminTarget(c)
	if !ok {
		return
	}
	n, xerr := h.OAuthService.AdminRevokeGrants(c.Request.Context(), id)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	h.audit(c, models.AuditAction("revoke_oauth_app_grants"), models.AuditEntityOAuthApplication, &id, map[string]string{
		"revoked": strconv.FormatInt(n, 10),
	})
	c.JSON(http.StatusOK, gin.H{"revoked": n})
}

// AdminRemoveOAuthAppLogo is POST /admin/oauth-apps/:id/remove-logo.
func (h *Handler) AdminRemoveOAuthAppLogo(c *gin.Context) {
	_, id, ok := h.adminTarget(c)
	if !ok {
		return
	}
	app, previous, xerr := h.OAuthService.AdminRemoveLogo(c.Request.Context(), id)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	h.deleteAppLogo(c.Request.Context(), app.OrganizationID, id, previous, "")
	h.audit(c, models.AuditAction("remove_oauth_app_logo"), models.AuditEntityOAuthApplication, &id, map[string]string{"name": app.Name})
	c.JSON(http.StatusOK, app)
}

// AdminListOAuthDeveloperBlocks is GET /admin/oauth-developer-blocks.
func (h *Handler) AdminListOAuthDeveloperBlocks(c *gin.Context) {
	blocks, xerr := h.OAuthService.AdminListBlocks(c.Request.Context())
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": blocks})
}

// AdminCreateOAuthDeveloperBlock is POST /admin/oauth-developer-blocks.
func (h *Handler) AdminCreateOAuthDeveloperBlock(c *gin.Context) {
	adminID, ok := h.adminID(c)
	if !ok {
		return
	}
	var body models.CreateOAuthDeveloperBlock
	if err := c.ShouldBindJSON(&body); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	block, suspended, xerr := h.OAuthService.AdminBlock(c.Request.Context(), adminID, body)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	meta := map[string]string{"reason": block.Reason, "suspended_apps": strconv.Itoa(suspended)}
	if block.OrganizationID != nil {
		meta["organization_id"] = block.OrganizationID.String()
	}
	if block.UserID != nil {
		meta["user_id"] = block.UserID.String()
	}
	h.audit(c, models.AuditAction("block_oauth_developer"), models.AuditEntityOAuthDeveloperBlock, &block.ID, meta)
	c.JSON(http.StatusCreated, gin.H{"block": block, "suspended_apps": suspended})
}

// AdminDeleteOAuthDeveloperBlock is DELETE /admin/oauth-developer-blocks/:id.
func (h *Handler) AdminDeleteOAuthDeveloperBlock(c *gin.Context) {
	_, id, ok := h.adminTarget(c)
	if !ok {
		return
	}
	if xerr := h.OAuthService.AdminUnblock(c.Request.Context(), id); xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	h.audit(c, models.AuditAction("unblock_oauth_developer"), models.AuditEntityOAuthDeveloperBlock, &id, nil)
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (h *Handler) adminID(c *gin.Context) (uuid.UUID, bool) {
	id := middleware.GetAdminUserID(c)
	if id == nil {
		errx.JSON(c, errx.ErrUnauthorized)
		return uuid.Nil, false
	}
	return *id, true
}

// adminTarget reads the acting operator and the :id being acted on.
func (h *Handler) adminTarget(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	adminID, ok := h.adminID(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	id, ok := h.parseID(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	return adminID, id, true
}
