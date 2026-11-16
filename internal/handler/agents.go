package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"noder/internal/db"
	"noder/internal/model"
)

var agentIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

func agentRandomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func isAgentOnline(agent *model.AgentNode) bool {
	if agent == nil || agent.Status == "disabled" || agent.LastHeartbeatAt == nil {
		return false
	}
	cutoff := time.Now().UTC().Add(-90 * time.Second)
	return agent.LastHeartbeatAt.After(cutoff)
}

func RegisterAgentAdminRoutes(r chi.Router) {
	r.Route("/api/agents", func(r chi.Router) {
		r.Use(AdminAuth)
		r.Get("/", ListAgents)
		r.Post("/", CreateAgent)
		r.Get("/{id}", GetAgent)
		r.Put("/{id}", UpdateAgent)
		r.Delete("/{id}", DeleteAgent)
		r.Post("/{id}/token/rotate", RotateAgentToken)
		r.Get("/{id}/bindings", ListAgentBindings)
		r.Post("/{id}/bindings", AddAgentBindings)
		r.Delete("/{id}/bindings/{endpointId}", RemoveAgentBinding)
	})
	r.Route("/api/endpoints", func(r chi.Router) {
		r.Use(AdminAuth)
		r.Get("/{id}/bindings", ListEndpointBindings)
		r.Post("/{id}/bindings", AddEndpointBindings)
		r.Delete("/{id}/bindings/{agentId}", RemoveEndpointBinding)
	})
}

