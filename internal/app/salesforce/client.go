// Package salesforce is Warmbly's native Salesforce sync: the REST client, the
// link between a Warmbly contact and its Lead or Contact, the activity outbox
// that logs campaign events as Tasks, the pull loop that keeps linked records
// current, and imports from list views and Salesforce Campaigns.
package salesforce

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

	"github.com/warmbly/warmbly/internal/pkg/safehttp"
)

// APIVersion is the REST version every request targets.
const APIVersion = "v62.0"

// maxBatch is the record cap of one composite sObject collection call.
const maxBatch = 200

// TokenSource hands the client a usable access token and the org's API host.
// force asks for a fresh token after Salesforce refused the current one.
type TokenSource interface {
	Token(ctx context.Context, force bool) (token, instanceURL string, err error)
}

// Usage is the org-wide API consumption Salesforce reports on every response.
type Usage struct {
	Used int
	Max  int
}

// Client calls one connected Salesforce org.
type Client struct {
	http    *http.Client
	tokens  TokenSource
	onUsage func(Usage)

	mu    sync.Mutex
	calls int
}

// NewClient builds a client over a token source. onUsage, when set, receives
// the Sforce-Limit-Info reading of every response.
func NewClient(ts TokenSource, onUsage func(Usage)) *Client {
	hc := safehttp.Client(30 * time.Second)
	tr := safehttp.NewTransport()
	tr.ResponseHeaderTimeout = 30 * time.Second // a composite write can outlast the default header wait
	hc.Transport = tr
	return &Client{
		http:    hc,
		tokens:  ts,
		onUsage: onUsage,
	}
}

// Calls is how many API requests this client has made.
func (c *Client) Calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// APIError is Salesforce's own refusal, kept whole so the activity log can show
// the errorCode an admin can act on.
type APIError struct {
	Status  int
	Code    string
	Message string
	Fields  []string
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = http.StatusText(e.Status)
	}
	if e.Code != "" {
		msg = e.Code + ": " + msg
	}
	if len(e.Fields) > 0 {
		msg += " (" + strings.Join(e.Fields, ", ") + ")"
	}
	return msg
}

var (
	// ErrSessionExpired means a fresh token was refused too.
	ErrSessionExpired = errors.New("salesforce refused the session; reconnect required")
	// ErrRateLimited means the org has spent its daily API allocation.
	ErrRateLimited = errors.New("salesforce API request limit reached")
)

// IsCode reports whether err is a Salesforce refusal with this errorCode.
func IsCode(err error, code string) bool {
	var ae *APIError
	return errors.As(err, &ae) && ae.Code == code
}

// Retryable reports whether a failed call may succeed later unchanged.
func Retryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrRateLimited) {
		return true
	}
	var ae *APIError
	if errors.As(err, &ae) {
		switch ae.Code {
		case "UNABLE_TO_LOCK_ROW", "SERVER_UNAVAILABLE", "REQUEST_RUNNING_TOO_LONG", "QUERY_TIMEOUT":
			return true
		}
		return ae.Status >= 500 || ae.Status == http.StatusTooManyRequests
	}
	// Transport failures (timeouts, resets) carry no API error at all.
	return !errors.Is(err, ErrSessionExpired)
}

// Header is an extra request header for one call.
type Header struct{ Key, Value string }

// do sends one request. path is either a /services/... path or an absolute
// nextRecordsUrl-style path Salesforce handed back. A 401 is answered once with
// a forced token refresh.
func (c *Client) do(ctx context.Context, method, path string, body any, out any, headers ...Header) error {
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		payload = b
	}
	for attempt := 0; attempt < 2; attempt++ {
		token, instance, err := c.tokens.Token(ctx, attempt > 0)
		if err != nil {
			return err
		}
		if instance == "" || !strings.HasPrefix(path, "/") {
			return errors.New("salesforce instance URL unknown; reconnect the integration")
		}
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(instance, "/")+path, reader)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		for _, h := range headers {
			req.Header.Set(h.Key, h.Value)
		}
		resp, err := c.http.Do(req)
		c.mu.Lock()
		c.calls++
		c.mu.Unlock()
		if err != nil {
			return err
		}
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
		resp.Body.Close()
		if u, ok := parseLimitInfo(resp.Header.Get("Sforce-Limit-Info")); ok && c.onUsage != nil {
			c.onUsage(u)
		}
		if resp.StatusCode == http.StatusUnauthorized {
			if attempt == 0 {
				continue
			}
			return ErrSessionExpired
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if out != nil && len(raw) > 0 {
				return json.Unmarshal(raw, out)
			}
			return nil
		}
		apiErr := parseAPIError(resp.StatusCode, raw)
		if apiErr.Code == "REQUEST_LIMIT_EXCEEDED" {
			return fmt.Errorf("%w: %s", ErrRateLimited, apiErr.Message)
		}
		return apiErr
	}
	return ErrSessionExpired
}

