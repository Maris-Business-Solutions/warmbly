// Package slackapp is the Warmbly app for Slack: the assistant in DMs, threads
// and the assistant pane, notification cards, the unified inbox mirrored into a
// channel, and the App Home. Every inbound request is signature-verified before
// it is read, and anything the bot does for a Slack member runs as the linked
// Warmbly member with that member's current permissions.
package slackapp

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
	"time"
)

const defaultAPIBase = "https://slack.com/api/"

// ErrRateLimited is returned when Slack asks for a longer wait than a request
// is allowed to spend.
var ErrRateLimited = errors.New("slack rate limited")

// APIError is a Slack {ok:false,error:...} answer.
type APIError struct {
	Method string
	Code   string
}

func (e *APIError) Error() string { return "slack " + e.Method + ": " + e.Code }

// IsAPIError reports whether err is a Slack answer carrying one of codes.
func IsAPIError(err error, codes ...string) bool {
	var ae *APIError
	if !errors.As(err, &ae) {
		return false
	}
	for _, c := range codes {
		if ae.Code == c {
			return true
		}
	}
	return len(codes) == 0
}

// Block is one Block Kit element; kept as a map so any block type is expressible.
type Block = map[string]any

// Message is a chat.postMessage / chat.update / chat.postEphemeral payload.
type Message struct {
	Channel     string  `json:"channel,omitempty"`
	TS          string  `json:"ts,omitempty"`
	ThreadTS    string  `json:"thread_ts,omitempty"`
	User        string  `json:"user,omitempty"`
	Text        string  `json:"text"`
	Blocks      []Block `json:"blocks,omitempty"`
	UnfurlLinks bool    `json:"unfurl_links"`
	UnfurlMedia bool    `json:"unfurl_media"`
}

// Client calls the Slack Web API with a bot token passed per call.
type Client struct {
	http *http.Client
	base string
	// maxWait bounds how long one call sleeps for a 429 Retry-After.
	maxWait time.Duration
}

func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 15 * time.Second}, base: defaultAPIBase, maxWait: 5 * time.Second}
}

// callJSON POSTs a JSON body (write methods).
func (c *Client) callJSON(ctx context.Context, token, method string, payload, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.do(ctx, token, method, "application/json; charset=utf-8", body, out)
}

// callForm POSTs a form body (read methods, which do not all accept JSON).
func (c *Client) callForm(ctx context.Context, token, method string, form url.Values, out any) error {
	return c.do(ctx, token, method, "application/x-www-form-urlencoded", []byte(form.Encode()), out)
}

func (c *Client) do(ctx context.Context, token, method, contentType string, body []byte, out any) error {
	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+method, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", contentType)
		resp, err := c.http.Do(req)
		if err != nil {
			return fmt.Errorf("slack %s: %w", method, err)
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
		_ = resp.Body.Close()

		if resp.StatusCode == http.StatusTooManyRequests {
			wait := retryAfter(resp.Header.Get("Retry-After"))
			if attempt >= 2 || wait > c.maxWait {
				return ErrRateLimited
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
			}
			continue
		}
		var env struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
		}
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("slack %s: HTTP %d", method, resp.StatusCode)
		}
		if !env.OK {
			code := env.Error
			if code == "" {
				code = "HTTP " + strconv.Itoa(resp.StatusCode)
			}
			if code == "ratelimited" {
				return ErrRateLimited
			}
			return &APIError{Method: method, Code: code}
		}
		if out != nil {
			return json.Unmarshal(raw, out)
		}
		return nil
	}
}

func retryAfter(v string) time.Duration {
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 0 {
		return time.Second
	}
	return time.Duration(n) * time.Second
}

// PostMessage posts and returns the message ts.
func (c *Client) PostMessage(ctx context.Context, token string, m Message) (string, error) {
	ts, _, err := c.PostMessageIn(ctx, token, m)
	return ts, err
}

// PostMessageIn posts and returns the ts and the resolved channel id, which
// differs from m.Channel when that was a "#name".
func (c *Client) PostMessageIn(ctx context.Context, token string, m Message) (string, string, error) {
	var out struct {
		TS      string `json:"ts"`
		Channel string `json:"channel"`
	}
	if err := c.callJSON(ctx, token, "chat.postMessage", m, &out); err != nil {
		return "", "", err
	}
	return out.TS, out.Channel, nil
}

func (c *Client) UpdateMessage(ctx context.Context, token string, m Message) error {
	return c.callJSON(ctx, token, "chat.update", m, nil)
}

func (c *Client) PostEphemeral(ctx context.Context, token string, m Message) error {
	return c.callJSON(ctx, token, "chat.postEphemeral", m, nil)
}

