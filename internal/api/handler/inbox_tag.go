package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/config"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/repository"
	"github.com/warmbly/warmbly/internal/utils/paging"
)

// The phase-1 review surface. Automatic tagging writes labels and a score and
// nothing else, and the point of the phase is that a person watches it decide
// for a week before it is allowed to act. That is only possible if what it
// decided, and how sure it was, is on a screen.
//
// GET /analytics/inbox-tagging?limit=&cursor=&needs_review=

type inboxTagRow struct {
	ID               string          `json:"id"`
	MessageID        string          `json:"message_id"`
	ThreadID         string          `json:"thread_id"`
	Kind             string          `json:"kind"`
	KindConfidence   float64         `json:"kind_confidence"`
	KindSource       string          `json:"kind_source"`
	Intent           string          `json:"intent"`
	IntentConfidence float64         `json:"intent_confidence"`
	Relevance        int             `json:"relevance"`
	Priority         string          `json:"priority"`
	NeedsReview      bool            `json:"needs_review"`
	ReviewReason     string          `json:"review_reason"`
	Labels           []string        `json:"labels"`
	Answers          json.RawMessage `json:"answers"`
	Model            string          `json:"model"`
	InputTokens      int             `json:"input_tokens"`
	// Actions is what the workspace's switches let this verdict do.
	Actions []string `json:"actions"`
	// ReturnDate is the out-of-office return date the model was asked to
	// confirm (YYYY-MM-DD), null when it was not asked.
	ReturnDate *string `json:"return_date"`
	CreatedAt  string  `json:"created_at"`
}

type inboxTagReviewResponse struct {
	// Enabled says whether the feature is switched on for this instance, so the
	// page can explain an empty list rather than implying nothing was found.
	Enabled    bool                     `json:"enabled"`
	Data       []inboxTagRow            `json:"data"`
	Total      int                      `json:"total"`
	Summary    inboxTagReviewSummary    `json:"summary"`
	Pagination inboxTagReviewPagination `json:"pagination"`
}

type inboxTagReviewSummary struct {
	Total       int `json:"total"`
	NeedsReview int `json:"needs_review"`
	FromOffline int `json:"from_offline"`
	Acted       int `json:"acted"`
}

type inboxTagReviewPagination struct {
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

func (h *Handler) GetInboxTaggingReview(c *gin.Context) {
	orgID := middleware.GetOrganizationID(c)
	if orgID == nil {
		errx.Handle(c, errx.New(errx.BadRequest, "no organization selected"))
		return
	}

	enabled := config.InboxTaggingEnabled()
	if h.InboxTagRepo == nil {
		c.JSON(http.StatusOK, inboxTagReviewResponse{Enabled: enabled, Data: []inboxTagRow{}})
		return
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil || limit < 1 || limit > 200 {
		errx.Handle(c, errx.New(errx.BadRequest, "limit must be between 1 and 200"))
		return
	}
	offset, xerr := paging.DecodeOffsetCursor(c.Query("cursor"))
	if xerr != nil {
		errx.Handle(c, xerr)
		return
	}
	needsReviewOnly, err := strconv.ParseBool(c.DefaultQuery("needs_review", "false"))
	if err != nil {
		errx.Handle(c, errx.New(errx.BadRequest, "needs_review must be true or false"))
		return
	}

	rows, total, err := h.InboxTagRepo.ListForReview(c.Request.Context(), *orgID, limit, offset, needsReviewOnly)
	if err != nil {
		errx.Handle(c, errx.InternalError())
		return
	}

	out := make([]inboxTagRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, toTagRow(r))
	}
	summary, err := h.InboxTagRepo.ReviewSummary(c.Request.Context(), *orgID)
	if err != nil {
		errx.Handle(c, errx.InternalError())
		return
	}
	hasMore := offset+len(rows) < total
	var nextCursor *string
	if hasMore {
		nextCursor = paging.EncodeOffset(offset + len(rows))
	}
	c.JSON(http.StatusOK, inboxTagReviewResponse{
		Enabled:    enabled,
		Data:       out,
		Total:      total,
		Summary:    inboxTagReviewSummary{Total: summary.Total, NeedsReview: summary.NeedsReview, FromOffline: summary.FromOffline, Acted: summary.Acted},
		Pagination: inboxTagReviewPagination{NextCursor: nextCursor, HasMore: hasMore},
	})
}

func toTagRow(r repository.InboxTagResult) inboxTagRow {
	labels := r.Labels
	if labels == nil {
		labels = []string{}
	}
	actions := r.Actions
	if actions == nil {
		actions = []string{}
	}
	answers := r.Answers
	if len(answers) == 0 {
		answers = json.RawMessage(`{}`)
	}
	var returnDate *string
	if r.ReturnDate != nil {
		day := r.ReturnDate.UTC().Format("2006-01-02")
		returnDate = &day
	}
	return inboxTagRow{
		ID:               r.ID.String(),
		MessageID:        r.MessageID,
		ThreadID:         r.ThreadID,
		Kind:             r.Kind,
		KindConfidence:   r.KindConfidence,
		KindSource:       r.KindSource,
		Intent:           r.Intent,
		IntentConfidence: r.IntentConfidence,
		Relevance:        r.Relevance,
		Priority:         r.Priority,
		NeedsReview:      r.NeedsReview,
		ReviewReason:     r.ReviewReason,
		Labels:           labels,
		Actions:          actions,
		Answers:          answers,
		Model:            r.Model,
		InputTokens:      r.InputTokens,
		ReturnDate:       returnDate,
		CreatedAt:        r.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}