type agentListItem struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Status          string     `json:"status"`
	LastHeartbeatAt *time.Time `json:"last_heartbeat_at,omitempty"`
	IsOnline        bool       `json:"is_online"`
	Metadata        string     `json:"metadata"`
	BindingCount    int        `json:"binding_count"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func ListAgents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	statusFilter := strings.TrimSpace(q.Get("status"))
	keyword := strings.TrimSpace(q.Get("q"))
	page, _ := strconv.Atoi(q.Get("page"))
	limit, _ := strconv.Atoi(q.Get("limit"))
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	var agents []*model.AgentNode
	sel := db.DB.NewSelect().Model(&agents)
	if statusFilter != "" {
		sel = sel.Where("status = ?", statusFilter)
	}
	if keyword != "" {
		sel = sel.Where("(id LIKE ? OR name LIKE ?)", "%"+keyword+"%", "%"+keyword+"%")
	}
	// status online first, then updated_at desc, id asc
	sel = sel.OrderExpr("CASE status WHEN 'online' THEN 0 WHEN 'offline' THEN 1 WHEN 'disabled' THEN 2 ELSE 3 END ASC, updated_at DESC, id ASC").
		Limit(limit).Offset(offset)
	if err := sel.Scan(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	total, _ := db.DB.NewSelect().Model((*model.AgentNode)(nil)).Count(r.Context())
	// binding counts per agent
	bindingCounts := make(map[string]int)
	if len(agents) > 0 {
		type row struct {
			AgentID string `bun:"agent_node_id"`
			Cnt     int    `bun:"cnt"`
		}
		var rows []row
		_ = db.DB.NewSelect().TableExpr("endpoint_agent_bindings").
			ColumnExpr("agent_node_id").
			ColumnExpr("COUNT(*) AS cnt").
			Where("status = ?", "active").
			Group("agent_node_id").
			Scan(r.Context(), &rows)
		for _, row := range rows {
			bindingCounts[row.AgentID] = row.Cnt
		}
	}
	items := make([]agentListItem, 0, len(agents))
	for _, a := range agents {
		items = append(items, agentListItem{
			ID:              a.ID,
			Name:            a.Name,
			Status:          a.Status,
			LastHeartbeatAt: a.LastHeartbeatAt,
			IsOnline:        isAgentOnline(a),
			Metadata:        a.Metadata,
			BindingCount:    bindingCounts[a.ID],
			CreatedAt:       a.CreatedAt,
			UpdatedAt:       a.UpdatedAt,
		})
	}
	if items == nil {
		items = []agentListItem{}
	}
	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"total": total,
			"items": items,
			"page":  page,
			"limit": limit,
		},
	})
}

func CreateAgent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID   *string `json:"id"`
		Name *string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	name := ""
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
	}
	if name == "" {
		RespondError(w, http.StatusBadRequest, "name is required (1..64)")
		return
	}
	if len(name) > 64 {
		RespondError(w, http.StatusBadRequest, "name too long (max 64)")
		return
	}
	id := ""
	if req.ID != nil && strings.TrimSpace(*req.ID) != "" {
		id = strings.TrimSpace(*req.ID)
		if !agentIDPattern.MatchString(id) {
			RespondError(w, http.StatusBadRequest, "id must match ^[a-zA-Z0-9_-]{1,64}$")
			return
		}
		exists, _ := db.DB.NewSelect().Model((*model.AgentNode)(nil)).Where("id = ?", id).Exists(r.Context())
		if exists {
			RespondJSON(w, http.StatusConflict, map[string]interface{}{"code": 40901, "error": "AGENT_ID_CONFLICT", "message": fmt.Sprintf("agent id '%s' already exists", id)})
			return
		}
	} else {
		id = uuid.NewString()
		// also generate short style like ag_xxx if uuid?
	}
	token := "agt_live_" + agentRandomHex(16)
	now := time.Now().UTC()
	agent := &model.AgentNode{
		ID:                     id,
		Name:                   name,
		ControlTokenCiphertext: &token,
		Status:                 "offline",
		Metadata:               "{}",
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	if _, err := db.DB.NewInsert().Model(agent).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"id":            agent.ID,
			"name":          agent.Name,
			"status":        agent.Status,
			"control_token": token,
			"created_at":    agent.CreatedAt,
			"updated_at":    agent.UpdatedAt,
		},
	})
}

func GetAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		RespondError(w, http.StatusBadRequest, "id is required")
		return
	}
	var agent model.AgentNode
	if err := db.DB.NewSelect().Model(&agent).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondJSON(w, http.StatusNotFound, map[string]interface{}{"code": 40402, "error": "AGENT_NOT_FOUND", "message": "agent not found"})
		return
	}
	bindingCount, _ := db.DB.NewSelect().Model((*model.EndpointAgentBinding)(nil)).Where("agent_node_id = ? AND status = ?", id, "active").Count(r.Context())
	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"id":                agent.ID,
			"name":              agent.Name,
			"status":            agent.Status,
			"last_heartbeat_at": agent.LastHeartbeatAt,
			"is_online":         isAgentOnline(&agent),
			"metadata":          agent.Metadata,
			"binding_count":     bindingCount,
			"created_at":        agent.CreatedAt,
			"updated_at":        agent.UpdatedAt,
		},
	})
}

func UpdateAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var agent model.AgentNode
	if err := db.DB.NewSelect().Model(&agent).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondJSON(w, http.StatusNotFound, map[string]interface{}{"code": 40402, "error": "AGENT_NOT_FOUND", "message": "agent not found"})
		return
	}
	var req struct {
		Name   *string `json:"name"`
		Status *string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	updated := false
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			RespondError(w, http.StatusBadRequest, "name cannot be empty")
			return
		}
		if len(name) > 64 {
			RespondError(w, http.StatusBadRequest, "name too long (max 64)")
			return
		}
		agent.Name = name
		updated = true
	}
	if req.Status != nil {
		s := strings.ToLower(strings.TrimSpace(*req.Status))
		if s != "online" && s != "offline" && s != "disabled" {
			RespondError(w, http.StatusBadRequest, "status must be online|offline|disabled")
			return
		}
		agent.Status = s
		updated = true
	}
	if !updated {
		RespondError(w, http.StatusBadRequest, "no fields to update")
		return
	}
	agent.UpdatedAt = time.Now().UTC()
	if _, err := db.DB.NewUpdate().Model(&agent).Where("id = ?", id).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"id":                agent.ID,
			"name":              agent.Name,
			"status":            agent.Status,
			"last_heartbeat_at": agent.LastHeartbeatAt,
			"is_online":         isAgentOnline(&agent),
			"metadata":          agent.Metadata,
			"updated_at":        agent.UpdatedAt,
		},
	})
}

func DeleteAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var agent model.AgentNode
	if err := db.DB.NewSelect().Model(&agent).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondJSON(w, http.StatusNotFound, map[string]interface{}{"code": 40402, "error": "AGENT_NOT_FOUND", "message": "agent not found"})
		return
	}
	force := r.URL.Query().Get("force") == "true"
	count, _ := db.DB.NewSelect().Model((*model.EndpointAgentBinding)(nil)).Where("agent_node_id = ? AND status = ?", id, "active").Count(r.Context())
	if count > 0 && !force {
		RespondJSON(w, http.StatusConflict, map[string]interface{}{"code": 40903, "error": "BINDING_NOT_EMPTY", "message": fmt.Sprintf("agent has %d active bindings, use ?force=true to remove", count)})
		return
	}
	if force && count > 0 {
		_, _ = db.DB.NewDelete().Model((*model.EndpointAgentBinding)(nil)).Where("agent_node_id = ?", id).Exec(r.Context())
		// also clean deployments? keep history but remove future deployments
		_, _ = db.DB.NewDelete().Model((*model.CredentialDeployment)(nil)).Where("agent_node_id = ?", id).Exec(r.Context())
	}
	if _, err := db.DB.NewDelete().Model((*model.AgentNode)(nil)).Where("id = ?", id).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "success"})
}

func RotateAgentToken(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var agent model.AgentNode
	if err := db.DB.NewSelect().Model(&agent).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondJSON(w, http.StatusNotFound, map[string]interface{}{"code": 40402, "error": "AGENT_NOT_FOUND", "message": "agent not found"})
		return
	}
	newToken := "agt_live_" + agentRandomHex(16)
	now := time.Now().UTC()
	agent.ControlTokenCiphertext = &newToken
	agent.UpdatedAt = now
	if _, err := db.DB.NewUpdate().Model(&agent).Column("control_token_ciphertext", "updated_at").Where("id = ?", id).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"id":            agent.ID,
			"control_token": newToken,
			"updated_at":    now,
		},
	})
}

// bindings: agent -> endpoints

func ListAgentBindings(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	exists, _ := db.DB.NewSelect().Model((*model.AgentNode)(nil)).Where("id = ?", agentID).Exists(r.Context())
	if !exists {
		RespondJSON(w, http.StatusNotFound, map[string]interface{}{"code": 40402, "error": "AGENT_NOT_FOUND", "message": "agent not found"})
		return
	}
	type row struct {
		EndpointID int64      `bun:"endpoint_id"`
		Status     string     `bun:"status"`
		LastSyncAt *time.Time `bun:"last_sync_at"`
		LastError  *string    `bun:"last_error"`
		Tag        string     `bun:"tag"`
		NodeName   string     `bun:"node_name"`
		Protocol   string     `bun:"protocol"`
		IsActive   bool       `bun:"is_active"`
	}
	var rows []row
	err := db.DB.NewSelect().
		TableExpr("endpoint_agent_bindings AS b").
		Join("JOIN node AS n ON n.id = b.endpoint_id").
		ColumnExpr("b.endpoint_id, b.status, b.last_sync_at, b.last_error").
		ColumnExpr("n.tag, n.node_name, n.protocol, n.is_active").
		Where("b.agent_node_id = ? AND b.status = ?", agentID, "active").
		OrderExpr("b.endpoint_id ASC").
		Scan(r.Context(), &rows)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]interface{}{
			"endpoint_id":   row.EndpointID,
			"endpoint_tag":  row.Tag,
			"endpoint_name": row.NodeName,
			"protocol":      row.Protocol,
			"is_active":     row.IsActive,
			"status":        row.Status,
			"last_sync_at":  row.LastSyncAt,
			"last_error":    row.LastError,
		})
	}
	RespondJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "success", "data": items})
}

func AddAgentBindings(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	exists, _ := db.DB.NewSelect().Model((*model.AgentNode)(nil)).Where("id = ?", agentID).Exists(r.Context())
	if !exists {
		RespondJSON(w, http.StatusNotFound, map[string]interface{}{"code": 40402, "error": "AGENT_NOT_FOUND", "message": "agent not found"})
		return
	}
	var req struct {
		EndpointID  *int64  `json:"endpoint_id"`
		EndpointIDs []int64 `json:"endpoint_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	var ids []int64
	if req.EndpointID != nil && *req.EndpointID > 0 {
		ids = append(ids, *req.EndpointID)
	}
	ids = append(ids, req.EndpointIDs...)
	if len(ids) == 0 {
		RespondError(w, http.StatusBadRequest, "endpoint_id or endpoint_ids is required")
		return
	}
	// dedup
	seen := make(map[int64]bool)
	var uniq []int64
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		uniq = append(uniq, id)
	}
	for _, eid := range uniq {
		exists, _ := db.DB.NewSelect().Model((*model.Node)(nil)).Where("id = ?", eid).Exists(r.Context())
		if !exists {
			RespondJSON(w, http.StatusNotFound, map[string]interface{}{"code": 40401, "error": "ENDPOINT_NOT_FOUND", "message": fmt.Sprintf("endpoint %d not found", eid)})
			return
		}
		binding := &model.EndpointAgentBinding{
			EndpointID:  eid,
			AgentNodeID: agentID,
			Status:      "active",
		}
		_, _ = db.DB.NewInsert().Model(binding).
			On("CONFLICT (endpoint_id, agent_node_id) DO UPDATE").
			Set("status = ?", "active").
			Exec(r.Context())
	}
	// return updated list
	ListAgentBindings(w, r)
}

