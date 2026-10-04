package slackapp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/warmbly/warmbly/internal/models"
)

func TestApprovalValueBindsTheToolCall(t *testing.T) {
	row := uuid.New()
	gotRow, call, ok := parseApprovalValue(approvalValue(row, "call_1"))
	if !ok || gotRow != row || call != "call_1" {
		t.Fatalf("round trip = %v %q %v", gotRow, call, ok)
	}
	if _, _, ok := parseApprovalValue(row.String()); ok {
		t.Fatal("a value without a tool call id decides something")
	}
}

func cardJSON(t *testing.T, m Message) string {
	t.Helper()
	b, err := json.Marshal(m.Blocks)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestApprovalCardShowsTheSendAndOffersAlwaysOnlyWhenAllowed(t *testing.T) {
	a := pendingApprovalInfo{
		Tool: "send_reply", Risk: "send", ToolCallID: "c1",
		Arguments: `{"body": "hello"}`,
		Preview:   &models.AgentSendPreview{From: "me@example.com", To: []string{"you@example.com"}, Subject: "Re: hi", Body: "hello", BodyHTML: "<p>hello</p>"},
	}
	card := cardJSON(t, approvalCard("C1", "1.0", uuid.New(), uuid.New(), a))
	for _, want := range []string{"me@example.com", "you@example.com", "Re: hi", "\\u003cp\\u003ehello\\u003c/p\\u003e", "c1"} {
		if !strings.Contains(card, want) {
			t.Errorf("card is missing %q", want)
		}
	}
	if strings.Contains(card, ActionAlwaysAllow) {
		t.Error("a send offers always allow")
	}
	a = pendingApprovalInfo{Tool: "add_tag", Risk: "write", ToolCallID: "c2", Arguments: "{}"}
	if strings.Contains(cardJSON(t, approvalCard("C1", "1.0", uuid.New(), uuid.New(), a)), ActionAlwaysAllow) {
		t.Error("always allow offered without the server offering it")
	}
	a.AlwaysAllowOffered = true
	if !strings.Contains(cardJSON(t, approvalCard("C1", "1.0", uuid.New(), uuid.New(), a)), ActionAlwaysAllow) {
		t.Error("always allow missing when offered")
	}
}

func TestApprovalCardTruncatesWithinSlackLimits(t *testing.T) {
	a := pendingApprovalInfo{Tool: "update_campaign", Risk: "write", ToolCallID: "c3", Arguments: strings.Repeat("x", approvalChunkRunes*(approvalMaxChunks+2))}
	m := approvalCard("C1", "1.0", uuid.New(), uuid.New(), a)
	if len(m.Blocks) > 50 {
		t.Fatalf("%d blocks", len(m.Blocks))
	}
	if !strings.Contains(cardJSON(t, m), "Some of this is cut off") {
		t.Error("a cut card does not say so")
	}
}
