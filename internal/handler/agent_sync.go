package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"noder/internal/db"
	"noder/internal/logger"
	"noder/internal/model"
)

// RegisterAgentRoutes registers /api/agent/v1 for xray-agent pull/report.
func RegisterAgentRoutes(r chi.Router) {
	r.Route("/api/agent/v1", func(r chi.Router) {
		r.Use(AgentAuth)
		r.Post("/heartbeat", AgentHeartbeat)
		r.Get("/deployments", AgentGetDeployments)
		r.Post("/deployments/report", AgentReportDeployments)
		r.Post("/traffic/report", AgentReportTraffic)
		r.Post("/state/sync", AgentSyncState)
	})
}

type agentHeartbeatReq struct {
	AgentVersion string   `json:"agent_version"`
	XrayVersion  string   `json:"xray_version"`
	Hostname     string   `json:"hostname"`
	Capabilities []string `json:"capabilities"`
	Endpoints    []struct {
		EndpointID int64  `json:"endpoint_id"`
		InboundTag string `json:"inbound_tag"`
		Ready      bool   `json:"ready"`
	} `json:"endpoints"`
}

// AgentHeartbeat handles POST /api/agent/v1/heartbeat.
func AgentHeartbeat(w http.ResponseWriter, r *http.Request) {
	agent := AgentNodeFromContext(r.Context())
	if agent == nil {
		respondAgentErr(w, http.StatusUnauthorized, "ERR_INVALID_TOKEN", "unauthorized")
		return
	}
	var req agentHeartbeatReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondAgentErr(w, http.StatusBadRequest, "ERR_BAD_REQUEST", "invalid JSON body")
		return
	}
	now := time.Now().UTC()
	meta := map[string]interface{}{
		"agent_version": req.AgentVersion,
		"xray_version":  req.XrayVersion,
		"hostname":      req.Hostname,
		"capabilities":  req.Capabilities,
		"endpoints":     req.Endpoints,
	}
	metaJSON, _ := json.Marshal(meta)
	if _, err := db.DB.NewUpdate().Model((*model.AgentNode)(nil)).
		Set("status = ?", "online").
		Set("last_heartbeat_at = ?", now).
		Set("metadata = ?", string(metaJSON)).
		Set("updated_at = ?", now).
		Where("id = ?", agent.ID).Exec(r.Context()); err != nil {
		respondAgentErr(w, http.StatusInternalServerError, "ERR_INTERNAL", "failed to update heartbeat")
		return
	}
	// Update last_sync_at on bound endpoints that reported ready (best effort).
	for _, ep := range req.Endpoints {
		if ep.EndpointID <= 0 {
			continue
		}
		_, _ = db.DB.NewUpdate().Model((*model.EndpointAgentBinding)(nil)).
			Set("last_sync_at = ?", now).
			Set("last_error = NULL").
			Where("agent_node_id = ? AND endpoint_id = ?", agent.ID, ep.EndpointID).
			Exec(r.Context())
	}
	logger.DebugContext(r.Context(), "received agent heartbeat",
		"agent_id", agent.ID,
		"agent_ver", req.AgentVersion,
		"xray_ver", req.XrayVersion,
		"endpoints", len(req.Endpoints),
	)
	RespondServiceJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"server_time":       now.Format(time.RFC3339),
			"sync_interval_sec": 15,
		},
	})
}

