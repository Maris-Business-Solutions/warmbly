package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/app/salesforce"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/repository"
	"github.com/warmbly/warmbly/internal/utils/paging"
)

// salesforceReady resolves the org, the actor and the connection id, or
// answers the request itself.
func (h *Handler) salesforceReady(c *gin.Context) (orgID, userID, connID uuid.UUID, ok bool) {
	if h.SalesforceService == nil {
		errx.JSON(c, errx.New(errx.ServiceUnavailable, "Salesforce sync is not available on this instance"))
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	orgID, userID, ok = h.requireIntegrationActor(c, false)
	if !ok {
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	connID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "invalid connection id"))
		return uuid.Nil, uuid.Nil, uuid.Nil, false
	}
	return orgID, userID, connID, true
}

// salesforceFail answers a sync error: the service's own refusals as they
// are, anything else as an internal error with the cause logged.
func salesforceFail(c *gin.Context, err error, what string) {
	var xe *errx.Error
	if errors.As(err, &xe) {
		errx.JSON(c, xe)
		return
	}
	log.Error().Err(err).Str("op", what).Msg("salesforce request failed")
	errx.JSON(c, errx.InternalError())
}

// SalesforceOverview is the connection's health page.
func (h *Handler) SalesforceOverview(c *gin.Context) {
	orgID, _, connID, ok := h.salesforceReady(c)
	if !ok {
		return
	}
	out, err := h.SalesforceService.Overview(c.Request.Context(), orgID, connID, c.Query("checks") == "1")
	if err != nil {
		salesforceFail(c, err, "overview")
		return
	}
	c.JSON(http.StatusOK, out)
}

// GetSalesforceSettings returns the sync settings and the Warmbly field list.
func (h *Handler) GetSalesforceSettings(c *gin.Context) {
	orgID, _, connID, ok := h.salesforceReady(c)
	if !ok {
		return
	}
	st, err := h.SalesforceService.GetSettings(c.Request.Context(), orgID, connID)
	if err != nil {
		salesforceFail(c, err, "get settings")
		return
	}
	c.JSON(http.StatusOK, gin.H{"settings": st, "warmbly_fields": salesforce.WarmblyFields, "defaults": salesforce.DefaultSettings()})
}

// UpdateSalesforceSettings validates and saves the sync settings. A PUT of the
// whole document, so a retry converges.
func (h *Handler) UpdateSalesforceSettings(c *gin.Context) {
	orgID, userID, connID, ok := h.salesforceReady(c)
	if !ok {
		return
	}
	var st salesforce.Settings
	if err := c.ShouldBindJSON(&st); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	saved, err := h.SalesforceService.SaveSettings(c.Request.Context(), orgID, connID, st)
	if err != nil {
		salesforceFail(c, err, "save settings")
		return
	}
	h.auditIntegration(c, userID, models.AuditActionUpdate, connID, "salesforce:settings")
	c.JSON(http.StatusOK, gin.H{"settings": saved})
}

// SalesforceMetadata returns Lead and Contact fields and picklists.
func (h *Handler) SalesforceMetadata(c *gin.Context) {
	orgID, _, connID, ok := h.salesforceReady(c)
	if !ok {
		return
	}
	out, err := h.SalesforceService.Metadata(c.Request.Context(), orgID, connID)
	if err != nil {
		salesforceFail(c, err, "metadata")
		return
	}
	c.JSON(http.StatusOK, out)
}

