package audit

import "context"

// Source constants identify where an action originated.
const (
	SourceAPI = "api"
	SourceCLI = "cli"
	SourceSSH = "ssh"
	SourceMCP = "mcp"
)

type contextKey string

const (
	actorKey     contextKey = "audit_actor"
	actorRoleKey contextKey = "audit_actor_role"
	sourceKey    contextKey = "audit_source"
)

// Actor is the resolved identity+origin of an action, read from context by the
// recording hooks.
type Actor struct {
	Name   string
	Role   string
	Source string
}

// WithActor returns a context carrying the actor identity and origin. The
// recording hooks read these via ActorFrom.
func WithActor(ctx context.Context, name, role, source string) context.Context {
	ctx = context.WithValue(ctx, actorKey, name)
	ctx = context.WithValue(ctx, actorRoleKey, role)
	ctx = context.WithValue(ctx, sourceKey, source)
	return ctx
}

// ActorFrom extracts the actor from context. When nothing was set (e.g. a code
// path that never went through an entrypoint), it falls back to a "legacy"
// actor so events are still attributable to *something*.
func ActorFrom(ctx context.Context) Actor {
	a := Actor{Name: "legacy", Source: "unknown"}
	if v, ok := ctx.Value(actorKey).(string); ok && v != "" {
		a.Name = v
	}
	if v, ok := ctx.Value(actorRoleKey).(string); ok {
		a.Role = v
	}
	if v, ok := ctx.Value(sourceKey).(string); ok && v != "" {
		a.Source = v
	}
	return a
}