// AgentGetDeployments handles GET /api/agent/v1/deployments.
func AgentGetDeployments(w http.ResponseWriter, r *http.Request) {
	agent := AgentNodeFromContext(r.Context())
	if agent == nil {
		respondAgentErr(w, http.StatusUnauthorized, "ERR_INVALID_TOKEN", "unauthorized")
		return
	}
	q := r.URL.Query()
	var endpointFilter int64
	if v := strings.TrimSpace(q.Get("endpoint_id")); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			endpointFilter = n
		}
	}
	if !requireAgentEndpointAccess(w, r, agent.ID, endpointFilter) {
		return
	}
	sinceRevision := strings.TrimSpace(q.Get("since_revision"))
	limit := 500
	if v := strings.TrimSpace(q.Get("limit")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
			if limit > 2000 {
				limit = 2000
			}
		}
	}
	offset := 0
	if c := strings.TrimSpace(q.Get("cursor")); c != "" {
		if n, err := strconv.Atoi(c); err == nil && n >= 0 {
			offset = n
		}
	}

	var deps []*model.CredentialDeployment
	sel := db.DB.NewSelect().Model(&deps).
		Where("credential_deployment.agent_node_id = ?", agent.ID)
	if endpointFilter > 0 {
		sel = sel.Where("credential_deployment.endpoint_id = ?", endpointFilter)
	} else {
		// Only endpoints bound to this agent.
		sel = sel.Where("credential_deployment.endpoint_id IN (SELECT endpoint_id FROM endpoint_agent_bindings WHERE agent_node_id = ? AND status = ?)", agent.ID, "active")
	}
	if sinceRevision != "" {
		sel = sel.Where("CAST(COALESCE(NULLIF(credential_deployment.revision,''),'0') AS INTEGER) > CAST(? AS INTEGER)", sinceRevision)
	}
	sel = sel.OrderExpr("credential_deployment.endpoint_id ASC, CAST(COALESCE(NULLIF(credential_deployment.revision,''),'0') AS INTEGER) ASC, credential_deployment.credential_id ASC").
		Limit(limit + 1).Offset(offset)
	if err := sel.Scan(r.Context()); err != nil && !errors.Is(err, sql.ErrNoRows) {
		respondAgentErr(w, http.StatusInternalServerError, "ERR_INTERNAL", "failed to load deployments")
		return
	}

	hasMore := len(deps) > limit
	if hasMore {
		deps = deps[:limit]
	}
	items := make([]map[string]interface{}, 0, len(deps))
	for _, d := range deps {
		item, err := buildDeploymentItem(r, agent, d)
		if err != nil {
			continue
		}
		items = append(items, item)
	}
	var nextCursor interface{}
	if hasMore {
		nextCursor = strconv.Itoa(offset + limit)
	}
	logger.DebugContext(r.Context(), "agent pulled deployments",
		"agent_id", agent.ID,
		"endpoint_id", endpointFilter,
		"since_rev", sinceRevision,
		"count", len(items),
		"has_more", hasMore,
	)
	RespondServiceJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"agent_id":     agent.ID,
			"generated_at": time.Now().UTC().Format(time.RFC3339),
			"has_more":     hasMore,
			"next_cursor":  nextCursor,
			"items":        items,
		},
	})
}