func RemoveAgentBinding(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "id")
	endpointIDStr := chi.URLParam(r, "endpointId")
	eid, err := strconv.ParseInt(endpointIDStr, 10, 64)
	if err != nil || eid <= 0 {
		RespondError(w, http.StatusBadRequest, "invalid endpointId")
		return
	}
	_, _ = db.DB.NewDelete().Model((*model.EndpointAgentBinding)(nil)).Where("endpoint_id = ? AND agent_node_id = ?", eid, agentID).Exec(r.Context())
	RespondJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "success"})
}

// endpoint perspective

func ListEndpointBindings(w http.ResponseWriter, r *http.Request) {
	endpointIDStr := chi.URLParam(r, "id")
	eid, err := strconv.ParseInt(endpointIDStr, 10, 64)
	if err != nil || eid <= 0 {
		RespondError(w, http.StatusBadRequest, "invalid endpoint id")
		return
	}
	exists, _ := db.DB.NewSelect().Model((*model.Node)(nil)).Where("id = ?", eid).Exists(r.Context())
	if !exists {
		RespondJSON(w, http.StatusNotFound, map[string]interface{}{"code": 40401, "error": "ENDPOINT_NOT_FOUND", "message": "endpoint not found"})
		return
	}
	type row struct {
		AgentID         string     `bun:"agent_id"`
		AgentName       string     `bun:"agent_name"`
		AgentStatus     string     `bun:"agent_status"`
		LastHeartbeatAt *time.Time `bun:"last_heartbeat_at"`
		Status          string     `bun:"status"`
		LastSyncAt      *time.Time `bun:"last_sync_at"`
		LastError       *string    `bun:"last_error"`
	}
	var rows []row
	err = db.DB.NewSelect().
		TableExpr("endpoint_agent_bindings AS b").
		Join("JOIN agent_nodes AS a ON a.id = b.agent_node_id").
		ColumnExpr("a.id AS agent_id, a.name AS agent_name, a.status AS agent_status, a.last_heartbeat_at").
		ColumnExpr("b.status, b.last_sync_at, b.last_error").
		Where("b.endpoint_id = ? AND b.status = ?", eid, "active").
		OrderExpr("a.id ASC").
		Scan(r.Context(), &rows)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	items := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		isOnline := false
		if row.AgentStatus != "disabled" && row.LastHeartbeatAt != nil {
			isOnline = row.LastHeartbeatAt.After(time.Now().UTC().Add(-90 * time.Second))
		}
		items = append(items, map[string]interface{}{
			"agent_id":          row.AgentID,
			"agent_name":        row.AgentName,
			"agent_status":      row.AgentStatus,
			"is_online":         isOnline,
			"last_heartbeat_at": row.LastHeartbeatAt,
			"status":            row.Status,
			"last_sync_at":      row.LastSyncAt,
			"last_error":        row.LastError,
		})
	}
	RespondJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "success", "data": items})
}

