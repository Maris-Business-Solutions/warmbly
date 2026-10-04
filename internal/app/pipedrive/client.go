// Package pipedrive runs a workspace's CRM on Pipedrive: a rate-limited API
// client, the write-through for deals, activities and notes, the activity log,
// the pull and webhooks that keep the local mirror current, and filter import.
package pipedrive

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/warmbly/warmbly/internal/infrastructure/cache"
)

// Pipedrive allows at least 80 requests per two seconds per token on every
// plan and 10 searches; Warmbly stays well under both, shared through Redis.
const (
	requestsPerWindow = 30
	searchesPerWindow = 4
	limitWindow       = 2 * time.Second
	maxAttempts       = 4
)

// TokenFunc returns a current access token for the company.
type TokenFunc func(ctx context.Context) (string, error)

// Client calls one Pipedrive company.
type Client struct {
	company string
	base    string
	token   TokenFunc
	http    *http.Client
	limiter *limiter
}

// NewClient builds a client for a company host; rc may be nil (in-process
// limiting).
func NewClient(company, base string, token TokenFunc, rc *cache.Cache) *Client {
	return &Client{
		company: company,
		base:    strings.TrimRight(base, "/"),
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
		limiter: newLimiter(rc),
	}
}

// APIError is a non-2xx answer from Pipedrive.
type APIError struct {
	Status     int
	Message    string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.Status)
	}
	return "Pipedrive: " + msg
}

// Retryable reports a throttle or a Pipedrive-side failure.
func (e *APIError) Retryable() bool { return e.Status == http.StatusTooManyRequests || e.Status >= 500 }

// AuthProblem reports a revoked token or a missing scope.
func (e *APIError) AuthProblem() bool {
	return e.Status == http.StatusUnauthorized || e.Status == http.StatusForbidden
}

// NotFound reports a record that does not exist (or was deleted).
func (e *APIError) NotFound() bool {
	return e.Status == http.StatusNotFound || e.Status == http.StatusGone
}

// AsAPIError unwraps an *APIError.
func AsAPIError(err error) (*APIError, bool) {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

// envelope is every Pipedrive answer: data plus paging.
type envelope struct {
	Success        bool            `json:"success"`
	Data           json.RawMessage `json:"data"`
	Error          string          `json:"error"`
	ErrorInfo      string          `json:"error_info"`
	AdditionalData struct {
		NextCursor *string `json:"next_cursor"`
		Pagination *struct {
			MoreItems bool `json:"more_items_in_collection"`
			NextStart int  `json:"next_start"`
		} `json:"pagination"`
	} `json:"additional_data"`
}

// call sends one request and decodes data into out. search requests take the
// tighter search budget.
func (c *Client) call(ctx context.Context, method, path string, q url.Values, body, out any) (*envelope, error) {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return nil, err
		}
	}
	target := c.base + path
	if len(q) > 0 {
		target += "?" + q.Encode()
	}
	search := strings.HasSuffix(path, "/search")
	// A create Pipedrive may have saved is never sent twice: it has no
	// idempotency key, so a POST is retried only when Pipedrive refused it.
	create := method == http.MethodPost
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := c.limiter.wait(ctx, c.company, search); err != nil {
			return nil, err
		}
		token, err := c.token(ctx)
		if err != nil {
			return nil, err
		}
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, target, reader)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			if create {
				return nil, err
			}
			if !sleepCtx(ctx, backoff(attempt)) {
				return nil, ctx.Err()
			}
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			env := &envelope{}
			if len(raw) > 0 {
				if err := json.Unmarshal(raw, env); err != nil {
					return nil, err
				}
			}
			if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
				if err := json.Unmarshal(env.Data, out); err != nil {
					return nil, err
				}
			}
			return env, nil
		}
		apiErr := parseAPIError(resp, raw)
		lastErr = apiErr
		if apiErr.Status == http.StatusBadRequest && q.Get("include_fields") == "marketing_status" {
			// A company without Pipedrive Campaigns has no marketing status:
			// remember it and read without.
			c.setMarketingOff()
			q.Del("include_fields")
			target = c.base + path
			if len(q) > 0 {
				target += "?" + q.Encode()
			}
			continue
		}
		if !apiErr.Retryable() || (create && apiErr.Status != http.StatusTooManyRequests) {
			return nil, apiErr
		}
		wait := apiErr.RetryAfter
		if wait <= 0 {
			wait = backoff(attempt)
		}
		if wait > 15*time.Second {
			return nil, apiErr
		}
		if !sleepCtx(ctx, wait) {
			return nil, ctx.Err()
		}
	}
	return nil, lastErr
}

