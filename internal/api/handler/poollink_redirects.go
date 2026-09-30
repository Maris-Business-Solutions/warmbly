package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/app/sendingdomain"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// Root redirects Warmbly Cloud serves for a linked self-hosted instance. Every
// route is scoped to the calling instance's own rows.

func (h *Handler) poolLinkRedirectCaller(c *gin.Context) (*models.PoolLinkInstance, bool) {
	inst := middleware.GetPoolLinkInstance(c)
	if inst == nil {
		errx.JSON(c, errx.ErrUnauthorized)
		return nil, false
	}
	if h.SendingDomainService == nil {
		errx.JSON(c, errx.NewWithIdentifier(errx.Conflict, sendingdomain.ErrIDCloudUnavailable, "Warmbly Cloud does not serve redirects right now."))
		return nil, false
	}
	return inst, true
}

// PoolLinkListRedirects is GET /pool-link/instance/redirects.
func (h *Handler) PoolLinkListRedirects(c *gin.Context) {
	inst, ok := h.poolLinkRedirectCaller(c)
	if !ok {
		return
	}
	list, xerr := h.SendingDomainService.LinkedList(c.Request.Context(), inst)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// PoolLinkGetRedirect is GET /pool-link/instance/redirects/:domain: the last verdict, without a new check.
func (h *Handler) PoolLinkGetRedirect(c *gin.Context) {
	inst, ok := h.poolLinkRedirectCaller(c)
	if !ok {
		return
	}
	r, xerr := h.SendingDomainService.LinkedGet(c.Request.Context(), inst, c.Param("domain"))
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.JSON(http.StatusOK, r)
}

// PoolLinkPutRedirect is PUT /pool-link/instance/redirects/:domain. Idempotent:
// the domain keeps its TXT value, so a retry changes nothing already in place.
func (h *Handler) PoolLinkPutRedirect(c *gin.Context) {
	inst, ok := h.poolLinkRedirectCaller(c)
	if !ok {
		return
	}
	var req models.DomainRedirectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	r, xerr := h.SendingDomainService.LinkedSet(c.Request.Context(), inst, c.Param("domain"), req)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.JSON(http.StatusOK, r)
}

// PoolLinkVerifyRedirect is POST /pool-link/instance/redirects/:domain/verify: check DNS and reach now.
func (h *Handler) PoolLinkVerifyRedirect(c *gin.Context) {
	inst, ok := h.poolLinkRedirectCaller(c)
	if !ok {
		return
	}
	r, xerr := h.SendingDomainService.LinkedVerify(c.Request.Context(), inst, c.Param("domain"))
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.JSON(http.StatusOK, r)
}

// PoolLinkDeleteRedirect is DELETE /pool-link/instance/redirects/:domain.
func (h *Handler) PoolLinkDeleteRedirect(c *gin.Context) {
	inst, ok := h.poolLinkRedirectCaller(c)
	if !ok {
		return
	}
	if xerr := h.SendingDomainService.LinkedDelete(c.Request.Context(), inst, c.Param("domain")); xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.Status(http.StatusNoContent)
}
