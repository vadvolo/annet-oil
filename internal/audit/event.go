// Package audit records an auditable trail of actions — who ran which command
// on which devices, from where, and whether it succeeded — into a queryable
// store (PostgreSQL). Recording hangs off shared service choke-points
// (annet.Service, gnetcli.Client, check.Devices) rather than the HTTP layer, so
// CLI and SSH traffic is captured too; the actor/source travel via context.
package audit

import "time"

// Action constants describe the kind of action recorded.
const (
	ActionGen     = "gen"
	ActionDiff    = "diff"
	ActionPatch   = "patch"
	ActionDeploy  = "deploy"
	ActionExecute = "execute"
	ActionCheck   = "check"
	ActionState   = "state"
)

// Error carries a structured failure, mirroring the convention used by the
// diagnostic packages (check, checkeast).
type Error struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// Event is a single audit record.
type Event struct {
	ID         int64          `json:"id"`
	Timestamp  time.Time      `json:"timestamp"`
	Actor      string         `json:"actor"` // user.Name | os-user | remote addr | "legacy"
	ActorRole  string         `json:"actor_role,omitempty"`
	Source     string         `json:"source"` // api | cli | ssh | mcp
	Action     string         `json:"action"` // gen|diff|patch|deploy|execute|check|state|rfc_*
	Devices    []string       `json:"devices,omitempty"`
	Command    string         `json:"command,omitempty"` // raw command for execute
	Params     map[string]any `json:"params,omitempty"`  // generators/dry_run/…
	Success    bool           `json:"success"`
	DurationMs int64          `json:"duration_ms,omitempty"`
	Error      *Error         `json:"error,omitempty"`
	RequestID  string         `json:"request_id,omitempty"`
}
