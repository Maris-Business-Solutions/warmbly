package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// CRM mode: which CRM the workspace runs on, and the HubSpot side of it.

func (h *Handler) crmReady(c *gin.Context) (uuid.UUID, bool) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.Handle(c, errx.New(errx.BadRequest, "no organization selected"))
		return uuid.Nil, false
	}
	if h.HubSpot == nil {
		errx.Handle(c, errx.New(errx.NotImplemented, "CRM providers are not available on this instance"))
		return uuid.Nil, false
	}
	return *orgID, true
}

// GetCRMSettings returns the workspace's CRM mode and the connected account.
func (h *Handler) GetCRMSettings(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	out, xerr := h.HubSpot.Settings(c.Request.Context(), orgID)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, out)
}

// UpdateCRMSettings switches the CRM and stores the setup choices.
func (h *Handler) UpdateCRMSettings(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	var body models.UpdateCRMSettings
	if err := c.ShouldBindJSON(&body); err != nil {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}
	out, xerr := h.HubSpot.UpdateSettings(c.Request.Context(), orgID, &body)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	h.auditOrg(c, models.AuditActionUpdate, models.AuditEntityIntegration, out.ConnectionID, nil,
		map[string]string{"crm_provider": string(out.Provider)})
	c.JSON(http.StatusOK, out)
}

// GetCRMMetadata returns the provider's lifecycle stages, lead statuses, task
// types, contact properties and pipelines for the pickers.
func (h *Handler) GetCRMMetadata(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	out, xerr := h.HubSpot.Metadata(c.Request.Context(), orgID)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, out)
}

// ListCRMOwners lists the provider's owners and the member each one is.
func (h *Handler) ListCRMOwners(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	out, xerr := h.HubSpot.Owners(c.Request.Context(), orgID)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// MapCRMOwner pins an owner to a member (or clears the match).
func (h *Handler) MapCRMOwner(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	ext := strings.TrimSpace(c.Param("externalId"))
	if ext == "" || len(ext) > 64 {
		errx.Handle(c, errx.New(errx.BadRequest, "invalid owner id"))
		return
	}
	var body models.MapCRMOwner
	if err := c.ShouldBindJSON(&body); err != nil {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}
	if xerr := h.HubSpot.MapOwner(c.Request.Context(), orgID, ext, body.UserID); xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	h.auditOrg(c, models.AuditActionAssign, models.AuditEntityIntegration, nil, nil, map[string]string{"crm_owner": ext})
	c.Status(http.StatusNoContent)
}

// GetCRMSyncHealth reports what is waiting, what failed and when each pull ran.
func (h *Handler) GetCRMSyncHealth(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	out, xerr := h.HubSpot.SyncHealth(c.Request.Context(), orgID)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, out)
}

type crmJobSelection struct {
	IDs []uuid.UUID `json:"ids" binding:"max=500"`
}

// RetryCRMSyncFailures requeues failed sync jobs (all, or the ids given).
// Naturally idempotent: requeueing a requeued job changes nothing.
func (h *Handler) RetryCRMSyncFailures(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	var body crmJobSelection
	if err := c.ShouldBindJSON(&body); err != nil && err != io.EOF {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}
	n, xerr := h.HubSpot.RetryFailed(c.Request.Context(), orgID, body.IDs)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"affected": n})
}

// DiscardCRMSyncFailures drops failed sync jobs (all, or the ids given).
func (h *Handler) DiscardCRMSyncFailures(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	var body crmJobSelection
	if err := c.ShouldBindJSON(&body); err != nil && err != io.EOF {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}
	n, xerr := h.HubSpot.DiscardFailed(c.Request.Context(), orgID, body.IDs)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"affected": n})
}

// SyncCRMNow starts a full pull from the provider.
func (h *Handler) SyncCRMNow(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	if xerr := h.HubSpot.SyncNow(c.Request.Context(), orgID); xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.Status(http.StatusAccepted)
}

func (h *Handler) crmContactID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		errx.Handle(c, errx.ErrUuid)
		return uuid.Nil, false
	}
	return id, true
}

// GetCRMContact returns the provider side of a contact: owner, lifecycle stage,
// lead status, company and the chosen properties.
func (h *Handler) GetCRMContact(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	contactID, ok := h.crmContactID(c)
	if !ok {
		return
	}
	out, xerr := h.HubSpot.ContactView(c.Request.Context(), orgID, contactID)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, out)
}

// RefreshCRMContact pulls a contact's provider record, deals, tasks and notes
// now. Debounced per contact, so it is safe to call whenever a panel opens.
func (h *Handler) RefreshCRMContact(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	contactID, ok := h.crmContactID(c)
	if !ok {
		return
	}
	out, xerr := h.HubSpot.RefreshContact(c.Request.Context(), orgID, contactID)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, out)
}