func parseAPIError(resp *http.Response, raw []byte) *APIError {
	e := &APIError{Status: resp.StatusCode}
	var body struct {
		Error     string `json:"error"`
		ErrorInfo string `json:"error_info"`
		Message   string `json:"message"`
	}
	if json.Unmarshal(raw, &body) == nil {
		e.Message = firstNonEmpty(body.Error, body.Message)
	}
	e.Message = strings.TrimSpace(e.Message)
	if len(e.Message) > 400 {
		e.Message = e.Message[:400]
	}
	for _, h := range []string{"Retry-After", "X-RateLimit-Reset"} {
		if v := resp.Header.Get(h); v != "" {
			if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
				e.RetryAfter = time.Duration(max(secs, 1)) * time.Second
				break
			}
		}
	}
	if e.Status == http.StatusTooManyRequests && e.RetryAfter == 0 {
		e.RetryAfter = 2 * time.Second
	}
	return e
}

func backoff(attempt int) time.Duration {
	return time.Duration(500*(1<<attempt)) * time.Millisecond
}

func sleepCtx(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}

// listV2 pages through a v2 collection by cursor, up to max items (0: all).
func listV2[T any](ctx context.Context, c *Client, path string, q url.Values, max int) ([]T, error) {
	if q == nil {
		q = url.Values{}
	}
	limit := 500
	if max > 0 && max < limit {
		limit = max
	}
	q.Set("limit", strconv.Itoa(limit))
	var out []T
	for {
		var page []T
		env, err := c.call(ctx, http.MethodGet, path, q, nil, &page)
		if err != nil {
			return out, err
		}
		out = append(out, page...)
		next := env.AdditionalData.NextCursor
		if next == nil || *next == "" || len(page) == 0 || (max > 0 && len(out) >= max) {
			break
		}
		q.Set("cursor", *next)
	}
	if max > 0 && len(out) > max {
		out = out[:max]
	}
	return out, nil
}

// listV1 pages through a v1 collection by offset, up to max items (0: all).
func listV1[T any](ctx context.Context, c *Client, path string, q url.Values, max int) ([]T, error) {
	if q == nil {
		q = url.Values{}
	}
	q.Set("limit", "500")
	start := 0
	var out []T
	for {
		q.Set("start", strconv.Itoa(start))
		var page []T
		env, err := c.call(ctx, http.MethodGet, path, q, nil, &page)
		if err != nil {
			return out, err
		}
		out = append(out, page...)
		pg := env.AdditionalData.Pagination
		if pg == nil || !pg.MoreItems || len(page) == 0 || (max > 0 && len(out) >= max) {
			break
		}
		start = pg.NextStart
	}
	if max > 0 && len(out) > max {
		out = out[:max]
	}
	return out, nil
}

// limiter is a fixed window per company, shared through Redis when available
// and per process otherwise. A Redis error fails open; Pipedrive's own 429 is
// the backstop.
type limiter struct {
	rc    *cache.Cache
	mu    sync.Mutex
	local map[string]*window
}

type window struct {
	start time.Time
	count int
}

func newLimiter(rc *cache.Cache) *limiter {
	return &limiter{rc: rc, local: map[string]*window{}}
}

func (l *limiter) wait(ctx context.Context, company string, search bool) error {
	key, budget := company, requestsPerWindow
	if search {
		key, budget = company+":search", searchesPerWindow
	}
	for {
		ok, retryIn := l.take(ctx, key, budget)
		if ok {
			return nil
		}
		if !sleepCtx(ctx, retryIn) {
			return ctx.Err()
		}
	}
}

func (l *limiter) take(ctx context.Context, key string, budget int) (bool, time.Duration) {
	now := time.Now()
	slot := now.UnixNano() / int64(limitWindow)
	untilNext := time.Duration(int64(limitWindow)*(slot+1) - now.UnixNano())
	if l.rc != nil {
		k := fmt.Sprintf("pipedrive:rl:%s:%d", key, slot)
		n, err := l.rc.Incr(ctx, k).Result()
		if err == nil {
			if n == 1 {
				l.rc.Expire(ctx, k, limitWindow+time.Second)
			}
			return n <= int64(budget), untilNext
		}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.local[key]
	if w == nil || now.Sub(w.start) >= limitWindow {
		w = &window{start: now}
		l.local[key] = w
	}
	if w.count >= max(budget/2, 1) {
		return false, limitWindow - now.Sub(w.start)
	}
	w.count++
	return true, 0
}
