package handler

import (
	"net/http"

	"noder/internal/db"
	"noder/internal/model"
)

type serviceEndpointAgent struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

type serviceEndpointResponse struct {
	ID           int64                  `json:"id"`
	Tag          string                 `json:"tag"`
	NodeName     string                 `json:"node_name"`
	Protocol     string                 `json:"protocol"`
	IsActive     bool                   `json:"is_active"`
	Status       string                 `json:"status"`
	IsAgentless  bool                   `json:"is_agentless"`
	BindingCount int                    `json:"binding_count"`
	BoundAgents  []serviceEndpointAgent `json:"bound_agents"`
}

type serviceEndpointBindingRow struct {
	EndpointID  int64  `bun:"endpoint_id"`
	AgentID     string `bun:"agent_id"`
	AgentName   string `bun:"agent_name"`
	AgentStatus string `bun:"agent_status"`
}

// ListServiceEndpoints returns Noder endpoints and their active agent bindings.
// Agent status is reported independently from binding status, so an active
// binding to an offline agent is still visible to the caller.
func ListServiceEndpoints(w http.ResponseWriter, r *http.Request) {
	var nodes []model.Node
	if err := db.DB.NewSelect().Model(&nodes).
		OrderExpr("sort_order ASC, id ASC").
		Scan(r.Context()); err != nil {
		RespondServiceError(w, http.StatusInternalServerError, "failed to list endpoints")
		return
	}

	var bindings []serviceEndpointBindingRow
	if err := db.DB.NewSelect().
		TableExpr("endpoint_agent_bindings AS b").
		Join("JOIN agent_nodes AS a ON a.id = b.agent_node_id").
		ColumnExpr("b.endpoint_id").
		ColumnExpr("a.id AS agent_id").
		ColumnExpr("a.name AS agent_name").
		ColumnExpr("a.status AS agent_status").
		Where("b.status = ?", "active").
		OrderExpr("b.endpoint_id ASC, a.id ASC").
		Scan(r.Context(), &bindings); err != nil {
		RespondServiceError(w, http.StatusInternalServerError, "failed to list endpoint bindings")
		return
	}

	agentsByEndpoint := make(map[int64][]serviceEndpointAgent, len(nodes))
	for _, binding := range bindings {
		agentsByEndpoint[binding.EndpointID] = append(
			agentsByEndpoint[binding.EndpointID],
			serviceEndpointAgent{
				ID:     binding.AgentID,
				Name:   binding.AgentName,
				Status: binding.AgentStatus,
			},
		)
	}

	endpoints := make([]serviceEndpointResponse, 0, len(nodes))
	for _, node := range nodes {
		agents := agentsByEndpoint[node.ID]
		if agents == nil {
			agents = make([]serviceEndpointAgent, 0)
		}
		endpoints = append(endpoints, serviceEndpointResponse{
			ID:           node.ID,
			Tag:          node.Tag,
			NodeName:     node.NodeName,
			Protocol:     node.Protocol,
			IsActive:     node.IsActive,
			Status:       node.Status,
			IsAgentless:  node.IsAgentless,
			BindingCount: len(agents),
			BoundAgents:  agents,
		})
	}

	// This endpoint's contract uses code=200 in the JSON envelope, matching
	// the published Mall-facing response example.
	RespondServiceJSON(w, http.StatusOK, map[string]interface{}{
		"code": 200,
		"data": endpoints,
	})
}