func parseAPIError(status int, raw []byte) *APIError {
	out := &APIError{Status: status}
	var list []struct {
		Message   string   `json:"message"`
		ErrorCode string   `json:"errorCode"`
		Fields    []string `json:"fields"`
	}
	if json.Unmarshal(raw, &list) == nil && len(list) > 0 {
		out.Code, out.Message, out.Fields = list[0].ErrorCode, list[0].Message, list[0].Fields
		return out
	}
	// The OAuth endpoints answer with a single object instead.
	var single struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if json.Unmarshal(raw, &single) == nil && single.Error != "" {
		out.Code, out.Message = single.Error, single.Description
		return out
	}
	out.Message = strings.TrimSpace(string(raw))
	if r := []rune(out.Message); len(r) > 300 {
		out.Message = string(r[:300])
	}
	return out
}

// parseLimitInfo reads "api-usage=18/15000".
func parseLimitInfo(h string) (Usage, bool) {
	for _, part := range strings.Split(h, ";") {
		k, v, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok || k != "api-usage" {
			continue
		}
		used, max, ok := strings.Cut(v, "/")
		if !ok {
			return Usage{}, false
		}
		u, err1 := strconv.Atoi(used)
		m, err2 := strconv.Atoi(max)
		if err1 != nil || err2 != nil {
			return Usage{}, false
		}
		return Usage{Used: u, Max: m}, true
	}
	return Usage{}, false
}

func dataPath(rest string) string {
	return "/services/data/" + APIVersion + rest
}

// Record is one row as Salesforce returns it, relationship fields nested.
type Record map[string]any

// String reads a field, following a dotted relationship path ("Owner.Name").
func (r Record) String(path string) string {
	var cur any = map[string]any(r)
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur = m[part]
	}
	switch v := cur.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	default:
		return ""
	}
}

// Bool reads a boolean field.
func (r Record) Bool(field string) bool {
	b, _ := r[field].(bool)
	return b
}

// Time reads a datetime field.
func (r Record) Time(field string) *time.Time {
	s := r.String(field)
	if s == "" {
		return nil
	}
	for _, layout := range []string{"2006-01-02T15:04:05.000-0700", time.RFC3339Nano, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			t = t.UTC()
			return &t
		}
	}
	return nil
}

type queryPage struct {
	Done           bool     `json:"done"`
	TotalSize      int      `json:"totalSize"`
	NextRecordsURL string   `json:"nextRecordsUrl"`
	Records        []Record `json:"records"`
}

// Query runs SOQL and hands every page to fn until the result is exhausted,
// fn returns false, or max records (0 = no cap) have been read.
func (c *Client) Query(ctx context.Context, soql string, max int, fn func([]Record) bool) (total int, err error) {
	path := dataPath("/query?q=" + url.QueryEscape(soql))
	read := 0
	for path != "" {
		var page queryPage
		if err := c.do(ctx, http.MethodGet, path, nil, &page); err != nil {
			return total, err
		}
		total = page.TotalSize
		recs := page.Records
		if max > 0 && read+len(recs) > max {
			recs = recs[:max-read]
		}
		for _, r := range recs {
			delete(r, "attributes")
		}
		read += len(recs)
		if !fn(recs) || page.Done || (max > 0 && read >= max) {
			return total, nil
		}
		path = page.NextRecordsURL
	}
	return total, nil
}

