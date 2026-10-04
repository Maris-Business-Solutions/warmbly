package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/app/appdirectory"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/utils/paging"
)

// --- Developer: an app's own listing (GET/PUT/DELETE /oauth/applications/:id/listing)

func (h *Handler) appListingTarget(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	orgID, ok := requireOrgID(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "invalid id"))
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, id, true
}

// GetOAuthAppListing returns the app's directory listing; listing is null when unpublished.
func (h *Handler) GetOAuthAppListing(c *gin.Context) {
	orgID, appID, ok := h.appListingTarget(c)
	if !ok {
		return
	}
	l, xerr := h.AppDirectoryService.GetListing(c.Request.Context(), orgID, appID)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"listing": l})
}

// PutOAuthAppListing publishes or edits the app's listing.
func (h *Handler) PutOAuthAppListing(c *gin.Context) {
	orgID, appID, ok := h.appListingTarget(c)
	if !ok {
		return
	}
	userID, uerr := middleware.GetUserUUID(c)
	if uerr != nil {
		errx.JSON(c, errx.ErrUnauthorized)
		return
	}
	var w models.AppListingWrite
	if err := c.ShouldBindJSON(&w); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	l, outcome, xerr := h.AppDirectoryService.SaveListing(c.Request.Context(), orgID, userID, appID, w)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	if outcome != appdirectory.SaveUnchanged {
		action := models.AuditActionUpdate
		if outcome == appdirectory.SaveCreated {
			action = models.AuditActionCreate
		}
		h.auditOrg(c, action, models.AuditEntityAppListing, &appID, nil, map[string]string{"slug": l.Slug, "status": string(l.Status)})
	}
	c.JSON(http.StatusOK, gin.H{"listing": l})
}

// DeleteOAuthAppListing unpublishes the app.
func (h *Handler) DeleteOAuthAppListing(c *gin.Context) {
	orgID, appID, ok := h.appListingTarget(c)
	if !ok {
		return
	}
	if xerr := h.AppDirectoryService.DeleteListing(c.Request.Context(), orgID, appID); xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	h.auditOrg(c, models.AuditActionDelete, models.AuditEntityAppListing, &appID, nil, nil)
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// --- Workspace: browsing the directory (GET /integrations/community[/:slug])

// ListCommunityApps returns the listings shown in discovery: featured first, then most installed.
func (h *Handler) ListCommunityApps(c *gin.Context) {
	orgID, ok := requireOrgID(c)
	if !ok {
		return
	}
	limit := 100
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > 200 {
			errx.JSON(c, errx.New(errx.BadRequest, "limit must be between 1 and 200"))
			return
		}
		limit = n
	}
	offset, xerr := paging.DecodeOffsetCursor(c.Query("cursor"))
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	apps, total, xerr := h.AppDirectoryService.Browse(c.Request.Context(), orgID, limit, offset)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	pg := models.Pagination{Total: &total}
	if int64(offset+len(apps)) < total {
		pg.HasMore = true
		pg.NextCursor = paging.EncodeOffset(offset + len(apps))
	}
	c.JSON(http.StatusOK, gin.H{"data": apps, "pagination": pg})
}

// GetCommunityApp opens a published listing by its link, listed or not.
func (h *Handler) GetCommunityApp(c *gin.Context) {
	orgID, ok := requireOrgID(c)
	if !ok {
		return
	}
	app, xerr := h.AppDirectoryService.Open(c.Request.Context(), orgID, c.Param("slug"))
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.JSON(http.StatusOK, app)
}

// --- Operator: directory moderation (/admin/app-listings)

// AdminListAppListings is GET /admin/app-listings.
func (h *Handler) AdminListAppListings(c *gin.Context) {
	var search models.AdminAppListingSearch
	if err := c.ShouldBindQuery(&search); err != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "invalid query parameters"))
		return
	}
	offset, ok := offsetCursor(c)
	if !ok {
		return
	}
	search.Offset = offset
	res, xerr := h.AppDirectoryService.AdminList(c.Request.Context(), &search)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.JSON(http.StatusOK, res)
}

// AdminSetAppListingStatus is PUT /admin/app-listings/:id/status: feature,
// unfeature (published) or hide a listing.
func (h *Handler) AdminSetAppListingStatus(c *gin.Context) {
	adminID := middleware.GetAdminUserID(c)
	if adminID == nil {
		errx.JSON(c, errx.ErrUnauthorized)
		return
	}
	id, ok := h.parseID(c)
	if !ok {
		return
	}
	var body models.SetAppListingStatus
	if err := c.ShouldBindJSON(&body); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	item, xerr := h.AppDirectoryService.SetStatus(c.Request.Context(), id, *adminID, body.Status, body.Note)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	h.audit(c, models.AuditAction("set_app_listing_status"), models.AuditEntityAppListing, &id, map[string]string{
		"slug":            item.Slug,
		"organization_id": item.OrganizationID.String(),
		"status":          string(item.Status),
		"note":            body.Note,
	})
	c.JSON(http.StatusOK, item)
}
