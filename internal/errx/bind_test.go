package errx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type bindSend struct {
	To      []string `json:"to" binding:"required"`
	Subject string   `json:"subject" binding:"required"`
	Mode    string   `json:"send_mode" binding:"omitempty,oneof=instant smart"`
	Limit   int      `json:"limit" binding:"omitempty,max=10"`
	Inner   struct {
		Name string `json:"name" binding:"required"`
	} `json:"inner"`
}

type bindContact struct {
	Email    string            `json:"email" binding:"required"`
	Due      *time.Time        `json:"due"`
	Campaign uuid.UUID         `json:"campaign"`
	Tags     []uuid.UUID       `json:"tags"`
	Metadata map[string]string `json:"metadata"`
}

type bindQuery struct {
	Search string `form:"q"`
}

func bindBody(t *testing.T, body string, dst any) error {
	t.Helper()
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c.ShouldBindJSON(dst)
}

func TestInvalidBody(t *testing.T) {
	cases := []struct {
		name string
		body string
		dst  func() any
		want []string
	}{
		{"empty", "", func() any { return &bindSend{} }, []string{"request body is empty"}},
		{"syntax", `{"to": [}`, func() any { return &bindSend{} }, []string{"not valid JSON", "at byte 9"}},
		{"truncated", `{"to": ["a"`, func() any { return &bindSend{} }, []string{"ends before the value is complete"}},
		{"required fields on an empty object", `{}`, func() any { return &bindSend{} }, []string{`"to" is required`, `"subject" is required`, `"inner.name" is required`}},
		{"oneof", `{"to":["a"],"subject":"s","send_mode":"later","inner":{"name":"n"}}`, func() any { return &bindSend{} }, []string{`"send_mode" must be one of: instant, smart`}},
		{"max", `{"to":["a"],"subject":"s","limit":11,"inner":{"name":"n"}}`, func() any { return &bindSend{} }, []string{`"limit" must be at most 10`}},
		{"field of the wrong type", `{"to":"a@b.co","subject":"s"}`, func() any { return &bindSend{} }, []string{`Field "to" must be a JSON array, not a JSON string`}},
		{"nested field of the wrong type", `{"to":["a"],"subject":"s","inner":{"name":5}}`, func() any { return &bindSend{} }, []string{`Field "inner.name" must be a JSON string, not a JSON number`}},
		{"object where an array is expected", `{"email":"a@b.co"}`, func() any { return &[]bindContact{} }, []string{"must be a JSON array, not a JSON object"}},
		{"array where an object is expected", `[]`, func() any { return &bindSend{} }, []string{"must be a JSON object, not a JSON array"}},
		{"slice element validation", `[{"email":"a@b.co"},{}]`, func() any { return &[]bindContact{} }, []string{`"email" is required`}},
		{"uuid", `{"email":"a@b.co","campaign":"nope"}`, func() any { return &bindContact{} }, []string{"should be a UUID"}},
		{"time", `{"email":"a@b.co","due":"tomorrow"}`, func() any { return &bindContact{} }, []string{"not RFC 3339", `"tomorrow"`}},
		{"NUL in text", `{"to":["a"],"subject":"x\u0000y","inner":{"name":"n"}}`, func() any { return &bindSend{} }, []string{`Field "subject" contains a NUL character`}},
		{"NUL in map key", `{"email":"a@b.co","metadata":{"x\u0000y":"value"}}`, func() any { return &bindContact{} }, []string{`Field "metadata.key" contains a NUL character`}},
		{"uuid of the wrong JSON type", `{"email":"a@b.co","campaign":5}`, func() any { return &bindContact{} }, []string{`Field "campaign" must be a JSON string, not a JSON number`}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := bindBody(t, tc.body, tc.dst())
			if err == nil {
				t.Fatalf("bind of %q succeeded, want an error", tc.body)
			}
			got := InvalidBody(err)
			if got.Code != BadRequest {
				t.Fatalf("code = %d, want %d", got.Code, BadRequest)
			}
			if strings.Contains(got.Message, "malformed JSON") {
				t.Fatalf("message still says malformed JSON: %q", got.Message)
			}
			for _, w := range tc.want {
				if !strings.Contains(got.Message, w) {
					t.Errorf("message %q does not contain %q", got.Message, w)
				}
			}
		})
	}
}

func TestInvalidQueryRejectsNUL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/?q=%00", nil)

	var query bindQuery
	err := c.ShouldBindQuery(&query)
	if err == nil {
		t.Fatal("binding a NUL query succeeded")
	}
	got := InvalidQuery(err)
	if got.Code != BadRequest || !strings.Contains(got.Message, `Query parameter "q" contains a NUL character`) {
		t.Fatalf("InvalidQuery() = %#v", got)
	}
}

func TestInvalidBodyCapsTheList(t *testing.T) {
	type many struct {
		A string `json:"a" binding:"required"`
		B string `json:"b" binding:"required"`
		C string `json:"c" binding:"required"`
		D string `json:"d" binding:"required"`
		E string `json:"e" binding:"required"`
		F string `json:"f" binding:"required"`
		G string `json:"g" binding:"required"`
	}
	got := InvalidBody(bindBody(t, `{}`, &many{}))
	if !strings.Contains(got.Message, "and 2 more") || strings.Contains(got.Message, `"f"`) {
		t.Fatalf("message = %q, want five fields and a count of the rest", got.Message)
	}
}