// QueryAll is Query collected into one slice.
func (c *Client) QueryAll(ctx context.Context, soql string, max int) ([]Record, error) {
	var out []Record
	_, err := c.Query(ctx, soql, max, func(rs []Record) bool {
		out = append(out, rs...)
		return true
	})
	return out, err
}

// PicklistValue is one option of a picklist field.
type PicklistValue struct {
	Value        string `json:"value"`
	Label        string `json:"label"`
	Active       bool   `json:"active"`
	DefaultValue bool   `json:"defaultValue"`
}

// Field is one field of an object's describe.
type Field struct {
	Name           string          `json:"name"`
	Label          string          `json:"label"`
	Type           string          `json:"type"`
	Length         int             `json:"length"`
	Createable     bool            `json:"createable"`
	Updateable     bool            `json:"updateable"`
	Calculated     bool            `json:"calculated"`
	AutoNumber     bool            `json:"autoNumber"`
	Custom         bool            `json:"custom"`
	Nillable       bool            `json:"nillable"`
	DefaultedOn    bool            `json:"defaultedOnCreate"`
	Restricted     bool            `json:"restrictedPicklist"`
	PicklistValues []PicklistValue `json:"picklistValues"`
	ReferenceTo    []string        `json:"referenceTo"`
}

// Describe is the subset of an object's describe the sync reads.
type Describe struct {
	Name       string  `json:"name"`
	Label      string  `json:"label"`
	Createable bool    `json:"createable"`
	Updateable bool    `json:"updateable"`
	Queryable  bool    `json:"queryable"`
	Fields     []Field `json:"fields"`
}

// Field returns the named field, or nil.
func (d *Describe) Field(name string) *Field {
	for i := range d.Fields {
		if strings.EqualFold(d.Fields[i].Name, name) {
			return &d.Fields[i]
		}
	}
	return nil
}

// Describe reads an object's metadata.
func (c *Client) Describe(ctx context.Context, object string) (*Describe, error) {
	var d Describe
	if err := c.do(ctx, http.MethodGet, dataPath("/sobjects/"+url.PathEscape(object)+"/describe"), nil, &d); err != nil {
		return nil, err
	}
	return &d, nil
}

// ListView is a saved list view of an object.
type ListView struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Name  string `json:"developerName"`
}

// ListViews returns the list views the connected user can see on an object.
func (c *Client) ListViews(ctx context.Context, object string) ([]ListView, error) {
	var out struct {
		ListViews []ListView `json:"listviews"`
		NextURL   string     `json:"nextRecordsUrl"`
	}
	if err := c.do(ctx, http.MethodGet, dataPath("/sobjects/"+url.PathEscape(object)+"/listviews?limit=200"), nil, &out); err != nil {
		return nil, err
	}
	for i := range out.ListViews {
		out.ListViews[i].ID = NormalizeID(out.ListViews[i].ID)
	}
	return out.ListViews, nil
}

// ListViewQuery returns the SOQL a list view runs, so its whole result can be
// paged through /query instead of the 2,000-row results resource.
func (c *Client) ListViewQuery(ctx context.Context, object, id string) (string, error) {
	var out struct {
		Query string `json:"query"`
	}
	if err := c.do(ctx, http.MethodGet, dataPath("/sobjects/"+url.PathEscape(object)+"/listviews/"+url.PathEscape(id)+"/describe"), nil, &out); err != nil {
		return "", err
	}
	if strings.TrimSpace(out.Query) == "" {
		return "", errors.New("salesforce returned no query for this list view")
	}
	return out.Query, nil
}

// SaveResult is the outcome of one record in a collection write.
type SaveResult struct {
	ID      string `json:"id"`
	Success bool   `json:"success"`
	Errors  []struct {
		StatusCode string   `json:"statusCode"`
		Message    string   `json:"message"`
		Fields     []string `json:"fields"`
	} `json:"errors"`
}

// Err turns a failed result into an APIError.
func (r SaveResult) Err() error {
	if r.Success {
		return nil
	}
	if len(r.Errors) == 0 {
		return &APIError{Status: http.StatusBadRequest, Message: "salesforce rejected the record"}
	}
	e := r.Errors[0]
	return &APIError{Status: http.StatusBadRequest, Code: e.StatusCode, Message: e.Message, Fields: e.Fields}
}

