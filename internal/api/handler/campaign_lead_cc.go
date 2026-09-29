package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// leadCCResponse is what the read and the write answer with.
type leadCCResponse struct {
	CampaignID string                  `json:"campaign_id"`
	ContactID  string                  `json:"contact_id"`
	CC         []models.CampaignLeadCC `json:"cc"`
}

func leadCCOK(c *gin.Context, campaignID, contactID uuid.UUID, cc []models.CampaignLeadCC) {
	if cc == nil {
		cc = []models.CampaignLeadCC{}
	}
	c.JSON(http.StatusOK, leadCCResponse{CampaignID: campaignID.String(), ContactID: contactID.String(), CC: cc})
}

// GetCampaignLeadCC lists the contacts copied on every email to one lead.
//
// GET /campaigns/:id/leads/:contactId/cc
func (h *Handler) GetCampaignLeadCC(c *gin.Context) {
	orgID, campaignID, contactID, ok := leadHoldParams(c)
	if !ok {
		return
	}
	cc, xerr := h.CampaignService.ListLeadCC(c.Request.Context(), orgID, campaignID, contactID)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	leadCCOK(c, campaignID, contactID, cc)
}

// SetCampaignLeadCC replaces the contacts copied on one lead.
//
// No Idempotency-Key: the body is the whole list, so a retry lands on the
// same state.
//
// PUT /campaigns/:id/leads/:contactId/cc
func (h *Handler) SetCampaignLeadCC(c *gin.Context) {
	orgID, campaignID, contactID, ok := leadHoldParams(c)
	if !ok {
		return
	}
	var req models.SetCampaignLeadCC
	if err := c.ShouldBindJSON(&req); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	cc, xerr := h.CampaignService.SetLeadCC(c.Request.Context(), orgID, campaignID, contactID, req.ContactIDs)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	h.auditOrg(c, models.AuditActionUpdate, models.AuditEntityCampaignLead, &contactID, nil, map[string]string{
		"campaign_id": campaignID.String(),
		"cc":          strconv.Itoa(len(cc)),
	})
	leadCCOK(c, campaignID, contactID, cc)
}

// SuggestCampaignLeadCC offers the lead's likely colleagues to copy.
//
// GET /campaigns/:id/leads/:contactId/cc/suggestions
func (h *Handler) SuggestCampaignLeadCC(c *gin.Context) {
	orgID, campaignID, contactID, ok := leadHoldParams(c)
	if !ok {
		return
	}
	out, xerr := h.CampaignService.SuggestLeadCC(c.Request.Context(), orgID, campaignID, contactID)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}
