package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// viewActor resolves the member and workspace a saved layout belongs to.
func viewActor(c *gin.Context) (uuid.UUID, uuid.UUID, bool) {
	uid, err := middleware.GetUserUUID(c)
	if err != nil {
		errx.Handle(c, errx.ErrUser)
		return uuid.Nil, uuid.Nil, false
	}
	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.Handle(c, errx.New(errx.BadRequest, "no organization selected"))
		return uuid.Nil, uuid.Nil, false
	}
	return uid, *orgID, true
}

// GetViewPreferences returns the caller's saved layout for one dashboard list
// in the current workspace, or the empty layout when none is saved.
// GET /me/views/:view
func (h *Handler) GetViewPreferences(c *gin.Context) {
	uid, orgID, ok := viewActor(c)
	if !ok {
		return
	}
	prefs, xerr := h.ViewPreferencesService.Get(c.Request.Context(), uid, orgID, c.Param("view"))
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"preferences": prefs})
}

// A field left out keeps its saved value; "columns": [] is the default layout,
// "sort": {"by": ""} the default sort, and "layout": null the default layout
// of a view that has one.
type updateViewPreferencesRequest struct {
	Columns *[]string        `json:"columns"`
	Sort    *models.ViewSort `json:"sort"`
	Layout  json.RawMessage  `json:"layout"`
}

// UpdateViewPreferences writes the caller's saved layout for one list: the
// columns, the sort, or both.
// PUT /me/views/:view
func (h *Handler) UpdateViewPreferences(c *gin.Context) {
	uid, orgID, ok := viewActor(c)
	if !ok {
		return
	}
	var req updateViewPreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}
	upd := models.ViewPreferencesUpdate{Columns: req.Columns, Sort: req.Sort, Layout: req.Layout}
	saved, xerr := h.ViewPreferencesService.Put(c.Request.Context(), uid, orgID, c.Param("view"), upd)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"preferences": saved})
}

// ResetViewPreferences forgets the caller's saved layout for one list.
// DELETE /me/views/:view
func (h *Handler) ResetViewPreferences(c *gin.Context) {
	uid, orgID, ok := viewActor(c)
	if !ok {
		return
	}
	if xerr := h.ViewPreferencesService.Reset(c.Request.Context(), uid, orgID, c.Param("view")); xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.Status(http.StatusNoContent)
}