func buildDeploymentItem(r *http.Request, agent *model.AgentNode, d *model.CredentialDeployment) (map[string]interface{}, error) {
	var cred model.Credential
	if err := db.DB.NewSelect().Model(&cred).Where("id = ?", d.CredentialID).Scan(r.Context()); err != nil {
		return nil, err
	}
	var ep model.Node
	if err := db.DB.NewSelect().Model(&ep).Where("id = ?", d.EndpointID).Scan(r.Context()); err != nil {
		return nil, err
	}
	// Resolve grant for this credential+endpoint (D10 allows multiple; pick most relevant active/latest).
	var grants []*model.AccessGrant
	_ = db.DB.NewSelect().Model(&grants).
		Where("credential_id = ? AND endpoint_id = ?", d.CredentialID, d.EndpointID).
		OrderExpr("CASE WHEN status='active' THEN 0 ELSE 1 END, id DESC").
		Limit(1).Scan(r.Context())
	var grant *model.AccessGrant
	if len(grants) > 0 {
		grant = grants[0]
	}

	desired := normalizeDesiredState(d.DesiredState)
	// If desired active but grant/credential not usable, report disabled (safety).
	now := time.Now().UTC()
	if desired == "active" {
		if grant != nil && !grant.IsUsable(now) {
			desired = "disabled"
		}
		if !cred.IsUsable(now) && cred.Status == model.CredentialStatusRevoked {
			desired = "revoked"
		} else if !cred.IsUsable(now) && desired == "active" {
			desired = "disabled"
		}
	}

	email := credentialEmail(&cred, &ep)
	proto := strings.ToLower(strings.TrimSpace(cred.Scheme))
	if proto == "landing" {
		proto = "vless"
	}
	credPayload := map[string]interface{}{
		"credential_id": cred.ID,
		"scheme":        proto,
		"email":         email,
		"uuid":          nil,
		"username":      nil,
		"password":      nil,
		"flow":          nil,
		"cipher":        nil,
	}
	if cred.UUIDValue != nil && *cred.UUIDValue != "" {
		credPayload["uuid"] = *cred.UUIDValue
	}
	if cred.Username != nil && *cred.Username != "" {
		credPayload["username"] = *cred.Username
	}
	// Password: needed for trojan/shadowsocks; for vless/vmess usually empty.
	if pw := cred.EffectivePassword(); pw != nil && *pw != "" {
		credPayload["password"] = *pw
	}
	if proto == "vless" && ep.Flow != nil && *ep.Flow != "" {
		credPayload["flow"] = *ep.Flow
	}
	// shadowsocks cipher from secret_config.method if present
	if proto == "shadowsocks" {
		var sc map[string]interface{}
		if json.Unmarshal([]byte(cred.SecretConfig), &sc) == nil {
			if m, ok := sc["method"].(string); ok && m != "" {
				credPayload["cipher"] = m
			}
			if m, ok := sc["cipher"].(string); ok && m != "" && credPayload["cipher"] == nil {
				credPayload["cipher"] = m
			}
		}
		// fallback to endpoint method
		if credPayload["cipher"] == nil && ep.Method != nil && *ep.Method != "" {
			credPayload["cipher"] = *ep.Method
		}
	}

	grantPayload := map[string]interface{}{
		"grant_id":            nil,
		"status":              "active",
		"format":              nil,
		"expires_at":          nil,
		"traffic_quota_bytes": int64(0),
		"traffic_used_bytes":  int64(0),
	}
	if grant != nil {
		CheckAndResetTrafficCycle(r.Context(), grant, now)
		grantPayload["grant_id"] = grant.ID
		grantPayload["status"] = effectiveGrantStatus(grant, now)
		if grant.Format != nil {
			grantPayload["format"] = *grant.Format
		}
		if grant.ExpiresAt != nil {
			grantPayload["expires_at"] = grant.ExpiresAt.UTC().Format(time.RFC3339)
		}
		grantPayload["traffic_quota_bytes"] = grant.TrafficQuotaBytes
		grantPayload["traffic_used_bytes"] = grant.TrafficUsedBytes
	}

	rev := d.Revision
	if rev == "" {
		rev = "0"
	}
	return map[string]interface{}{
		"deployment_key": map[string]interface{}{
			"credential_id": d.CredentialID,
			"endpoint_id":   d.EndpointID,
			"agent_node_id": d.AgentNodeID,
		},
		"desired_state": desired,
		"revision":      rev,
		"grant":         grantPayload,
		"endpoint": map[string]interface{}{
			"endpoint_id":    ep.ID,
			"inbound_tag":    ep.EffectiveInboundTag(),
			"server_address": ep.ServerAddress,
			"server_port":    ep.ServerPort,
			"protocol":       strings.ToLower(strings.TrimSpace(ep.Protocol)),
		},
		"credential": credPayload,
	}, nil
}

func credentialEmail(cred *model.Credential, ep *model.Node) string {
	if cred != nil && cred.Username != nil {
		u := strings.TrimSpace(*cred.Username)
		if strings.Contains(u, "@") {
			return u
		}
	}
	if cred != nil && cred.DisplayName != nil {
		dn := strings.TrimSpace(*cred.DisplayName)
		if strings.Contains(dn, "@") {
			return dn
		}
	}
	exit := "node"
	if ep != nil && ep.Tag != "" {
		exit = ep.Tag
	}
	if cred != nil {
		return fmt.Sprintf("c_%s@%s", cred.ID, exit)
	}
	return fmt.Sprintf("c_unknown@%s", exit)
}

func normalizeDesiredState(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "active":
		return "active"
	case "revoked":
		return "revoked"
	case "disabled":
		return "disabled"
	default:
		if s == "" {
			return "disabled"
		}
		return strings.ToLower(s)
	}
}

type agentReportReq struct {
	Items []struct {
		CredentialID string `json:"credential_id"`
		EndpointID   int64  `json:"endpoint_id"`
		AppliedState string `json:"applied_state"`
		Revision     string `json:"revision"`
		AppliedAt    string `json:"applied_at"`
		Error        string `json:"error"`
		XrayUserKey  string `json:"xray_user_key"`
	} `json:"items"`
}