func AddEndpointBindings(w http.ResponseWriter, r *http.Request) {
	endpointIDStr := chi.URLParam(r, "id")
	eid, err := strconv.ParseInt(endpointIDStr, 10, 64)
	if err != nil || eid <= 0 {
		RespondError(w, http.StatusBadRequest, "invalid endpoint id")
		return
	}
	exists, _ := db.DB.NewSelect().Model((*model.Node)(nil)).Where("id = ?", eid).Exists(r.Context())
	if !exists {
		RespondJSON(w, http.StatusNotFound, map[string]interface{}{"code": 40401, "error": "ENDPOINT_NOT_FOUND", "message": "endpoint not found"})
		return
	}
	var req struct {
		AgentID  *string  `json:"agent_id"`
		AgentIDs []string `json:"agent_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	var ids []string
	if req.AgentID != nil && strings.TrimSpace(*req.AgentID) != "" {
		ids = append(ids, strings.TrimSpace(*req.AgentID))
	}
	for _, id := range req.AgentIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		RespondError(w, http.StatusBadRequest, "agent_id or agent_ids is required")
		return
	}
	seen := make(map[string]bool)
	var uniq []string
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		uniq = append(uniq, id)
	}
	for _, aid := range uniq {
		exists, _ := db.DB.NewSelect().Model((*model.AgentNode)(nil)).Where("id = ?", aid).Exists(r.Context())
		if !exists {
			RespondJSON(w, http.StatusNotFound, map[string]interface{}{"code": 40402, "error": "AGENT_NOT_FOUND", "message": fmt.Sprintf("agent %s not found", aid)})
			return
		}
		binding := &model.EndpointAgentBinding{
			EndpointID:  eid,
			AgentNodeID: aid,
			Status:      "active",
		}
		_, _ = db.DB.NewInsert().Model(binding).
			On("CONFLICT (endpoint_id, agent_node_id) DO UPDATE").
			Set("status = ?", "active").
			Exec(r.Context())
	}
	ListEndpointBindings(w, r)
}

func RemoveEndpointBinding(w http.ResponseWriter, r *http.Request) {
	endpointIDStr := chi.URLParam(r, "id")
	agentID := chi.URLParam(r, "agentId")
	eid, err := strconv.ParseInt(endpointIDStr, 10, 64)
	if err != nil || eid <= 0 {
		RespondError(w, http.StatusBadRequest, "invalid endpoint id")
		return
	}
	if strings.TrimSpace(agentID) == "" {
		RespondError(w, http.StatusBadRequest, "agentId is required")
		return
	}
	_, _ = db.DB.NewDelete().Model((*model.EndpointAgentBinding)(nil)).Where("endpoint_id = ? AND agent_node_id = ?", eid, agentID).Exec(r.Context())
	RespondJSON(w, http.StatusOK, map[string]interface{}{"code": 0, "message": "success"})
}
