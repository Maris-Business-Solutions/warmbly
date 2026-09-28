package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
)

// The imports list pages between 1 and contactImportListMax, 50 by default.
const (
	contactImportListDefault = 50
	contactImportListMax     = 100
)

var errContactImportListLimit = errx.New(errx.BadRequest, "limit must be between 1 and 100")

// CreateContactImport is POST /contacts/imports: upload a file once and get a
// draft back with the preview the column mapper needs.
func (h *Handler) CreateContactImport(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.Handle(c, errx.ErrNoOrganization)
		return
	}
	userID, err := middleware.GetUserUUID(c)
	if err != nil {
		errx.Handle(c, errx.ErrUser)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxImportUploadBytes)
	file, header, ferr := c.Request.FormFile("file")
	if ferr != nil {
		errx.Handle(c, importFileErr(ferr))
		return
	}
	defer file.Close()

	imp, xerr := h.ContactImportService.Create(c.Request.Context(), *orgID, userID, file, header.Filename)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusCreated, imp)
}

// ListContactImports is GET /contacts/imports, newest first.
func (h *Handler) ListContactImports(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.Handle(c, errx.ErrNoOrganization)
		return
	}
	limit := contactImportListDefault
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > contactImportListMax {
			errx.Handle(c, errContactImportListLimit)
			return
		}
		limit = n
	}
	list, xerr := h.ContactImportService.List(c.Request.Context(), *orgID, c.Query("cursor"), limit)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, list)
}

// GetContactImport is GET /contacts/imports/:id.
func (h *Handler) GetContactImport(c *gin.Context) {
	orgID, id, ok := importID(c)
	if !ok {
		return
	}
	imp, xerr := h.ContactImportService.Get(c.Request.Context(), orgID, id)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, imp)
}

// SaveContactImportDraft is PATCH /contacts/imports/:id: autosaves a draft's
// mapping and options so a reload resumes it. Idempotent by nature.
func (h *Handler) SaveContactImportDraft(c *gin.Context) {
	orgID, id, ok := importID(c)
	if !ok {
		return
	}
	var opts models.ContactImportCommit
	if err := c.ShouldBindJSON(&opts); err != nil {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}
	imp, xerr := h.ContactImportService.SaveDraft(c.Request.Context(), orgID, id, &opts)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, imp)
}

// AnalyzeContactImport is POST /contacts/imports/:id/analyze: what the draft
// would do under a mapping, over the whole file, writing nothing.
func (h *Handler) AnalyzeContactImport(c *gin.Context) {
	orgID, id, ok := importID(c)
	if !ok {
		return
	}
	userID, err := middleware.GetUserUUID(c)
	if err != nil {
		errx.Handle(c, errx.ErrUser)
		return
	}
	var req models.ContactImportAnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}
	analysis, xerr := h.ContactImportService.Analyze(c.Request.Context(), orgID, userID, id, &req)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, analysis)
}

// StartContactImport is POST /contacts/imports/:id/start. Starting an import
// that already started returns it unchanged, so a retry is safe.
func (h *Handler) StartContactImport(c *gin.Context) {
	orgID, id, ok := importID(c)
	if !ok {
		return
	}
	userID, err := middleware.GetUserUUID(c)
	if err != nil {
		errx.Handle(c, errx.ErrUser)
		return
	}
	var opts models.ContactImportCommit
	if err := c.ShouldBindJSON(&opts); err != nil {
		errx.Handle(c, errx.InvalidBody(err))
		return
	}
	imp, started, xerr := h.ContactImportService.Start(c.Request.Context(), orgID, userID, id, &opts)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	if started {
		h.auditOrg(c, models.AuditActionImport, models.AuditEntityContact, nil, nil, map[string]string{
			"import_id": imp.ID.String(),
			"total":     fmt.Sprintf("%d", imp.Total),
			"filename":  imp.Filename,
			"dedup":     string(opts.Dedup),
		})
	}
	c.JSON(http.StatusOK, imp)
}

// CancelContactImport is POST /contacts/imports/:id/cancel. Rows already
// written stay written.
func (h *Handler) CancelContactImport(c *gin.Context) {
	orgID, id, ok := importID(c)
	if !ok {
		return
	}
	imp, xerr := h.ContactImportService.Cancel(c.Request.Context(), orgID, id)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.JSON(http.StatusOK, imp)
}

// DownloadContactImportFailures is GET /contacts/imports/:id/failed.csv.
func (h *Handler) DownloadContactImportFailures(c *gin.Context) {
	orgID, id, ok := importID(c)
	if !ok {
		return
	}
	data, name, xerr := h.ContactImportService.FailedCSV(c.Request.Context(), orgID, id)
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+strings.ReplaceAll(name, `"`, "")+`"`)
	c.Data(http.StatusOK, "text/csv; charset=utf-8", data)
}
