package handler

import (
	"context"
	"net/url"
	"strings"
	"unicode"

	"github.com/warmbly/warmbly/internal/config"
)

// Stripe returns only to the request's trusted dashboard, preserving its path.
func billingReturnURL(ctx context.Context, raw, fallbackPath string) string {
	base := strings.TrimRight(config.DashboardBaseURL(ctx), "/")
	raw = strings.TrimSpace(raw)
	b, err := url.Parse(base)
	if err != nil || (b.Scheme != "https" && b.Scheme != "http") || b.Host == "" || b.User != nil {
		return ""
	}
	fallback := base + fallbackPath
	if raw == "" || strings.ContainsRune(raw, '\\') || strings.ContainsFunc(raw, unicode.IsControl) {
		return fallback
	}
	u, err := url.Parse(raw)
	if err != nil || u.User != nil {
		return fallback
	}
	if u.IsAbs() && u.Scheme == b.Scheme && u.Host == b.Host &&
		(b.Path == "" || u.Path == b.Path || strings.HasPrefix(u.Path, b.Path+"/")) {
		return raw
	}
	if u.Scheme == "" && u.Host == "" && strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return base + raw
	}
	return fallback
}