// AgentReportDeployments handles POST /api/agent/v1/deployments/report.
func AgentReportDeployments(w http.ResponseWriter, r *http.Request) {
	agent := AgentNodeFromContext(r.Context())
	if agent == nil {
		respondAgentErr(w, http.StatusUnauthorized, "ERR_INVALID_TOKEN", "unauthorized")
		return
	}
	var req agentReportReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondAgentErr(w, http.StatusBadRequest, "ERR_BAD_REQUEST", "invalid JSON body")
		return
	}
	accepted := 0
	conflicts := make([]map[string]interface{}, 0)
	now := time.Now().UTC()
	for _, it := range req.Items {
		if it.CredentialID == "" || it.EndpointID <= 0 {
			continue
		}
		if !requireAgentEndpointAccess(w, r, agent.ID, it.EndpointID) {
			return
		}
		var dep model.CredentialDeployment
		err := db.DB.NewSelect().Model(&dep).
			Where("credential_id = ? AND endpoint_id = ? AND agent_node_id = ?",
				it.CredentialID, it.EndpointID, agent.ID).
			Scan(r.Context())
		if err != nil {
			conflicts = append(conflicts, map[string]interface{}{
				"credential_id":    it.CredentialID,
				"endpoint_id":      it.EndpointID,
				"reason":           "not_found",
				"current_revision": nil,
			})
			continue
		}
		curRev := revInt(dep.Revision)
		repRev := revInt(it.Revision)
		if repRev < curRev {
			logger.WarnContext(r.Context(), "deployment revision conflict",
				"agent_id", agent.ID,
				"credential_id", it.CredentialID,
				"endpoint_id", it.EndpointID,
				"report_rev", it.Revision,
				"cur_rev", dep.Revision,
			)
			conflicts = append(conflicts, map[string]interface{}{
				"credential_id":    it.CredentialID,
				"endpoint_id":      it.EndpointID,
				"reason":           "stale_revision",
				"current_revision": dep.Revision,
			})
			continue
		}
		appliedAt := now
		if it.AppliedAt != "" {
			if t, e := time.Parse(time.RFC3339, it.AppliedAt); e == nil {
				appliedAt = t.UTC()
			}
		}
		upd := db.DB.NewUpdate().Model((*model.CredentialDeployment)(nil)).
			Set("applied_state = ?", normalizeAppliedState(it.AppliedState)).
			Set("last_sync_at = ?", appliedAt).
			Where("credential_id = ? AND endpoint_id = ? AND agent_node_id = ?",
				it.CredentialID, it.EndpointID, agent.ID)
		if it.Error != "" {
			upd = upd.Set("last_error = ?", it.Error)
		} else {
			upd = upd.Set("last_error = NULL")
		}
		// Only advance revision tracking if report carries >= current (already checked).
		if it.Revision != "" {
			upd = upd.Set("revision = ?", it.Revision)
		}
		if _, err := upd.Exec(r.Context()); err != nil {
			conflicts = append(conflicts, map[string]interface{}{
				"credential_id":    it.CredentialID,
				"endpoint_id":      it.EndpointID,
				"reason":           "update_failed",
				"current_revision": dep.Revision,
			})
			continue
		}
		accepted++
	}
	logger.InfoContext(r.Context(), "processed agent deployment reports",
		"agent_id", agent.ID,
		"accepted", accepted,
		"conflicts", len(conflicts),
	)
	RespondServiceJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"accepted":  accepted,
			"conflicts": conflicts,
		},
	})
}

func revInt(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

func normalizeAppliedState(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "active", "revoked", "disabled", "failed":
		return strings.ToLower(strings.TrimSpace(s))
	default:
		return "unknown"
	}
}

type agentTrafficReq struct {
	ReportID   string `json:"report_id"`
	EndpointID int64  `json:"endpoint_id"`
	ObservedAt string `json:"observed_at"`
	Items      []struct {
		CredentialID  string `json:"credential_id"`
		GrantID       string `json:"grant_id"`
		UplinkBytes   int64  `json:"uplink_bytes"`
		DownlinkBytes int64  `json:"downlink_bytes"`
	} `json:"items"`
}

