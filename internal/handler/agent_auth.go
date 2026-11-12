package handler

import (
	"context"
	"net/http"

	"noder/internal/db"
	"noder/internal/model"
)

type agentNodeContextKey struct{}

func contextWithAgentNode(ctx context.Context, agent *model.AgentNode) context.Context {
	return context.WithValue(ctx, agentNodeContextKey{}, agent)
}

// AgentNodeFromContext returns the authenticated agent node.
func AgentNodeFromContext(ctx context.Context) *model.AgentNode {
	v := ctx.Value(agentNodeContextKey{})
	if v == nil {
		return nil
	}
	agent, _ := v.(*model.AgentNode)
	return agent
}

// AgentAuth authenticates /api/agent/v1 requests via X-Agent-Token and binds
// the resolved AgentNode to the request context.
func AgentAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("X-Agent-Token")
		if token == "" {
			RespondServiceJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"code":    401,
				"message": "missing X-Agent-Token",
				"error":   "ERR_INVALID_TOKEN",
			})
			return
		}

		var agent model.AgentNode
		if err := db.DB.NewSelect().Model(&agent).
			Where("control_token_ciphertext = ?", token).
			Where("status != ?", "disabled").
			Scan(r.Context()); err != nil {
			RespondServiceJSON(w, http.StatusUnauthorized, map[string]interface{}{
				"code":    401,
				"message": "invalid agent token",
				"error":   "ERR_INVALID_TOKEN",
			})
			return
		}

		next.ServeHTTP(w, r.WithContext(contextWithAgentNode(r.Context(), &agent)))
	})
}

// requireAgentEndpointAccess returns false (and writes 403) if the authenticated
// agent is not bound to endpointID. endpointID<=0 means "no specific endpoint".
func requireAgentEndpointAccess(w http.ResponseWriter, r *http.Request, agentID string, endpointID int64) bool {
	if endpointID <= 0 {
		return true
	}
	exists, err := db.DB.NewSelect().Model((*model.EndpointAgentBinding)(nil)).
		Where("agent_node_id = ? AND endpoint_id = ? AND status = ?", agentID, endpointID, "active").
		Exists(r.Context())
	if err != nil || !exists {
		RespondServiceJSON(w, http.StatusForbidden, map[string]interface{}{
			"code":    403,
			"message": "agent not bound to endpoint",
			"error":   "ERR_FORBIDDEN",
		})
		return false
	}
	return true
}