// SalesforceUsers searches active Salesforce users.
func (h *Handler) SalesforceUsers(c *gin.Context) {
	orgID, _, connID, ok := h.salesforceReady(c)
	if !ok {
		return
	}
	out, err := h.SalesforceService.Users(c.Request.Context(), orgID, connID, c.Query("q"))
	if err != nil {
		salesforceFail(c, err, "users")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// SalesforceListViews lists Lead or Contact list views.
func (h *Handler) SalesforceListViews(c *gin.Context) {
	orgID, _, connID, ok := h.salesforceReady(c)
	if !ok {
		return
	}
	out, err := h.SalesforceService.ListViews(c.Request.Context(), orgID, connID, c.Query("object"))
	if err != nil {
		salesforceFail(c, err, "list views")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// SalesforceCampaigns searches Salesforce Campaigns.
func (h *Handler) SalesforceCampaigns(c *gin.Context) {
	orgID, _, connID, ok := h.salesforceReady(c)
	if !ok {
		return
	}
	out, err := h.SalesforceService.Campaigns(c.Request.Context(), orgID, connID, c.Query("q"))
	if err != nil {
		salesforceFail(c, err, "campaigns")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// PreviewSalesforceImport reads the first rows of a list view or Campaign.
// Read-only, so retries are naturally safe.
func (h *Handler) PreviewSalesforceImport(c *gin.Context) {
	orgID, _, connID, ok := h.salesforceReady(c)
	if !ok {
		return
	}
	var in salesforce.ImportSource
	if err := c.ShouldBindJSON(&in); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	out, err := h.SalesforceService.Preview(c.Request.Context(), orgID, connID, in)
	if err != nil {
		salesforceFail(c, err, "import preview")
		return
	}
	c.JSON(http.StatusOK, out)
}

// ListSalesforceImportSources lists a connection's saved imports.
func (h *Handler) ListSalesforceImportSources(c *gin.Context) {
	orgID, _, connID, ok := h.salesforceReady(c)
	if !ok {
		return
	}
	out, err := h.SalesforceService.Repo.ListSources(c.Request.Context(), orgID, connID)
	if err != nil {
		salesforceFail(c, err, "list sources")
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// CreateSalesforceImportSource saves an import and starts its first run. A
// retried create makes a second source whose run imports nothing new: people
// are deduplicated by address, so the retry is harmless.
func (h *Handler) CreateSalesforceImportSource(c *gin.Context) {
	orgID, userID, connID, ok := h.salesforceReady(c)
	if !ok {
		return
	}
	var in salesforce.SourceInput
	if err := c.ShouldBindJSON(&in); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	out, err := h.SalesforceService.CreateSource(c.Request.Context(), orgID, connID, userID, in)
	if err != nil {
		salesforceFail(c, err, "create source")
		return
	}
	h.auditIntegration(c, userID, models.AuditActionCreate, connID, "salesforce:import:"+out.ID.String())
	c.JSON(http.StatusCreated, out)
}

// UpdateSalesforceImportSource edits an import's targets and schedule.
func (h *Handler) UpdateSalesforceImportSource(c *gin.Context) {
	orgID, userID, connID, ok := h.salesforceReady(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("sourceId"))
	if err != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "invalid import source id"))
		return
	}
	raw, err := c.GetRawData()
	if err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	var in salesforce.SourceInput
	if err := json.Unmarshal(raw, &in); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	// campaign_id: null clears the target; absent leaves it.
	var probe map[string]json.RawMessage
	if json.Unmarshal(raw, &probe) == nil {
		if v, ok := probe["campaign_id"]; ok && strings.TrimSpace(string(v)) == "null" {
			in.ClearCampaign = true
		}
	}
	src, err := h.SalesforceService.Repo.GetSource(c.Request.Context(), orgID, id)
	if err != nil || src == nil || src.ConnectionID != connID {
		errx.JSON(c, errx.New(errx.NotFound, "import source not found"))
		return
	}
	out, err := h.SalesforceService.UpdateSource(c.Request.Context(), orgID, id, in)
	if err != nil {
		salesforceFail(c, err, "update source")
		return
	}
	h.auditIntegration(c, userID, models.AuditActionUpdate, connID, "salesforce:import:"+id.String())
	c.JSON(http.StatusOK, out)
}

// RunSalesforceImportSource runs an import now. A second request while one
// runs is refused with 409, so retries cannot double-run it.
func (h *Handler) RunSalesforceImportSource(c *gin.Context) {
	orgID, userID, connID, ok := h.salesforceReady(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("sourceId"))
	if err != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "invalid import source id"))
		return
	}
	src, err := h.SalesforceService.Repo.GetSource(c.Request.Context(), orgID, id)
	if err != nil || src == nil || src.ConnectionID != connID {
		errx.JSON(c, errx.New(errx.NotFound, "import source not found"))
		return
	}
	out, err := h.SalesforceService.StartRun(c.Request.Context(), orgID, id)
	if err != nil {
		salesforceFail(c, err, "run source")
		return
	}
	h.auditIntegration(c, userID, models.AuditActionUpdate, connID, "salesforce:import-run:"+id.String())
	c.JSON(http.StatusOK, out)
}

// DeleteSalesforceImportSource removes an import; contacts it brought in stay.
func (h *Handler) DeleteSalesforceImportSource(c *gin.Context) {
	orgID, userID, connID, ok := h.salesforceReady(c)
	if !ok {
		return
	}
	id, err := uuid.Parse(c.Param("sourceId"))
	if err != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "invalid import source id"))
		return
	}
	src, err := h.SalesforceService.Repo.GetSource(c.Request.Context(), orgID, id)
	if err != nil || src == nil || src.ConnectionID != connID {
		errx.JSON(c, errx.New(errx.NotFound, "import source not found"))
		return
	}
	if _, err := h.SalesforceService.Repo.DeleteSource(c.Request.Context(), orgID, id); err != nil {
		salesforceFail(c, err, "delete source")
		return
	}
	h.auditIntegration(c, userID, models.AuditActionDelete, connID, "salesforce:import:"+id.String())
	c.Status(http.StatusNoContent)
}

