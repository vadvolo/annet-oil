package middleware

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"annet-oil/internal/audit"
	"annet-oil/internal/logging"
)

// AuditContextMiddleware attaches the authenticated user's identity to the
// request context so downstream service choke-points (annet.Service, gnetcli)
// can attribute audit events to the API caller. It must run after
// AuthMiddleware, which populates the user. The chi RequestID (if present) is
// also carried so events can be correlated with request logs.
func AuditContextMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		actor := "anonymous"
		role := ""
		if user := GetUser(ctx); user != nil {
			actor = user.Name
			if user.Role != nil {
				role = user.Role.Name
			}
		}

		ctx = audit.WithActor(ctx, actor, role, audit.SourceAPI)

		// Bridge chi's request ID into the audit logging key namespace.
		if reqID := middleware.GetReqID(ctx); reqID != "" {
			ctx = context.WithValue(ctx, logging.RequestIDKey, reqID)
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