// OpenDM opens (or finds) the bot's DM with a member and returns its channel id.
func (c *Client) OpenDM(ctx context.Context, token, userID string) (string, error) {
	var out struct {
		Channel struct {
			ID string `json:"id"`
		} `json:"channel"`
	}
	if err := c.callJSON(ctx, token, "conversations.open", map[string]any{"users": userID}, &out); err != nil {
		return "", err
	}
	return out.Channel.ID, nil
}

type conversation struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IsPrivate   bool   `json:"is_private"`
	IsMember    bool   `json:"is_member"`
	IsIM        bool   `json:"is_im"`
	IsMpIM      bool   `json:"is_mpim"`
	IsExtShared bool   `json:"is_ext_shared"`
	IsArchived  bool   `json:"is_archived"`
}

// ListChannels returns one page of public and private channels the bot can see.
func (c *Client) ListChannels(ctx context.Context, token, cursor string) ([]conversation, string, error) {
	form := url.Values{
		"types":            {"public_channel,private_channel"},
		"exclude_archived": {"true"},
		"limit":            {"200"},
	}
	if cursor != "" {
		form.Set("cursor", cursor)
	}
	var out struct {
		Channels []conversation `json:"channels"`
		Meta     struct {
			NextCursor string `json:"next_cursor"`
		} `json:"response_metadata"`
	}
	if err := c.callForm(ctx, token, "conversations.list", form, &out); err != nil {
		return nil, "", err
	}
	return out.Channels, out.Meta.NextCursor, nil
}

func (c *Client) ConversationInfo(ctx context.Context, token, channel string) (*conversation, error) {
	var out struct {
		Channel conversation `json:"channel"`
	}
	if err := c.callForm(ctx, token, "conversations.info", url.Values{"channel": {channel}}, &out); err != nil {
		return nil, err
	}
	return &out.Channel, nil
}

// slackMessage is the part of a history message the bot reads.
type slackMessage struct {
	Type     string `json:"type"`
	Subtype  string `json:"subtype"`
	User     string `json:"user"`
	BotID    string `json:"bot_id"`
	Text     string `json:"text"`
	TS       string `json:"ts"`
	ThreadTS string `json:"thread_ts"`
}

// Replies reads up to limit messages of a thread, oldest first.
func (c *Client) Replies(ctx context.Context, token, channel, threadTS string, limit int) ([]slackMessage, error) {
	form := url.Values{"channel": {channel}, "ts": {threadTS}, "limit": {strconv.Itoa(limit)}}
	var out struct {
		Messages []slackMessage `json:"messages"`
	}
	if err := c.callForm(ctx, token, "conversations.replies", form, &out); err != nil {
		return nil, err
	}
	return out.Messages, nil
}

func (c *Client) OpenView(ctx context.Context, token, triggerID string, view Block) error {
	return c.callJSON(ctx, token, "views.open", map[string]any{"trigger_id": triggerID, "view": view}, nil)
}

func (c *Client) PublishView(ctx context.Context, token, userID string, view Block) error {
	return c.callJSON(ctx, token, "views.publish", map[string]any{"user_id": userID, "view": view}, nil)
}

func (c *Client) SetAssistantStatus(ctx context.Context, token, channel, threadTS, status string) error {
	return c.callJSON(ctx, token, "assistant.threads.setStatus", map[string]any{
		"channel_id": channel, "thread_ts": threadTS, "status": status,
	}, nil)
}

// SuggestedPrompt is one assistant-pane prompt chip.
type SuggestedPrompt struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

func (c *Client) SetSuggestedPrompts(ctx context.Context, token, channel, threadTS, title string, prompts []SuggestedPrompt) error {
	return c.callJSON(ctx, token, "assistant.threads.setSuggestedPrompts", map[string]any{
		"channel_id": channel, "thread_ts": threadTS, "title": title, "prompts": prompts,
	}, nil)
}

func (c *Client) SetAssistantTitle(ctx context.Context, token, channel, threadTS, title string) error {
	return c.callJSON(ctx, token, "assistant.threads.setTitle", map[string]any{
		"channel_id": channel, "thread_ts": threadTS, "title": title,
	}, nil)
}

type authTest struct {
	UserID string `json:"user_id"`
	TeamID string `json:"team_id"`
	Team   string `json:"team"`
	BotID  string `json:"bot_id"`
}

func (c *Client) AuthTest(ctx context.Context, token string) (*authTest, error) {
	var out authTest
	if err := c.callForm(ctx, token, "auth.test", url.Values{}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
