package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

func (h *Handler) PoolLinkWarmupStats(c *gin.Context) {
	h.poolLinkWarmupReport(c, false)
}

func (h *Handler) PoolLinkWarmupPlacement(c *gin.Context) {
	h.poolLinkWarmupReport(c, true)
}

func (h *Handler) poolLinkWarmupReport(c *gin.Context, placement bool) {
	inst := middleware.GetPoolLinkInstance(c)
	if inst == nil {
		errx.JSON(c, errx.ErrUnauthorized)
		return
	}
	var req models.PoolLinkWarmupReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errx.JSON(c, errx.InvalidBody(err))
		return
	}
	if placement {
		report, xerr := h.PoolLinkService.WarmupPlacementData(c.Request.Context(), inst, req)
		if xerr != nil {
			errx.JSON(c, xerr)
			return
		}
		c.JSON(http.StatusOK, report)
		return
	}
	stats, xerr := h.PoolLinkService.WarmupStats(c.Request.Context(), inst, req)
	if xerr != nil {
		errx.JSON(c, xerr)
		return
	}
	c.JSON(http.StatusOK, stats)
}