// AgentReportTraffic handles POST /api/agent/v1/traffic/report (P1).
func AgentReportTraffic(w http.ResponseWriter, r *http.Request) {
	agent := AgentNodeFromContext(r.Context())
	if agent == nil {
		respondAgentErr(w, http.StatusUnauthorized, "ERR_INVALID_TOKEN", "unauthorized")
		return
	}
	var req agentTrafficReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondAgentErr(w, http.StatusBadRequest, "ERR_BAD_REQUEST", "invalid JSON body")
		return
	}
	if req.ReportID == "" {
		respondAgentErr(w, http.StatusBadRequest, "ERR_BAD_REQUEST", "report_id is required")
		return
	}
	if req.EndpointID > 0 && !requireAgentEndpointAccess(w, r, agent.ID, req.EndpointID) {
		return
	}
	// Idempotency: dedup by report_id (24h TTL handled by cleanup best-effort).
	now := time.Now().UTC()
	dedup := &model.AgentTrafficReport{ReportID: req.ReportID, AgentNodeID: agent.ID, CreatedAt: now}
	if _, err := db.DB.NewInsert().Model(dedup).Exec(r.Context()); err != nil {
		// Duplicate (PK conflict) or other - treat duplicate as already accepted.
		var exists model.AgentTrafficReport
		if e := db.DB.NewSelect().Model(&exists).Where("report_id = ?", req.ReportID).Scan(r.Context()); e == nil {
			logger.DebugContext(r.Context(), "duplicate agent traffic report ignored",
				"agent_id", agent.ID,
				"report_id", req.ReportID,
			)
			RespondServiceJSON(w, http.StatusOK, map[string]interface{}{
				"code": 0, "message": "success", "data": map[string]interface{}{"accepted": true},
			})
			return
		}
		respondAgentErr(w, http.StatusInternalServerError, "ERR_INTERNAL", "failed to record report")
		return
	}
	// Best-effort cleanup of old dedup rows.
	cutoff := now.Add(-24 * time.Hour)
	_, _ = db.DB.NewDelete().Model((*model.AgentTrafficReport)(nil)).Where("created_at < ?", cutoff).Exec(r.Context())

	for _, it := range req.Items {
		delta := it.UplinkBytes + it.DownlinkBytes
		if delta <= 0 {
			continue
		}
		if it.GrantID != "" {
			_, _ = db.DB.NewUpdate().Model((*model.AccessGrant)(nil)).
				Set("traffic_used_bytes = traffic_used_bytes + ?", delta).
				Set("traffic_updated_at = ?", now).
				Where("id = ?", it.GrantID).
				Exec(r.Context())
			continue
		}
		if it.CredentialID != "" {
			// Fallback: update grants for this credential on endpoint (or all).
			q := db.DB.NewUpdate().Model((*model.AccessGrant)(nil)).
				Set("traffic_used_bytes = traffic_used_bytes + ?", delta).
				Set("traffic_updated_at = ?", now).
				Where("credential_id = ?", it.CredentialID)
			if req.EndpointID > 0 {
				q = q.Where("endpoint_id = ?", req.EndpointID)
			}
			_, _ = q.Exec(r.Context())
		}
	}
	logger.InfoContext(r.Context(), "processed agent traffic report",
		"agent_id", agent.ID,
		"report_id", req.ReportID,
		"items_count", len(req.Items),
	)
	RespondServiceJSON(w, http.StatusOK, map[string]interface{}{
		"code": 0, "message": "success", "data": map[string]interface{}{"accepted": true},
	})
}

type agentStateSyncReq struct {
	Items []struct {
		CredentialID string `json:"credential_id"`
		EndpointID   int64  `json:"endpoint_id"`
		AppliedState string `json:"applied_state"`
		Revision     string `json:"revision"`
		Present      *bool  `json:"present"`
	} `json:"items"`
}