// ListSalesforceActivity pages the activity log, newest first.
func (h *Handler) ListSalesforceActivity(c *gin.Context) {
	orgID, _, connID, ok := h.salesforceReady(c)
	if !ok {
		return
	}
	f := repository.SalesforceActivityFilter{Limit: 50}
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 200 {
			errx.JSON(c, errx.New(errx.BadRequest, "limit must be between 1 and 200"))
			return
		}
		f.Limit = n
	}
	switch st := c.Query("status"); st {
	case "", models.SalesforceActivityPending, models.SalesforceActivitySynced, models.SalesforceActivitySkipped, models.SalesforceActivityFailed:
		f.Status = st
	default:
		errx.JSON(c, errx.New(errx.BadRequest, "status must be pending, synced, skipped or failed"))
		return
	}
	if v := c.Query("contact_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			errx.JSON(c, errx.New(errx.BadRequest, "invalid contact_id"))
			return
		}
		f.ContactID = &id
	}
	at, id, xerr := paging.DecodeTimeCursor(c.Query("cursor"))
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	if id != uuid.Nil {
		f.Before, f.BeforeID = &at, &id
	}
	rows, err := h.SalesforceService.Repo.ListActivities(c.Request.Context(), orgID, connID, f)
	if err != nil {
		salesforceFail(c, err, "list activity")
		return
	}
	var next *string
	if len(rows) > f.Limit {
		last := rows[f.Limit-1]
		next = paging.EncodeTime(last.CreatedAt, last.ID)
		rows = rows[:f.Limit]
	}
	if rows == nil {
		rows = []models.SalesforceActivity{}
	}
	c.JSON(http.StatusOK, gin.H{
		"data":       rows,
		"pagination": gin.H{"next_cursor": next, "has_more": next != nil},
	})
}

type salesforceRetryPayload struct {
	IDs []string `json:"ids"`
}

// RetrySalesforceActivity re-queues failed or skipped rows. Re-queuing a row
// that is already pending changes nothing, so retries are safe.
func (h *Handler) RetrySalesforceActivity(c *gin.Context) {
	orgID, userID, connID, ok := h.salesforceReady(c)
	if !ok {
		return
	}
	var p salesforceRetryPayload
	if err := c.ShouldBindJSON(&p); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	if len(p.IDs) > 500 {
		errx.JSON(c, errx.New(errx.BadRequest, "at most 500 ids per retry"))
		return
	}
	ids := make([]uuid.UUID, 0, len(p.IDs))
	for _, raw := range p.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			errx.JSON(c, errx.New(errx.BadRequest, "invalid activity id: "+raw))
			return
		}
		ids = append(ids, id)
	}
	n, err := h.SalesforceService.Repo.RetryActivities(c.Request.Context(), orgID, connID, ids)
	if err != nil {
		salesforceFail(c, err, "retry activity")
		return
	}
	h.auditIntegration(c, userID, models.AuditActionUpdate, connID, "salesforce:retry:"+strconv.Itoa(n))
	c.JSON(http.StatusOK, gin.H{"requeued": n})
}

