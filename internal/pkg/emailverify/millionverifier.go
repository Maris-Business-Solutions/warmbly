package emailverify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// MillionVerifier is the paid Verifier backed by MillionVerifier's single
// address API. Pay-as-you-go credits, one credit per lookup; the customer's
// own key is stored on their integration connection.
type MillionVerifier struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

const (
	millionVerifierAPI     = "https://api.millionverifier.com"
	millionVerifierTimeout = 20 * time.Second
	// millionVerifierProbeTimeout is the per-address server-side budget, in
	// seconds, sent as the API's timeout parameter.
	millionVerifierProbeTimeout = 15
)

// NewMillionVerifier constructs the client. baseURL is overridable for tests.
func NewMillionVerifier(apiKey string, baseURL string) *MillionVerifier {
	if baseURL == "" {
		baseURL = millionVerifierAPI
	}
	return &MillionVerifier{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: millionVerifierTimeout, CheckRedirect: refuseInsecureRedirect},
	}
}

type mvSingleResponse struct {
	Email      string `json:"email"`
	Quality    string `json:"quality"`
	Result     string `json:"result"`
	ResultCode int    `json:"resultcode"`
	SubResult  string `json:"subresult"`
	Free       bool   `json:"free"`
	Role       bool   `json:"role"`
	Error      string `json:"error"`
	Credits    int    `json:"credits"`
}

type mvCreditsResponse struct {
	Credits int `json:"credits"`
	// RenewingCredits is a subscription's allowance, which real-time checks
	// also draw on; an account living on one reports zero Credits.
	RenewingCredits int    `json:"renewing_credits"`
	Error           string `json:"error"`
}

// Verify implements Verifier. An API failure is never an "invalid" verdict:
// it degrades to unknown with the reason, and the typed error is reported
// through Check so the connection can be marked degraded.
func (m *MillionVerifier) Verify(ctx context.Context, email string) Result {
	res, _ := m.Check(ctx, email)
	return res
}

// Check is Verify with the transport/account error surfaced.
func (m *MillionVerifier) Check(ctx context.Context, email string) (Result, error) {
	now := time.Now().UTC()
	res := Result{Email: strings.ToLower(strings.TrimSpace(email)), CheckedAt: now, Status: StatusUnknown, Provider: ProviderMillionVerifier}

	q := url.Values{}
	q.Set("api", m.apiKey)
	q.Set("email", res.Email)
	q.Set("timeout", fmt.Sprintf("%d", millionVerifierProbeTimeout))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.baseURL+"/api/v3/?"+q.Encode(), nil)
	if err != nil {
		err = withoutURL(err)
		res.Reason = "millionverifier request failed: " + err.Error()
		return res, err
	}
	resp, err := m.client.Do(req)
	if err != nil {
		err = withoutURL(err)
		res.Reason = "millionverifier unreachable: " + err.Error()
		return res, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		res.Reason = "millionverifier rejected the API key"
		return res, ErrProviderKey
	}
	if resp.StatusCode != http.StatusOK {
		res.Reason = fmt.Sprintf("millionverifier answered HTTP %d", resp.StatusCode)
		return res, fmt.Errorf("millionverifier: http %d", resp.StatusCode)
	}
	var out mvSingleResponse
	if err := json.Unmarshal(body, &out); err != nil {
		res.Reason = "millionverifier answered with an unreadable body"
		return res, err
	}
	if out.Error != "" {
		return res, m.accountError(&res, out.Error)
	}

	v, ok := NormalizeExternal(ProviderMillionVerifier, out.Result)
	if !ok {
		res.Reason = "millionverifier result not recognised: " + out.Result
		return res, nil
	}
	res.Status, res.SubStatus = v.Status, v.SubStatus
	res.IsCatchAll = v.SubStatus == SubStatusCatchAll
	res.HasMX = out.Result != "invalid" || out.SubResult != ""
	if out.Role && res.Status == StatusValid {
		res.Status = StatusRisky
		res.SubStatus = SubStatusRole
	}
	reason := out.Result
	if out.SubResult != "" {
		reason += " (" + out.SubResult + ")"
	}
	res.Reason = "millionverifier: " + reason
	return res, nil
}

// ObservesBalance is true: the credits endpoint reports the balance, so an
// exhausted account is visible without spending anything.
func (m *MillionVerifier) ObservesBalance() bool { return true }

// Account validates the key and returns the remaining credits.
func (m *MillionVerifier) Account(ctx context.Context) (*int, error) {
	n, err := m.Credits(ctx)
	return &n, err
}

// Credits returns the credits a real-time check can spend (pay-as-you-go plus
// a subscription's renewing allowance), and validates the key.
func (m *MillionVerifier) Credits(ctx context.Context) (int, error) {
	q := url.Values{}
	q.Set("api", m.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.baseURL+"/api/v3/credits?"+q.Encode(), nil)
	if err != nil {
		return 0, withoutURL(err)
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return 0, withoutURL(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return 0, ErrProviderKey
	}
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("millionverifier: http %d", resp.StatusCode)
	}
	var out mvCreditsResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return 0, err
	}
	if out.Error != "" {
		return 0, m.accountError(nil, out.Error)
	}
	return out.Credits + out.RenewingCredits, nil
}

func (m *MillionVerifier) accountError(res *Result, msg string) error {
	lower := strings.ToLower(msg)
	var err error
	switch {
	case strings.Contains(lower, "api key") || strings.Contains(lower, "apikey") || strings.Contains(lower, "unauthori"):
		err = ErrProviderKey
	case strings.Contains(lower, "credit"):
		err = ErrProviderCredits
	default:
		err = fmt.Errorf("millionverifier: %s", msg)
	}
	if res != nil {
		res.Reason = "millionverifier: " + msg
	}
	return err
}

// withoutURL drops the request URL from a transport error, because the API key
// travels in its query string.
func withoutURL(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return fmt.Errorf("millionverifier: %s: %w", ue.Op, ue.Err)
	}
	return err
}
