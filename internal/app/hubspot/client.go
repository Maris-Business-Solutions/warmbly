// Package hubspot runs a workspace's CRM on HubSpot: a rate-limited API client,
// the write-through for deals, tasks and notes, the activity log, the pull that
// keeps the local mirror current, and list import.
package hubspot

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/warmbly/warmbly/internal/infrastructure/cache"
)

const apiBase = "https://api.hubapi.com"

// requestsPerWindow stays under HubSpot's per-account burst limit for public
// apps (110 per 10 seconds), shared by every process through Redis.
const (
	requestsPerWindow = 90
	limitWindow       = 10 * time.Second
	maxAttempts       = 4
)

// TokenFunc returns a current access token for the portal.
type TokenFunc func(ctx context.Context) (string, error)

// Client calls one HubSpot portal.
type Client struct {
	portal  string
	token   TokenFunc
	http    *http.Client
	limiter *limiter
}

// NewClient builds a client for a portal; rc may be nil (in-process limiting).
func NewClient(portal string, token TokenFunc, rc *cache.Cache) *Client {
	return &Client{
		portal:  portal,
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
		limiter: newLimiter(rc),
	}
}

// APIError is a non-2xx answer from HubSpot.
type APIError struct {
	Status     int
	Category   string
	Message    string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.Status)
	}
	return "HubSpot: " + msg
}

// Retryable reports a throttle or a HubSpot-side failure.
func (e *APIError) Retryable() bool { return e.Status == http.StatusTooManyRequests || e.Status >= 500 }

// AuthProblem reports a revoked token or a missing scope.
func (e *APIError) AuthProblem() bool {
	return e.Status == http.StatusUnauthorized || (e.Status == http.StatusForbidden && e.Category == "MISSING_SCOPES")
}

// NotFound reports a record that does not exist (or was deleted).
func (e *APIError) NotFound() bool { return e.Status == http.StatusNotFound }

// Conflict reports a create that collided with an existing record.
func (e *APIError) Conflict() bool { return e.Status == http.StatusConflict }

// AsAPIError unwraps an *APIError.
func AsAPIError(err error) (*APIError, bool) {
	var ae *APIError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}

// do sends one request, waiting on the shared rate limit and retrying throttles
// and server errors with the delay HubSpot asks for.
func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return err
		}
	}
	var lastErr error
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if err := c.limiter.wait(ctx, c.portal); err != nil {
			return err
		}
		token, err := c.token(ctx)
		if err != nil {
			return err
		}
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, apiBase+path, reader)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			if !sleepCtx(ctx, backoff(attempt)) {
				return ctx.Err()
			}
			continue
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out != nil && len(raw) > 0 && resp.StatusCode != http.StatusNoContent {
				return json.Unmarshal(raw, out)
			}
			return nil
		}
		apiErr := parseAPIError(resp, raw)
		lastErr = apiErr
		if !apiErr.Retryable() {
			return apiErr
		}
		wait := apiErr.RetryAfter
		if wait <= 0 {
			wait = backoff(attempt)
		}
		if wait > 15*time.Second {
			return apiErr
		}
		if !sleepCtx(ctx, wait) {
			return ctx.Err()
		}
	}
	return lastErr
}

func parseAPIError(resp *http.Response, raw []byte) *APIError {
	e := &APIError{Status: resp.StatusCode}
	var body struct {
		Message  string `json:"message"`
		Category string `json:"category"`
		Errors   []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if json.Unmarshal(raw, &body) == nil {
		e.Category = body.Category
		e.Message = body.Message
		if len(body.Errors) > 0 && body.Errors[0].Message != "" {
			e.Message = body.Errors[0].Message
		}
	}
	e.Message = cleanMessage(e.Message)
	if ra := resp.Header.Get("Retry-After"); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil {
			e.RetryAfter = time.Duration(secs) * time.Second
		}
	}
	if e.Status == http.StatusTooManyRequests && e.RetryAfter == 0 {
		e.RetryAfter = 2 * time.Second
	}
	return e
}

// cleanMessage trims HubSpot's correlation ids and keeps the sentence a person
// can act on.
func cleanMessage(m string) string {
	m = strings.TrimSpace(m)
	if i := strings.Index(m, "correlationId"); i > 0 {
		m = strings.TrimRight(strings.TrimSpace(m[:i]), ",.(")
	}
	if len(m) > 400 {
		m = m[:400]
	}
	return m
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

// limiter is a fixed window per portal, shared through Redis when available
// and per process otherwise. A Redis error fails open; HubSpot's own 429 is the
// backstop.
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

func (l *limiter) wait(ctx context.Context, portal string) error {
	for {
		ok, retryIn := l.take(ctx, portal)
		if ok {
			return nil
		}
		if !sleepCtx(ctx, retryIn) {
			return ctx.Err()
		}
	}
}

func (l *limiter) take(ctx context.Context, portal string) (bool, time.Duration) {
	now := time.Now()
	slot := now.UnixNano() / int64(limitWindow)
	untilNext := time.Duration(int64(limitWindow)*(slot+1) - now.UnixNano())
	if l.rc != nil {
		key := fmt.Sprintf("hubspot:rl:%s:%d", portal, slot)
		n, err := l.rc.Incr(ctx, key).Result()
		if err == nil {
			if n == 1 {
				l.rc.Expire(ctx, key, limitWindow+time.Second)
			}
			return n <= requestsPerWindow, untilNext
		}
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	w := l.local[portal]
	if w == nil || now.Sub(w.start) >= limitWindow {
		w = &window{start: now}
		l.local[portal] = w
	}
	if w.count >= requestsPerWindow/2 {
		return false, limitWindow - now.Sub(w.start)
	}
	w.count++
	return true, 0
}