// LinkCRMContact finds or creates the contact in the provider.
func (h *Handler) LinkCRMContact(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	contactID, ok := h.crmContactID(c)
	if !ok {
		return
	}
	out, xerr := h.HubSpot.LinkContact(c.Request.Context(), orgID, contactID)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	h.auditOrg(c, models.AuditActionConnect, models.AuditEntityContact, &contactID, nil, map[string]string{"crm": "hubspot"})
	c.JSON(http.StatusOK, out)
}

// UpdateCRMContact edits owner, lifecycle stage or lead status in place.
func (h *Handler) UpdateCRMContact(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	contactID, ok := h.crmContactID(c)
	if !ok {
		return
	}
	var body models.UpdateCRMContact
	if err := c.ShouldBindJSON(&body); err != nil {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}
	for _, v := range []*string{body.OwnerExternalID, body.LifecycleStage, body.LeadStatus} {
		if v != nil && len(*v) > 100 {
			errx.Handle(c, errx.New(errx.BadRequest, "value too long"))
			return
		}
	}
	out, xerr := h.HubSpot.UpdateContactRecord(c.Request.Context(), orgID, contactID, &body)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	h.auditOrg(c, models.AuditActionUpdate, models.AuditEntityContact, &contactID, nil, map[string]string{"crm": "hubspot"})
	c.JSON(http.StatusOK, out)
}

// ListCRMLists lists the provider's contact lists for import.
func (h *Handler) ListCRMLists(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	limit := 25
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 100 {
			errx.Handle(c, errx.New(errx.BadRequest, "limit must be between 1 and 100"))
			return
		}
		limit = n
	}
	q := c.Query("q")
	if len(q) > 200 {
		errx.Handle(c, errx.New(errx.BadRequest, "query too long"))
		return
	}
	out, xerr := h.HubSpot.Lists(c.Request.Context(), orgID, q, c.Query("cursor"), limit)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, out)
}

// PreviewCRMImport counts who a list import brings in and who it skips.
func (h *Handler) PreviewCRMImport(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	var body models.CRMImportRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}
	out, xerr := h.HubSpot.PreviewImport(c.Request.Context(), orgID, &body)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, out)
}

// ImportCRMList turns a provider list into a contact import draft, finished in
// the regular import review.
func (h *Handler) ImportCRMList(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	userID, err := middleware.GetUserUUID(c)
	if err != nil {
		errx.Handle(c, errx.New(errx.Unauthorized, "a signed-in member starts an import"))
		return
	}
	var body models.CRMImportRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}
	out, xerr := h.HubSpot.Import(c.Request.Context(), orgID, userID, &body)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	h.auditOrg(c, models.AuditActionImport, models.AuditEntityContact, nil, nil, map[string]string{"crm_list": body.ListID})
	c.JSON(http.StatusCreated, out)
}

// GetCRMBackfill counts the Warmbly-only records a switch would copy.
func (h *Handler) GetCRMBackfill(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	out, xerr := h.HubSpot.BackfillPreview(c.Request.Context(), orgID)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, out)
}

// StartCRMBackfill copies Warmbly's own deals, tasks and notes into the
// provider once. Idempotent: a second call while one runs is folded into it,
// and copied records are never copied twice.
func (h *Handler) StartCRMBackfill(c *gin.Context) {
	orgID, ok := h.crmReady(c)
	if !ok {
		return
	}
	var body models.CRMBackfillRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}
	if xerr := h.HubSpot.StartBackfill(c.Request.Context(), orgID, &body); xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	h.auditOrg(c, models.AuditActionImport, models.AuditEntityIntegration, nil, nil, map[string]string{"crm_backfill": "hubspot"})
	c.Status(http.StatusAccepted)
}

// verifyHubSpot checks a request HubSpot signed with the app's client secret
// (v3, or v2 on requests that still carry it). HubSpot signs the URL it
// called; behind a proxy that is the public backend URL.
func (h *Handler) verifyHubSpot(c *gin.Context, body []byte) bool {
	scheme := "https"
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = strings.TrimSpace(strings.Split(proto, ",")[0])
	} else if c.Request.TLS == nil {
		scheme = "http"
	}
	candidates := []string{scheme + "://" + c.Request.Host + c.Request.URL.RequestURI()}
	candidates = append(candidates, config.BackendPublicURL()+c.Request.URL.RequestURI())
	v3, ts := c.GetHeader("X-HubSpot-Signature-v3"), c.GetHeader("X-HubSpot-Request-Timestamp")
	v2 := c.GetHeader("X-HubSpot-Signature")
	for _, u := range candidates {
		if v3 != "" && h.HubSpot.VerifySignature(c.Request.Method, u, body, v3, ts, time.Now()) == nil {
			return true
		}
		if v3 == "" && v2 != "" && h.HubSpot.VerifySignatureV2(c.Request.Method, u, body, v2) == nil {
			return true
		}
	}
	return false
}

// readHubSpot reads and verifies a signed HubSpot request body.
func (h *Handler) readHubSpot(c *gin.Context) ([]byte, bool) {
	if h.HubSpot == nil {
		c.Status(http.StatusNotFound)
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		c.Status(http.StatusBadRequest)
		return nil, false
	}
	if !h.verifyHubSpot(c, body) {
		c.Status(http.StatusUnauthorized)
		return nil, false
	}
	return body, true
}