func AgentSyncState(w http.ResponseWriter, r *http.Request) {
	agent := AgentNodeFromContext(r.Context())
	if agent == nil {
		respondAgentErr(w, http.StatusUnauthorized, "ERR_INVALID_TOKEN", "unauthorized")
		return
	}
	var req agentStateSyncReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondAgentErr(w, http.StatusBadRequest, "ERR_BAD_REQUEST", "invalid JSON body")
		return
	}
	if req.Items == nil {
		req.Items = []struct {
			CredentialID string `json:"credential_id"`
			EndpointID   int64  `json:"endpoint_id"`
			AppliedState string `json:"applied_state"`
			Revision     string `json:"revision"`
			Present      *bool  `json:"present"`
		}{}
	}
	now := time.Now().UTC()
	conflicts := make([]map[string]interface{}, 0)
	drift := make([]map[string]interface{}, 0)

	// reportMap: key credential_id:endpoint_id -> item index, only for bound items
	type syncItem struct {
		CredentialID string
		EndpointID   int64
		AppliedState string
		Revision     string
		Present      bool
	}
	reportMap := make(map[string]syncItem)
	// per-item binding validation
	for _, it := range req.Items {
		credID := strings.TrimSpace(it.CredentialID)
		epID := it.EndpointID
		if credID == "" || epID <= 0 {
			conflicts = append(conflicts, map[string]interface{}{
				"credential_id": it.CredentialID,
				"endpoint_id":   it.EndpointID,
				"reason":        "invalid_argument",
			})
			logger.WarnContext(r.Context(), "agent state sync invalid item",
				"agent_id", agent.ID,
				"credential_id", it.CredentialID,
				"endpoint_id", it.EndpointID,
			)
			continue
		}
		// Reuse binding logic from requireAgentEndpointAccess (active binding check)
		exists, err := db.DB.NewSelect().Model((*model.EndpointAgentBinding)(nil)).
			Where("agent_node_id = ? AND endpoint_id = ? AND status = ?", agent.ID, epID, "active").
			Exists(r.Context())
		if err != nil || !exists {
			conflicts = append(conflicts, map[string]interface{}{
				"credential_id": credID,
				"endpoint_id":   epID,
				"reason":        "not_bound",
			})
			logger.WarnContext(r.Context(), "agent state sync not bound",
				"agent_id", agent.ID,
				"endpoint_id", epID,
				"credential_id", credID,
			)
			continue
		}
		present := true
		if it.Present != nil {
			present = *it.Present
		}
		key := fmt.Sprintf("%s:%d", credID, epID)
		reportMap[key] = syncItem{
			CredentialID: credID,
			EndpointID:   epID,
			AppliedState: normalizeAppliedState(it.AppliedState),
			Revision:     strings.TrimSpace(it.Revision),
			Present:      present,
		}
	}

	// Load desired deployments for this agent (all bound endpoints)
	var deps []*model.CredentialDeployment
	if err := db.DB.NewSelect().Model(&deps).
		Where("agent_node_id = ?", agent.ID).
		Scan(r.Context()); err != nil && !errors.Is(err, sql.ErrNoRows) {
		respondAgentErr(w, http.StatusInternalServerError, "ERR_INTERNAL", "failed to load deployments")
		return
	}

	// For last_sync_at aggregation: primary endpoint_id for logging (first dep or first item)
	var primaryEndpointID int64
	if len(deps) > 0 {
		primaryEndpointID = deps[0].EndpointID
	} else if len(req.Items) > 0 {
		primaryEndpointID = req.Items[0].EndpointID
	}

	// Build drift by comparing desired vs reported
	for _, d := range deps {
		key := fmt.Sprintf("%s:%d", d.CredentialID, d.EndpointID)
		rep, ok := reportMap[key]
		desired := normalizeDesiredState(d.DesiredState)

		if !ok || !rep.Present {
			// Agent missing this deployment
			appliedMismatch := false
			if ok {
				appliedMismatch = rep.AppliedState != desired
			} else {
				// No report at all => treat as drift if desired active (present=false)
				// For drift last_error logic: applied_state != desired && present==false
				// Here applied is essentially unknown, count as drift when desired active
				appliedMismatch = d.AppliedState != desired
			}
			// Update last_sync_at/last_error
			upd := db.DB.NewUpdate().Model((*model.CredentialDeployment)(nil)).
				Set("last_sync_at = ?", now).
				Where("credential_id = ? AND endpoint_id = ? AND agent_node_id = ?", d.CredentialID, d.EndpointID, d.AgentNodeID)
			if appliedMismatch && !ok || (ok && !rep.Present && rep.AppliedState != desired) {
				// present=false and drift
				upd = upd.Set("last_error = ?", "drift: present=false")
			} else if !ok && desired == "active" {
				upd = upd.Set("last_error = ?", "drift: missing on agent")
			} else if !rep.Present && desired == "active" {
				upd = upd.Set("last_error = ?", "drift: present=false")
			} else {
				upd = upd.Set("last_error = NULL")
			}
			_, _ = upd.Exec(r.Context())

			if desired == "active" {
				drift = append(drift, map[string]interface{}{
					"credential_id": d.CredentialID,
					"endpoint_id":   d.EndpointID,
					"action":        "add",
					"desired_state": desired,
					"revision":      d.Revision,
					"reason":        "missing_on_agent",
				})
			} else {
				// desired revoked/disabled but agent already absent -> in sync, no drift
				// ensure last_error cleared handled above
			}
			continue
		}
		// present == true
		applied := rep.AppliedState
		if applied == "unknown" && rep.AppliedState == "" {
			applied = ""
		}
		needDrift := false
		action := "update"
		reason := ""
		if applied != desired {
			needDrift = true
			if desired == "revoked" || desired == "disabled" {
				action = "remove"
				reason = "desired_revoked"
			} else {
				action = "update"
				reason = "state_mismatch"
			}
		} else if rep.Revision != d.Revision {
			needDrift = true
			action = "update"
			reason = "revision_mismatch"
		}
		// Update DB sync markers
		upd := db.DB.NewUpdate().Model((*model.CredentialDeployment)(nil)).
			Set("last_sync_at = ?", now).
			Where("credential_id = ? AND endpoint_id = ? AND agent_node_id = ?", d.CredentialID, d.EndpointID, d.AgentNodeID)
		if needDrift && !rep.Present {
			upd = upd.Set("last_error = ?", "drift")
		} else if needDrift {
			// still mark drift as last_error when applied != desired? spec says present=false, but keep clear for update case
			upd = upd.Set("last_error = NULL")
		} else {
			upd = upd.Set("last_error = NULL")
		}
		_, _ = upd.Exec(r.Context())

		if needDrift {
			entry := map[string]interface{}{
				"credential_id": d.CredentialID,
				"endpoint_id":   d.EndpointID,
				"action":        action,
				"desired_state": desired,
				"revision":      d.Revision,
			}
			if reason != "" {
				entry["reason"] = reason
			}
			if applied != desired {
				entry["applied_state"] = applied
			}
			drift = append(drift, entry)
		}
	}

	// Detect extra items on agent that have no desired deployment -> need remove
	for key, rep := range reportMap {
		if !rep.Present {
			continue
		}
		found := false
		for _, d := range deps {
			if fmt.Sprintf("%s:%d", d.CredentialID, d.EndpointID) == key {
				found = true
				break
			}
		}
		if !found {
			drift = append(drift, map[string]interface{}{
				"credential_id": rep.CredentialID,
				"endpoint_id":   rep.EndpointID,
				"action":        "remove",
				"reason":        "not_desired",
			})
			logger.WarnContext(r.Context(), "agent state sync extra deployment on agent",
				"agent_id", agent.ID,
				"credential_id", rep.CredentialID,
				"endpoint_id", rep.EndpointID,
			)
		}
	}

	logger.InfoContext(r.Context(), "agent state sync completed",
		"agent_id", agent.ID,
		"endpoint_id", primaryEndpointID,
		"drift_count", len(drift),
		"conflicts", len(conflicts),
	)

	if len(drift) > 0 {
		logger.WarnContext(r.Context(), "agent drift detected",
			"agent_id", agent.ID,
			"endpoint_id", primaryEndpointID,
			"drift_count", len(drift),
		)
	}

	RespondServiceJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"drift":         drift,
			"conflicts":     conflicts,
			"next_sync_sec": 30,
			"server_time":   now.Format(time.RFC3339),
		},
	})
}

func respondAgentErr(w http.ResponseWriter, status int, errCode, msg string) {
	RespondServiceJSON(w, status, map[string]interface{}{
		"code":    status,
		"message": msg,
		"error":   errCode,
	})
}
