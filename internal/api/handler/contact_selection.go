package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// resolveContactSelection turns a bulk request's selection into the contact ids
// the action applies to: the explicit list as sent, or everything matching the
// search behind the dashboard's "select all matching". It writes the error
// response itself and returns ok=false when the selection is empty, too large,
// or malformed, so callers can `if !ok { return }`.
func (h *Handler) resolveContactSelection(c *gin.Context, orgID uuid.UUID, sel models.ContactSelection) ([]string, bool) {
	if !sel.All {
		if len(sel.Contacts) == 0 {
			errx.Handle(c, errx.New(errx.BadRequest, "no contacts provided"))
			return nil, false
		}
		if len(sel.Contacts) > models.MaxContactBatchIDs {
			errx.Handle(c, errx.NewWithIdentifier(errx.BadRequest, "too_many_contacts",
				fmt.Sprintf("too many contacts, maximum is %d per batch", models.MaxContactBatchIDs)))
			return nil, false
		}
		return sel.Contacts, true
	}

	if h.ContactService == nil {
		errx.Handle(c, errx.InternalError())
		return nil, false
	}
	ids, xerr := h.ContactService.ResolveSelection(c.Request.Context(), orgID, sel)
	if xerr != nil {
		errx.Handle(c, xerr)
		return nil, false
	}
	if len(ids) == 0 {
		errx.Handle(c, errx.New(errx.BadRequest, "no contacts match that selection"))
		return nil, false
	}
	return ids, true
}