// HubSpotWebhook receives HubSpot's app webhooks. It only queues re-reads, so
// a delivery can at most make a pull come sooner.
func (h *Handler) HubSpotWebhook(c *gin.Context) {
	body, ok := h.readHubSpot(c)
	if !ok {
		return
	}
	if err := h.HubSpot.HandleWebhook(c.Request.Context(), body); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	c.Status(http.StatusNoContent)
}

// hubspotCardRequest is what the Warmbly card in HubSpot sends. HubSpot adds
// portalId and userEmail to the query string and signs the request.
type hubspotCardRequest struct {
	ContactID  string `json:"contact_id"`
	Email      string `json:"email"`
	CampaignID string `json:"campaign_id"`
	Paused     bool   `json:"paused"`
}

func (h *Handler) hubspotCardInput(c *gin.Context) (*hubspotCardRequest, bool) {
	body, ok := h.readHubSpot(c)
	if !ok {
		return nil, false
	}
	// HubSpot appends the portal it signed for; a second value would let the
	// URL name a different one, so exactly one is accepted.
	if len(c.QueryArray("portalId")) != 1 || c.Query("portalId") == "" {
		errx.Handle(c, errx.New(errx.BadRequest, "invalid request"))
		return nil, false
	}
	var req hubspotCardRequest
	if err := json.Unmarshal(body, &req); err != nil || len(req.ContactID) > 32 || len(req.Email) > 320 {
		errx.Handle(c, errx.New(errx.BadRequest, "invalid request"))
		return nil, false
	}
	return &req, true
}

// HubSpotCard returns the Warmbly card for a HubSpot contact record.
func (h *Handler) HubSpotCard(c *gin.Context) {
	req, ok := h.hubspotCardInput(c)
	if !ok {
		return
	}
	out, xerr := h.HubSpot.Card(c.Request.Context(), c.Query("portalId"), req.ContactID, req.Email)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, out)
}

// HubSpotCardEnroll adds the record's contact to a campaign from the card.
// Idempotent: enrolling a lead twice leaves one lead.
func (h *Handler) HubSpotCardEnroll(c *gin.Context) {
	req, ok := h.hubspotCardInput(c)
	if !ok {
		return
	}
	if xerr := h.HubSpot.Enroll(c.Request.Context(), c.Query("portalId"), req.ContactID, req.Email, req.CampaignID, c.Query("userEmail")); xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.Status(http.StatusNoContent)
}

// HubSpotCardPause holds or resumes the contact's campaigns from the card.
// Idempotent: it sets a state rather than toggling one.
func (h *Handler) HubSpotCardPause(c *gin.Context) {
	req, ok := h.hubspotCardInput(c)
	if !ok {
		return
	}
	if xerr := h.HubSpot.SetPaused(c.Request.Context(), c.Query("portalId"), req.ContactID, req.Email, req.Paused); xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.Status(http.StatusNoContent)
}

// hubspotActionRequest is a custom workflow action execution.
type hubspotActionRequest struct {
	CallbackID string `json:"callbackId"`
	Origin     struct {
		PortalID int64 `json:"portalId"`
	} `json:"origin"`
	Object struct {
		ObjectID   int64             `json:"objectId"`
		Properties map[string]string `json:"properties"`
	} `json:"object"`
	InputFields  map[string]any `json:"inputFields"`
	FetchOptions struct {
		Q string `json:"q"`
	} `json:"fetchOptions"`
}

// HubSpotActionEnroll runs the "Add to Warmbly campaign" workflow action.
// A refusal answers FAIL_CONTINUE with the reason, so the workflow goes on.
func (h *Handler) HubSpotActionEnroll(c *gin.Context) {
	body, ok := h.readHubSpot(c)
	if !ok {
		return
	}
	var req hubspotActionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	campaign, _ := req.InputFields["campaign"].(string)
	xerr := h.HubSpot.Enroll(c.Request.Context(), strconv.FormatInt(req.Origin.PortalID, 10),
		strconv.FormatInt(req.Object.ObjectID, 10), req.Object.Properties["email"], campaign, "")
	out := gin.H{"hs_execution_state": "SUCCESS", "warmbly_result": "Added to campaign"}
	if xerr != nil {
		out = gin.H{"hs_execution_state": "FAIL_CONTINUE", "warmbly_result": xerr.Message}
	}
	c.JSON(http.StatusOK, gin.H{"outputFields": out})
}

// HubSpotActionCampaigns lists campaigns for the workflow action's dropdown.
func (h *Handler) HubSpotActionCampaigns(c *gin.Context) {
	body, ok := h.readHubSpot(c)
	if !ok {
		return
	}
	var req hubspotActionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	opts, xerr := h.HubSpot.CampaignChoices(c.Request.Context(), strconv.FormatInt(req.Origin.PortalID, 10), req.FetchOptions.Q)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"options": opts, "searchable": true})
}
