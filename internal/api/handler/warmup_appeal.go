package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

type submitWarmupAppealRequest struct {
	Reason string `json:"reason"`
}

// GetWarmupBanStatus returns whether a mailbox is blocked from warmup, why, and
// whether the owner can appeal. Powers the dashboard ban banner.
func (h *Handler) GetWarmupBanStatus(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.JSON(c, errx.ErrNoOrganization)
		return
	}
	accountID, perr := uuid.Parse(c.Param("id"))
	if perr != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "invalid email account id"))
		return
	}

	status, xerr := h.WarmupService.GetBanStatus(c.Request.Context(), *orgID, accountID)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.JSON(http.StatusOK, status)
}

// SubmitWarmupAppeal lets a member who manages mailboxes appeal a warmup ban with a reason.
func (h *Handler) SubmitWarmupAppeal(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.JSON(c, errx.ErrNoOrganization)
		return
	}
	userID, err := middleware.GetUserUUID(c)
	if err != nil {
		errx.JSON(c, errx.ErrUnauthorized)
		return
	}
	accountID, perr := uuid.Parse(c.Param("id"))
	if perr != nil {
		errx.JSON(c, errx.New(errx.BadRequest, "invalid email account id"))
		return
	}

	var req submitWarmupAppealRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		errx.JSON(c, errx.InvalidBody(bindErr))
		return
	}

	appealID, xerr := h.WarmupService.SubmitAppeal(c.Request.Context(), *orgID, userID, accountID, req.Reason)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}

	h.auditOrg(c, models.AuditActionCreate, models.AuditEntityEmailAccount, &accountID, map[string]string{"warmup_appeal": "submitted"}, nil)
	c.JSON(http.StatusOK, gin.H{"appeal_id": appealID})
}
