package models

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// AgentSession is one dashboard-agent conversation. Sessions are per-user
// (private to the member who started them) unless the org enables
// assistant_shared_history, which makes every conversation workspace-shared.
type AgentSession struct {
	ID      uuid.UUID           `json:"id"`
	OrgID   uuid.UUID           `json:"org_id"`
	UserID  uuid.UUID           `json:"user_id"`
	Title   string              `json:"title"`
	Context AgentSessionContext `json:"context"`
	// UserName is the owner's display name, populated only by the shared
	// org-wide listing so the history rail can attribute conversations.
	UserName  string    `json:"user_name,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AgentSessionContext is the read-then-execute jsonb blob on a session: the
// client's page/resource awareness, the model chosen for the run, and any tool
// call paused awaiting approval. Validated at the app boundary (this struct),
// never filtered in SQL.
type AgentSessionContext struct {
	// Page / Resource mirror the presence shape the client already pushes
	// ({page, resource}); injected into the system prompt as context.
	Page     string `json:"page,omitempty"`
	Resource string `json:"resource,omitempty"`
	// Model is the provider model id resolved for this session's tier.
	Model string `json:"model,omitempty"`
	// FreeModel is true when the session ran on a free/local backend
	// (AI_FREE): persisted so a reopened tab still shows the warning.
	FreeModel bool `json:"free_model,omitempty"`
	// Pending is the tool call awaiting the user's approve/deny when a run is
	// paused; nil when the session is idle or running.
	Pending *PendingAgentTool `json:"pending,omitempty"`
}

// PendingAgentTool is a write/send tool call paused for human approval.
type PendingAgentTool struct {
	MessageID   string          `json:"message_id"`
	ToolCallID  string          `json:"tool_call_id"`
	ToolName    string          `json:"tool_name"`
	Risk        string          `json:"risk"`
	Args        json.RawMessage `json:"args"`
	ArgsSummary string          `json:"args_summary,omitempty"`
	// Arguments is Args as indented JSON with sorted keys, capped at AgentApprovalArgsMax bytes.
	Arguments          string `json:"arguments,omitempty"`
	ArgumentsTruncated bool   `json:"arguments_truncated,omitempty"`
	// Preview is what a send will do: sender, recipients, subject and bodies.
	Preview *AgentSendPreview `json:"preview,omitempty"`
	// AlwaysAllowOffered is whether the viewer may make this tool a workspace "always allow".
	AlwaysAllowOffered bool `json:"always_allow_offered,omitempty"`
}

// AgentApprovalArgsMax caps the arguments an approval card carries.
const AgentApprovalArgsMax = 16 << 10

// AgentSendPreview is the resolved shape of a send awaiting approval.
type AgentSendPreview struct {
	From     string   `json:"from"`
	To       []string `json:"to"`
	Subject  string   `json:"subject"`
	Body     string   `json:"body"`
	BodyHTML string   `json:"body_html,omitempty"`
}

// AgentMessageRow is one persisted transcript turn. Content is the serialized
// provider-agnostic message (role, text, tool_calls, tool result) so a run
// resumes losslessly after an approval pause.
type AgentMessageRow struct {
	ID        uuid.UUID       `json:"id"`
	SessionID uuid.UUID       `json:"session_id"`
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	Tokens    int             `json:"tokens"`
	CreatedAt time.Time       `json:"created_at"`
}

// AIToolPolicy is a per-org "always allow this tool" decision. Only write-class
// tools can have a policy; send-class tools are never auto-allowed.
type AIToolPolicy struct {
	OrgID     uuid.UUID  `json:"org_id"`
	ToolName  string     `json:"tool_name"`
	Decision  string     `json:"decision"`
	CreatedBy *uuid.UUID `json:"created_by,omitempty"`
	// CreatedByName is the setter's display name, for the settings list.
	CreatedByName string    `json:"created_by_name,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}