// Create inserts records of one object, up to 200 per call, without letting one
// bad record fail the rest. Results are in input order.
func (c *Client) Create(ctx context.Context, object string, records []map[string]any, headers ...Header) ([]SaveResult, error) {
	return c.collection(ctx, http.MethodPost, object, records, headers...)
}

// Update patches records (each carrying Id) of one object.
func (c *Client) Update(ctx context.Context, object string, records []map[string]any, headers ...Header) ([]SaveResult, error) {
	return c.collection(ctx, http.MethodPatch, object, records, headers...)
}

func (c *Client) collection(ctx context.Context, method, object string, records []map[string]any, headers ...Header) ([]SaveResult, error) {
	out := make([]SaveResult, 0, len(records))
	for start := 0; start < len(records); start += maxBatch {
		end := min(start+maxBatch, len(records))
		chunk := make([]map[string]any, 0, end-start)
		for _, r := range records[start:end] {
			rec := make(map[string]any, len(r)+1)
			for k, v := range r {
				rec[k] = v
			}
			rec["attributes"] = map[string]string{"type": object}
			chunk = append(chunk, rec)
		}
		var res []SaveResult
		body := map[string]any{"allOrNone": false, "records": chunk}
		if err := c.do(ctx, method, dataPath("/composite/sobjects"), body, &res, headers...); err != nil {
			return out, err
		}
		for i := range res {
			res[i].ID = NormalizeID(res[i].ID)
		}
		out = append(out, res...)
	}
	return out, nil
}

// Get reads one record with the named fields.
func (c *Client) Get(ctx context.Context, object, id string, fields []string) (Record, error) {
	var out Record
	path := dataPath("/sobjects/" + url.PathEscape(object) + "/" + url.PathEscape(id))
	if len(fields) > 0 {
		path += "?fields=" + url.QueryEscape(strings.Join(fields, ","))
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}
	delete(out, "attributes")
	return out, nil
}

// IdentityIDs pulls the org and user ids out of the identity URL the token
// response carries (https://login.salesforce.com/id/<org>/<user>).
func IdentityIDs(identityURL string) (orgID, userID string) {
	u, err := url.Parse(identityURL)
	if err != nil {
		return "", ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 3 && parts[0] == "id" {
		return NormalizeID(parts[1]), NormalizeID(parts[2])
	}
	return "", ""
}

// Limits reads the org's daily API allocation.
func (c *Client) Limits(ctx context.Context) (Usage, error) {
	var out map[string]struct {
		Max       int `json:"Max"`
		Remaining int `json:"Remaining"`
	}
	if err := c.do(ctx, http.MethodGet, dataPath("/limits"), nil, &out); err != nil {
		return Usage{}, err
	}
	d := out["DailyApiRequests"]
	return Usage{Used: d.Max - d.Remaining, Max: d.Max}, nil
}

// Quote renders s as a SOQL string literal.
func Quote(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `'`, `\'`, "\n", `\n`, "\r", `\r`, "\t", `\t`, "\"", `\"`)
	return "'" + r.Replace(s) + "'"
}

// QuoteList renders values as the body of a SOQL IN clause.
func QuoteList(values []string) string {
	parts := make([]string, len(values))
	for i, v := range values {
		parts[i] = Quote(v)
	}
	return "(" + strings.Join(parts, ",") + ")"
}

// NormalizeID returns the 18-character form of a Salesforce id. The API speaks
// 18 characters and the UI 15; storing one form keeps comparisons honest.
func NormalizeID(id string) string {
	id = strings.TrimSpace(id)
	if len(id) != 15 {
		return id
	}
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345"
	suffix := make([]byte, 3)
	for block := 0; block < 3; block++ {
		n := 0
		for i := 0; i < 5; i++ {
			ch := id[block*5+i]
			if ch >= 'A' && ch <= 'Z' {
				n |= 1 << i
			}
		}
		suffix[block] = alphabet[n]
	}
	return id + string(suffix)
}

// ValidID reports whether s looks like a Salesforce record id.
func ValidID(s string) bool {
	if len(s) != 15 && len(s) != 18 {
		return false
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
