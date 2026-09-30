package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

func (h *Handler) AddContacts(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)

	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.Handle(c, errx.New(errx.BadRequest, "no organization selected"))
		return
	}

	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}
	// A single contact may be sent as a bare object; it is read as a one-item array.
	if trimmed := bytes.TrimSpace(raw); len(trimmed) > 0 && trimmed[0] == '{' {
		raw = append(append(json.RawMessage{'['}, trimmed...), ']')
	}
	var data []models.AddContact
	if err := json.Unmarshal(raw, &data); err != nil {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}

	if len(data) == 0 {
		errx.Handle(c, errx.New(errx.BadRequest, "no contacts provided"))
		return
	}
	if len(data) > config.MaxContactSize {
		errx.Handle(c, errx.New(errx.BadRequest, fmt.Sprintf("too many contacts, maximum is %d", config.MaxContactSize)))
		return
	}

	for i := range data {
		data[i].Source, data[i].SourceDetail = contactSourceFor(c, data[i].Source)
	}

	resp, err := h.ContactService.Add(c.Request.Context(), userIDStr, *orgID, data)
	if err != nil {
		errx.Handle(c, err)
		return
	}

	// Audit log - bulk import
	h.auditOrg(c, models.AuditActionImport, models.AuditEntityContact, nil, nil, map[string]string{"count": fmt.Sprintf("%d", len(data))})
	// New addresses are checked right away rather than on the next tick.
	if h.EmailVerifyService != nil {
		h.EmailVerifyService.Kick()
	}

	c.JSON(http.StatusOK, resp)
}

// contactSourceFor decides a new contact's first-touch source from the
// request: an API key is always "api" (named after the key), a dashboard
// session may say "campaign" when adding from a campaign's Leads tab, and
// anything else is a manual create.
func contactSourceFor(c *gin.Context, claimed models.ContactSource) (models.ContactSource, string) {
	if middleware.GetAPIKeyID(c) != nil {
		return models.ContactSourceAPI, middleware.GetAPIKeyName(c)
	}
	if claimed.RequestSettable() {
		return claimed, ""
	}
	return models.ContactSourceManual, ""
}

func (h *Handler) SearchContacts(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.Handle(c, errx.New(errx.BadRequest, "no organization selected"))
		return
	}

	cursor := c.Query("cursor")
	category := c.Query("category")
	limit := c.Query("limit")

	var data models.SearchContacts

	if err := c.ShouldBindJSON(&data); err != nil {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}

	resp, err := h.ContactService.Search(c.Request.Context(), orgID.String(), cursor, category, limit, data)
	if err != nil {
		errx.Handle(c, err)
		return
	}

	// Org-wide facet counts for the browse sidebar, returned by default on the
	// first page (no cursor); a `loadMore` reuses the counts the client already
	// has. Best-effort: a counts failure must not fail the search itself.
	if cursor == "" {
		if counts, cerr := h.ContactService.SearchCounts(c.Request.Context(), orgID.String()); cerr == nil {
			resp.Counts = counts
		}
		// Per-status lead totals for the campaign Leads view: only when the
		// search targets exactly one campaign. Independent of the request's
		// lead_status filter so every scope chip shows its own total.
		if len(data.CampaignIDs) == 1 {
			if lc, cerr := h.ContactService.CampaignLeadCounts(c.Request.Context(), orgID.String(), data.CampaignIDs[0]); cerr == nil {
				resp.LeadCounts = lc
			}
		}
	}

	c.JSON(http.StatusOK, resp)
}

// ListContactCustomFields returns the org's distinct contact custom-field keys,
// frequency-ranked (most common first) then alphabetical, capped at 200, so the
// dashboard variable picker can suggest fields contacts actually have. Read-only,
// no body, no query params.
func (h *Handler) ListContactCustomFields(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.Handle(c, errx.New(errx.BadRequest, "no organization selected"))
		return
	}

	keys, err := h.ContactService.ListCustomFieldKeys(c.Request.Context(), *orgID)
	if err != nil {
		errx.Handle(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": keys})
}

func (h *Handler) UpdateContactBulk(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)

	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.Handle(c, errx.New(errx.BadRequest, "no organization selected"))
		return
	}

	var data models.BulkEditContactsData

	if err := c.ShouldBindJSON(&data); err != nil {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}

	ids, ok := h.resolveContactSelection(c, *orgID, data.ContactSelection)
	if !ok {
		return
	}
	// From here the action names the ids it resolved to, so the service and
	// the audit entry never see the filter shape. A select-all can cover tens
	// of thousands of contacts, so its response carries the count rather than
	// every updated row.
	selectAll := data.All
	data.ContactSelection = models.ContactSelection{Contacts: ids}
	data.SkipRows = selectAll

	resp, err := h.ContactService.BulkUpdate(c.Request.Context(), userIDStr, *orgID, &data)
	if err != nil {
		errx.Handle(c, err)
		return
	}

	// Audit log - bulk update
	h.auditOrg(c, models.AuditActionUpdate, models.AuditEntityContact, nil, nil, map[string]string{"bulk": "true", "count": fmt.Sprintf("%d", len(data.Contacts))})

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) UpdateContact(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)

	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.Handle(c, errx.New(errx.BadRequest, "no organization selected"))
		return
	}

	id := c.Param("id")

	var data models.UpdateContact

	if err := c.ShouldBindJSON(&data); err != nil {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}

	resp, err := h.ContactService.Update(c.Request.Context(), userIDStr, id, *orgID, &data)
	if err != nil {
		errx.Handle(c, err)
		return
	}

	// Audit log
	if contactID, err := uuid.Parse(id); err == nil {
		h.auditOrg(c, models.AuditActionUpdate, models.AuditEntityContact, &contactID, nil, nil)
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) DeleteContactBulk(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)

	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.Handle(c, errx.New(errx.BadRequest, "no organization selected"))
		return
	}

	// Two accepted bodies: the original bare id array, and a selection object
	// ({"all":true,"filters":{...}}) for "select all matching". The array form
	// is the published contract, so it keeps working untouched.
	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}
	var sel models.ContactSelection
	if err := json.Unmarshal(raw, &sel.Contacts); err != nil {
		if err := json.Unmarshal(raw, &sel); err != nil {
			errx.Handle(c, errx.InvalidBody(err))
			return
		}
	}

	ids, ok := h.resolveContactSelection(c, *orgID, sel)
	if !ok {
		return
	}

	if err := h.ContactService.BulkDelete(c.Request.Context(), userIDStr, *orgID, ids); err != nil {
		errx.Handle(c, err)
		return
	}

	// Audit log - bulk delete
	h.auditOrg(c, models.AuditActionDelete, models.AuditEntityContact, nil, nil, map[string]string{"bulk": "true", "count": fmt.Sprintf("%d", len(ids))})

	c.Status(http.StatusNoContent)
}

func (h *Handler) DeleteContact(c *gin.Context) {
	userIDStr := middleware.GetUserID(c)

	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.Handle(c, errx.New(errx.BadRequest, "no organization selected"))
		return
	}

	id := c.Param("id")

	if err := h.ContactService.Delete(c.Request.Context(), userIDStr, *orgID, id); err != nil {
		errx.Handle(c, err)
		return
	}

	// Audit log
	if contactID, err := uuid.Parse(id); err == nil {
		h.auditOrg(c, models.AuditActionDelete, models.AuditEntityContact, &contactID, nil, nil)
	}

	c.Status(http.StatusNoContent)
}