// SalesforceSyncNow drains pending activity and pulls changes at once. Both
// are idempotent passes, so a retry only repeats a no-op.
func (h *Handler) SalesforceSyncNow(c *gin.Context) {
	orgID, _, connID, ok := h.salesforceReady(c)
	if !ok {
		return
	}
	if err := h.SalesforceService.SyncNow(c.Request.Context(), orgID, connID); err != nil {
		salesforceFail(c, err, "sync now")
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- contact panel ------------------------------------------------------------

func (h *Handler) salesforceContact(c *gin.Context) (orgID, contactID uuid.UUID, ok bool) {
	if h.SalesforceService == nil {
		errx.JSON(c, errx.New(errx.ServiceUnavailable, "Salesforce sync is not available on this instance"))
		return uuid.Nil, uuid.Nil, false
	}
	orgID, ok = requireOrgID(c)
	if !ok {
		return uuid.Nil, uuid.Nil, false
	}
	contactID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "invalid contact id"))
		return uuid.Nil, uuid.Nil, false
	}
	return orgID, contactID, true
}

// GetContactSalesforce returns the contact's Salesforce panel.
func (h *Handler) GetContactSalesforce(c *gin.Context) {
	orgID, contactID, ok := h.salesforceContact(c)
	if !ok {
		return
	}
	out, err := h.SalesforceService.ContactPanel(c.Request.Context(), orgID, contactID)
	if err != nil {
		salesforceFail(c, err, "contact panel")
		return
	}
	if out == nil {
		errx.JSON(c, errx.New(errx.NotFound, "contact not found"))
		return
	}
	c.JSON(http.StatusOK, out)
}

type contactSalesforceSyncPayload struct {
	ConnectionID *uuid.UUID `json:"connection_id"`
	CreateAs     string     `json:"create_as"`
}

// SyncContactSalesforce matches (or creates) the contact in Salesforce and
// pushes its mapped fields. Matching runs first, so a retry finds the record
// the first attempt created instead of creating another.
func (h *Handler) SyncContactSalesforce(c *gin.Context) {
	orgID, contactID, ok := h.salesforceContact(c)
	if !ok {
		return
	}
	var p contactSalesforceSyncPayload
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&p); err != nil {
			errx.JSON(c, errx.InvalidBody(err))
			return
		}
	}
	ctx := c.Request.Context()
	if err := h.SalesforceService.SyncContact(ctx, orgID, contactID, p.ConnectionID, p.CreateAs); err != nil {
		salesforceFail(c, err, "contact sync")
		return
	}
	h.SalesforceService.ForgetLive(ctx, orgID, contactID)
	out, err := h.SalesforceService.ContactPanel(ctx, orgID, contactID)
	if err != nil {
		salesforceFail(c, err, "contact panel")
		return
	}
	if uid, perr := uuid.Parse(middleware.GetUserID(c)); perr == nil {
		h.auditIntegrationEntity(c, uid, models.AuditActionUpdate, models.AuditEntityContact, contactID, "salesforce:sync")
	}
	c.JSON(http.StatusOK, out)
}

// UnlinkContactSalesforce drops a contact's link to a Salesforce record.
func (h *Handler) UnlinkContactSalesforce(c *gin.Context) {
	orgID, contactID, ok := h.salesforceContact(c)
	if !ok {
		return
	}
	linkID, err := uuid.Parse(c.Param("linkId"))
	if err != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "invalid link id"))
		return
	}
	if err := h.SalesforceService.Unlink(c.Request.Context(), orgID, contactID, linkID); err != nil {
		salesforceFail(c, err, "unlink")
		return
	}
	if uid, perr := uuid.Parse(middleware.GetUserID(c)); perr == nil {
		h.auditIntegrationEntity(c, uid, models.AuditActionUpdate, models.AuditEntityContact, contactID, "salesforce:unlink")
	}
	c.Status(http.StatusNoContent)
}
