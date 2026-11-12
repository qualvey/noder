package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"noder/internal/db"
	"noder/internal/model"
)

type activateReq struct {
	ExternalID        string `json:"external_id"`
	EndpointID        int64  `json:"endpoint_id"`
	Format            string `json:"format"` // socks | landing
	ExitRouteTag      string `json:"exit_route_tag"`
	DurationSeconds   int64  `json:"duration_seconds"`
	TrafficQuotaBytes int64  `json:"traffic_quota_bytes"`
	OrderItemID       string `json:"order_item_id"`
	TagOverride       string `json:"tag_override"`
}

type extendReq struct {
	DurationSeconds   int64  `json:"duration_seconds"`
	TrafficQuotaBytes int64  `json:"traffic_quota_bytes"`
	OrderItemID       string `json:"order_item_id"`
}

type quotaReq struct {
	DeltaBytes  int64  `json:"delta_bytes"`
	OrderItemID string `json:"order_item_id"`
}

type revokeReq struct {
	Reason string `json:"reason"`
}

func subURLsForToken(r *http.Request, token string) map[string]string {
	baseURL := getBaseURL(r)
	return map[string]string{
		"singbox": fmt.Sprintf("%s/sub?token=%s", baseURL, token),
		"mihomo":  fmt.Sprintf("%s/sub?token=%s&target=mihomo", baseURL, token),
		"xray":    fmt.Sprintf("%s/xray?token=%s", baseURL, token),
	}
}

func grantReadiness(ctx context.Context, grant *model.AccessGrant, endpointActive bool) map[string]bool {
	agentOnline := false
	if grant != nil && grant.EndpointID > 0 {
		agentOnline = isAgentOnlineForEndpoint(ctx, grant.EndpointID)
	}
	return map[string]bool{
		"endpoint_active": endpointActive,
		"agent_online":    agentOnline,
		"route_ok":        endpointActive && grant != nil && (grant.Format == nil || *grant.Format != "landing"),
		"egress_ip_match": endpointActive && grant != nil && (grant.Format == nil || *grant.Format != "landing"),
	}
}

func isAgentOnlineForEndpoint(ctx context.Context, endpointID int64) bool {
	cutoff := time.Now().UTC().Add(-90 * time.Second)
	exists, err := db.DB.NewSelect().
		TableExpr("endpoint_agent_bindings AS b").
		Join("JOIN agent_nodes AS a ON a.id = b.agent_node_id").
		Where("b.endpoint_id = ? AND b.status = ?", endpointID, "active").
		Where("a.status != ?", "disabled").
		Where("a.last_heartbeat_at > ?", cutoff).
		Exists(ctx)
	return err == nil && exists
}

func hasDeployment(ctx context.Context, credentialID string, endpointID int64) bool {
	n, err := db.DB.NewSelect().Model((*model.CredentialDeployment)(nil)).
		Where("credential_id = ? AND endpoint_id = ?", credentialID, endpointID).Count(ctx)
	return err == nil && n > 0
}

func isDelivered(ctx context.Context, grant *model.AccessGrant, endpointActive bool) bool {
	if grant == nil || grant.Status != model.AccessGrantStatusActive || !endpointActive {
		return false
	}
	// socks 直连落地机，无 agent 部署链路
	if grant.Format != nil && *grant.Format == "socks" {
		return true
	}
	if !isAgentOnlineForEndpoint(ctx, grant.EndpointID) {
		return false
	}
	return hasDeployment(ctx, grant.CredentialID, grant.EndpointID)
}

func effectiveGrantStatus(grant *model.AccessGrant, now time.Time) string {
	if grant == nil {
		return ""
	}
	if grant.Status == model.AccessGrantStatusRevoked {
		return model.AccessGrantStatusRevoked
	}
	if grant.ExpiresAt != nil && !grant.ExpiresAt.After(now) {
		return model.AccessGrantStatusExpired
	}
	return grant.Status
}

func activateResponse(r *http.Request, grant *model.AccessGrant, user *model.User, delivered bool, endpointActive bool) map[string]interface{} {
	resp := map[string]interface{}{
		"delivered":       delivered,
		"subscription_id": grant.ID,
		"status":          grant.Status,
		"user_id":         user.ID,
		"credential_id":   grant.CredentialID,
		"sub_urls":        subURLsForToken(r, user.Token),
		"readiness":       grantReadiness(r.Context(), grant, endpointActive),
	}
	if grant.DeliveredAt != nil {
		resp["delivered_at"] = grant.DeliveredAt.UTC().Format(time.RFC3339)
	}
	if grant.ExpiresAt != nil {
		resp["expires_at"] = grant.ExpiresAt.UTC().Format(time.RFC3339)
	}
	return resp
}

func findOrCreateServiceUser(ctx context.Context, externalID, name, remark string) (*model.User, error) {
	var existing model.User
	if err := db.DB.NewSelect().Model(&existing).Where("external_id = ?", externalID).Scan(ctx); err == nil {
		return &existing, nil
	}
	token := randomHex(32)
	if strings.TrimSpace(name) == "" {
		name = externalID
	}
	u := &model.User{ExternalID: &externalID, Name: name, Token: token, IsActive: true}
	if remark != "" {
		rmk := remark
		u.Remark = &rmk
	}
	if _, err := db.DB.NewInsert().Model(u).Exec(ctx); err != nil {
		// Race: retry lookup.
		if err2 := db.DB.NewSelect().Model(&existing).Where("external_id = ?", externalID).Scan(ctx); err2 == nil {
			return &existing, nil
		}
		return nil, err
	}
	return u, nil
}

func ActivateSubscription(w http.ResponseWriter, r *http.Request) {
	var req activateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondServiceError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	req.ExternalID = strings.TrimSpace(req.ExternalID)
	req.Format = strings.ToLower(strings.TrimSpace(req.Format))
	req.ExitRouteTag = strings.TrimSpace(req.ExitRouteTag)
	req.OrderItemID = strings.TrimSpace(req.OrderItemID)
	req.TagOverride = strings.TrimSpace(req.TagOverride)
	if req.ExternalID == "" || req.OrderItemID == "" || req.EndpointID <= 0 || (req.Format != "socks" && req.Format != "landing") {
		RespondServiceCodedError(w, http.StatusBadRequest, 40001, "INVALID_ARGUMENT", "external_id, order_item_id, endpoint_id, format(socks|landing) are required")
		return
	}
	if len(req.TagOverride) > 64 {
		RespondServiceCodedError(w, http.StatusBadRequest, 40001, "INVALID_ARGUMENT", "tag_override too long (max 64)")
		return
	}
	if req.Format == "socks" && req.ExitRouteTag != "" {
		RespondServiceCodedError(w, http.StatusBadRequest, 40001, "INVALID_ARGUMENT", "exit_route_tag must be empty for socks")
		return
	}
	if req.Format == "landing" && req.ExitRouteTag == "" {
		RespondServiceCodedError(w, http.StatusBadRequest, 40001, "INVALID_ARGUMENT", "exit_route_tag is required for landing")
		return
	}
	if req.DurationSeconds <= 0 {
		RespondServiceCodedError(w, http.StatusBadRequest, 40001, "INVALID_ARGUMENT", "duration_seconds must be > 0")
		return
	}
	if req.TrafficQuotaBytes < 0 {
		RespondServiceCodedError(w, http.StatusBadRequest, 40001, "INVALID_ARGUMENT", "traffic_quota_bytes must be >= 0")
		return
	}
	if req.Format == "socks" && req.TrafficQuotaBytes != 0 {
		RespondServiceCodedError(w, http.StatusBadRequest, 40001, "INVALID_ARGUMENT", "traffic_quota_bytes must be 0 for socks (unlimited)")
		return
	}

	// Idempotency: existing grant for this order_item_id.
	var existing model.AccessGrant
	if err := db.DB.NewSelect().Model(&existing).Relation("Credential").Relation("User").
		Where("access_grant.external_ref = ?", req.OrderItemID).Scan(r.Context()); err == nil {
		var user model.User
		if existing.User != nil {
			user = *existing.User
		} else {
			_ = db.DB.NewSelect().Model(&user).Where("id = ?", existing.UserID).Scan(r.Context())
		}
		var ep model.Node
		endpointActive := db.DB.NewSelect().Model(&ep).Where("id = ?", existing.EndpointID).Scan(r.Context()) == nil &&
			ep.IsActive && (ep.Status == "" || ep.Status == model.AccessGrantStatusActive)
		RespondServiceSuccess(w, activateResponse(r, &existing, &user, isDelivered(r.Context(), &existing, endpointActive), endpointActive))
		return
	}

	user, err := findOrCreateServiceUser(r.Context(), req.ExternalID, req.ExternalID, "")
	if err != nil {
		log.Printf("activate: find/create user failed order_item_id=%s error=%v", req.OrderItemID, err)
		RespondServiceError(w, http.StatusInternalServerError, "failed to resolve user")
		return
	}

	var endpoint model.Node
	if err := db.DB.NewSelect().Model(&endpoint).Where("id = ?", req.EndpointID).Scan(r.Context()); err != nil {
		log.Printf("activate: endpoint not found order_item_id=%s endpoint_id=%d", req.OrderItemID, req.EndpointID)
		RespondServiceCodedError(w, http.StatusNotFound, 40401, "ENDPOINT_NOT_FOUND", "endpoint not found")
		return
	}
	endpointActive := endpoint.IsActive && (endpoint.Status == "" || endpoint.Status == model.AccessGrantStatusActive)

	now := time.Now().UTC()
	deliveredAt := now
	var expiresAt *time.Time
	if req.DurationSeconds > 0 {
		t := now.Add(time.Duration(req.DurationSeconds) * time.Second)
		expiresAt = &t
	}

	credID := uuid.NewString()
	credUUID := uuid.NewString()
	email := fmt.Sprintf("c_%s@socks", credID)
	if req.Format == "landing" {
		email = fmt.Sprintf("c_%s@%s", credID, req.ExitRouteTag)
	}
	scheme := req.Format
	if scheme == "landing" {
		scheme = "vless"
	}
	cred := &model.Credential{
		ID:           credID,
		UserID:       user.ID,
		Scheme:       scheme,
		DisplayName:  &email,
		Username:     &email,
		UUIDValue:    &credUUID,
		Status:       model.CredentialStatusPendingBinding, // activated on manual release
		SecretConfig: "{}",
		ExpiresAt:    expiresAt,
	}
	if req.Format == "socks" {
		uname := fmt.Sprintf("u%d_%s", user.ID, strings.ReplaceAll(credID[:8], "-", ""))
		pwd := "pwd_" + randomHex(12)
		cred.Username = &uname
		cred.PasswordCiphertext = &pwd
	}
	if _, err := db.DB.NewInsert().Model(cred).Exec(r.Context()); err != nil {
		log.Printf("activate: create credential failed order_item_id=%s error=%v", req.OrderItemID, err)
		RespondServiceError(w, http.StatusInternalServerError, "failed to create credential")
		return
	}

	grant := &model.AccessGrant{
		ID:                uuid.NewString(),
		UserID:            user.ID,
		EndpointID:        req.EndpointID,
		CredentialID:      credID,
		ExternalRef:       &req.OrderItemID,
		DisplayName:       nil,
		Status:            model.AccessGrantStatusPendingActivation, // no auto-release; manual /release required
		Format:            &req.Format,
		ExitRouteTag:      &req.ExitRouteTag,
		IsURLTest:         true,
		OverrideConfig:    "{}",
		DeliveredAt:       &deliveredAt,
		ExpiresAt:         expiresAt,
		TrafficQuotaBytes: req.TrafficQuotaBytes,
	}
	if req.TagOverride != "" {
		tag := req.TagOverride
		grant.TagOverride = &tag
	} else {
		// 方案 A（D10 定稿）：同出口多份订阅自动生成 tag_override
		// #N = 该用户该 (endpoint_id, exit_route_tag) 下历史 grant 数 + 1，首份无后缀，永不重用
		histCount, err := db.DB.NewSelect().Model((*model.AccessGrant)(nil)).
			Where("user_id = ? AND endpoint_id = ? AND exit_route_tag = ?", user.ID, req.EndpointID, req.ExitRouteTag).
			Count(r.Context())
		if err == nil && histCount > 0 {
			tag := fmt.Sprintf("%s #%d", endpoint.Tag, histCount+1)
			grant.TagOverride = &tag
		}
	}
	if _, err := db.DB.NewInsert().Model(grant).Exec(r.Context()); err != nil {
		// Idempotency race: DB unique on external_ref.
		if err2 := db.DB.NewSelect().Model(&existing).Relation("Credential").Relation("User").
			Where("access_grant.external_ref = ?", req.OrderItemID).Scan(r.Context()); err2 == nil {
			var u model.User
			if existing.User != nil {
				u = *existing.User
			} else {
				_ = db.DB.NewSelect().Model(&u).Where("id = ?", existing.UserID).Scan(r.Context())
			}
			var ep2 model.Node
			epActive2 := db.DB.NewSelect().Model(&ep2).Where("id = ?", existing.EndpointID).Scan(r.Context()) == nil && ep2.IsActive && (ep2.Status == "" || ep2.Status == model.AccessGrantStatusActive)
			RespondServiceSuccess(w, activateResponse(r, &existing, &u, isDelivered(r.Context(), &existing, epActive2), epActive2))
			return
		}
		log.Printf("activate: create grant failed order_item_id=%s error=%v", req.OrderItemID, err)
		RespondServiceError(w, http.StatusInternalServerError, "failed to create grant")
		return
	}

	// Do not auto-deploy or auto-activate: manual release required before client use.
	RespondServiceSuccess(w, activateResponse(r, grant, user, false, endpointActive))
}

func createDesiredDeployments(ctx context.Context, credentialID string, endpointID int64, desired string) int {
	var bindings []*model.EndpointAgentBinding
	if err := db.DB.NewSelect().Model(&bindings).
		Where("endpoint_id = ? AND status = ?", endpointID, "active").Scan(ctx); err != nil {
		log.Printf("WARN createDesiredDeployments: query bindings failed endpoint_id=%d error=%v", endpointID, err)
		return 0
	}
	if len(bindings) == 0 {
		log.Printf("WARN createDesiredDeployments: no active bindings endpoint_id=%d desired=%s credential_id=%s", endpointID, desired, credentialID)
		return 0
	}
	rev := strconv.FormatInt(time.Now().UnixMilli(), 10)
	for _, b := range bindings {
		dep := &model.CredentialDeployment{
			CredentialID: credentialID, EndpointID: endpointID, AgentNodeID: b.AgentNodeID,
			DesiredState: desired, AppliedState: "unknown", Revision: rev,
		}
		_, _ = db.DB.NewInsert().Model(dep).
			On("CONFLICT (credential_id, endpoint_id, agent_node_id) DO UPDATE").
			Set("desired_state = ?", desired).
			Set("revision = ?", rev).
			Exec(ctx)
	}
	return len(bindings)
}

func GetSubscription(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		RespondServiceCodedError(w, http.StatusBadRequest, 40001, "INVALID_ARGUMENT", "id is required")
		return
	}
	var grant model.AccessGrant
	if err := db.DB.NewSelect().Model(&grant).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondServiceCodedError(w, http.StatusNotFound, 40402, "SUBSCRIPTION_NOT_FOUND", "subscription not found")
		return
	}
	var user model.User
	if err := db.DB.NewSelect().Model(&user).Where("id = ?", grant.UserID).Scan(r.Context()); err != nil {
		RespondServiceError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	var ep model.Node
	endpointActive := db.DB.NewSelect().Model(&ep).Where("id = ?", grant.EndpointID).Scan(r.Context()) == nil &&
		ep.IsActive && (ep.Status == "" || ep.Status == model.AccessGrantStatusActive)

	now := time.Now().UTC()
	status := effectiveGrantStatus(&grant, now)
	data := map[string]interface{}{
		"status":    status,
		"readiness": grantReadiness(r.Context(), &grant, endpointActive),
		"sub_urls":  subURLsForToken(r, user.Token),
		"traffic": map[string]interface{}{
			"quota_bytes": grant.TrafficQuotaBytes,
			"used_bytes":  grant.TrafficUsedBytes,
			"updated_at":  nil,
		},
	}
	if grant.ExpiresAt != nil {
		data["expires_at"] = grant.ExpiresAt.UTC().Format(time.RFC3339)
	}
	if grant.TrafficUpdatedAt != nil {
		data["traffic"].(map[string]interface{})["updated_at"] = grant.TrafficUpdatedAt.UTC().Format(time.RFC3339)
	}
	RespondServiceSuccess(w, data)
}

func ExtendSubscription(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	var req extendReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondServiceError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	req.OrderItemID = strings.TrimSpace(req.OrderItemID)
	if id == "" || req.OrderItemID == "" {
		RespondServiceCodedError(w, http.StatusBadRequest, 40001, "INVALID_ARGUMENT", "id and order_item_id are required")
		return
	}
	if req.DurationSeconds < 0 {
		req.DurationSeconds = 0
	}

	var grant model.AccessGrant
	if err := db.DB.NewSelect().Model(&grant).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondServiceCodedError(w, http.StatusNotFound, 40402, "SUBSCRIPTION_NOT_FOUND", "subscription not found")
		return
	}
	now := time.Now().UTC()
	status := effectiveGrantStatus(&grant, now)
	if grant.Status == model.AccessGrantStatusRevoked {
		RespondServiceCodedError(w, http.StatusConflict, 40901, "SUBSCRIPTION_REVOKED", "subscription is revoked")
		return
	}
	if status != model.AccessGrantStatusActive && status != model.AccessGrantStatusExpired &&
		status != model.AccessGrantStatusTrafficExhausted && status != model.AccessGrantStatusPendingActivation {
		RespondServiceCodedError(w, http.StatusConflict, 40902, "INVALID_SUBSCRIPTION_STATE", fmt.Sprintf("cannot extend subscription in state %s", status))
		return
	}

	op := &model.SubscriptionOperation{OrderItemID: req.OrderItemID, GrantID: grant.ID, Operation: "extend", Result: "{}"}
	res, err := db.DB.NewInsert().Model(op).On("CONFLICT (order_item_id) DO NOTHING").Exec(r.Context())
	if err != nil {
		log.Printf("extend: record operation failed order_item_id=%s endpoint_id=%d error=%v", req.OrderItemID, grant.EndpointID, err)
		RespondServiceError(w, http.StatusInternalServerError, "failed to record operation")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Idempotent replay.
		var user model.User
		_ = db.DB.NewSelect().Model(&user).Where("id = ?", grant.UserID).Scan(r.Context())
		var ep model.Node
		endpointActive := db.DB.NewSelect().Model(&ep).Where("id = ?", grant.EndpointID).Scan(r.Context()) == nil && ep.IsActive && (ep.Status == "" || ep.Status == model.AccessGrantStatusActive)
		replay := map[string]interface{}{
			"subscription_id": grant.ID,
			"status":          effectiveGrantStatus(&grant, now),
			"sub_urls":        subURLsForToken(r, user.Token),
			"readiness":       grantReadiness(r.Context(), &grant, endpointActive),
			"traffic": map[string]interface{}{
				"quota_bytes": grant.TrafficQuotaBytes,
				"used_bytes":  grant.TrafficUsedBytes,
				"updated_at":  timePtrFormat(grant.TrafficUpdatedAt),
			},
		}
		if grant.ExpiresAt != nil {
			replay["expires_at"] = grant.ExpiresAt.UTC().Format(time.RFC3339)
		}
		RespondServiceSuccess(w, replay)
		return
	}

	base := now
	if grant.ExpiresAt != nil && grant.ExpiresAt.After(base) {
		base = *grant.ExpiresAt
	}
	newExp := base.Add(time.Duration(req.DurationSeconds) * time.Second)
	grant.ExpiresAt = &newExp
	if req.TrafficQuotaBytes > 0 {
		grant.TrafficQuotaBytes += req.TrafficQuotaBytes
	}
	// Preserve pending_activation (manual release still required); otherwise reactivate.
	if grant.Status != model.AccessGrantStatusRevoked && grant.Status != model.AccessGrantStatusPendingActivation {
		grant.Status = model.AccessGrantStatusActive
	}
	grant.UpdatedAt = now
	if _, err := db.DB.NewUpdate().Model(&grant).
		Column("expires_at", "traffic_quota_bytes", "status", "updated_at").
		WherePK().Exec(r.Context()); err != nil {
		log.Printf("extend: update grant failed order_item_id=%s endpoint_id=%d error=%v", req.OrderItemID, grant.EndpointID, err)
		RespondServiceError(w, http.StatusInternalServerError, "failed to extend subscription")
		return
	}

	var user model.User
	_ = db.DB.NewSelect().Model(&user).Where("id = ?", grant.UserID).Scan(r.Context())
	var ep model.Node
	endpointActive := db.DB.NewSelect().Model(&ep).Where("id = ?", grant.EndpointID).Scan(r.Context()) == nil && ep.IsActive && (ep.Status == "" || ep.Status == model.AccessGrantStatusActive)
	RespondServiceSuccess(w, map[string]interface{}{
		"subscription_id": grant.ID,
		"expires_at":      newExp.UTC().Format(time.RFC3339),
		"status":          grant.Status,
		"sub_urls":        subURLsForToken(r, user.Token),
		"readiness":       grantReadiness(r.Context(), &grant, endpointActive),
		"traffic": map[string]interface{}{
			"quota_bytes": grant.TrafficQuotaBytes,
			"used_bytes":  grant.TrafficUsedBytes,
			"updated_at":  timePtrFormat(grant.TrafficUpdatedAt),
		},
	})
}

func timePtrFormat(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

func QuotaSubscription(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	var req quotaReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondServiceError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	req.OrderItemID = strings.TrimSpace(req.OrderItemID)
	if id == "" || req.OrderItemID == "" {
		RespondServiceCodedError(w, http.StatusBadRequest, 40001, "INVALID_ARGUMENT", "id and order_item_id are required")
		return
	}
	if req.DeltaBytes <= 0 {
		RespondServiceCodedError(w, http.StatusBadRequest, 40002, "INVALID_DELTA", "delta_bytes must be positive")
		return
	}

	var grant model.AccessGrant
	if err := db.DB.NewSelect().Model(&grant).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondServiceCodedError(w, http.StatusNotFound, 40402, "SUBSCRIPTION_NOT_FOUND", "subscription not found")
		return
	}
	now := time.Now().UTC()
	if grant.Status == model.AccessGrantStatusRevoked {
		RespondServiceCodedError(w, http.StatusConflict, 40901, "SUBSCRIPTION_REVOKED", "subscription is revoked")
		return
	}

	op := &model.SubscriptionOperation{OrderItemID: req.OrderItemID, GrantID: grant.ID, Operation: "quota", Result: "{}"}
	res, err := db.DB.NewInsert().Model(op).On("CONFLICT (order_item_id) DO NOTHING").Exec(r.Context())
	if err != nil {
		log.Printf("quota: record operation failed order_item_id=%s endpoint_id=%d error=%v", req.OrderItemID, grant.EndpointID, err)
		RespondServiceError(w, http.StatusInternalServerError, "failed to record operation")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		RespondServiceSuccess(w, map[string]interface{}{
			"subscription_id":     grant.ID,
			"status":              effectiveGrantStatus(&grant, now),
			"traffic_quota_bytes": grant.TrafficQuotaBytes,
			"traffic_used_bytes":  grant.TrafficUsedBytes,
		})
		return
	}

	grant.TrafficQuotaBytes += req.DeltaBytes
	grant.TrafficUpdatedAt = &now
	grant.UpdatedAt = now
	reactivated := false
	if grant.Status == model.AccessGrantStatusTrafficExhausted {
		grant.Status = model.AccessGrantStatusActive
		reactivated = true
	}
	if _, err := db.DB.NewUpdate().Model(&grant).
		Column("traffic_quota_bytes", "traffic_updated_at", "status", "updated_at").
		WherePK().Exec(r.Context()); err != nil {
		log.Printf("quota: update grant failed order_item_id=%s endpoint_id=%d error=%v", req.OrderItemID, grant.EndpointID, err)
		RespondServiceError(w, http.StatusInternalServerError, "failed to add quota")
		return
	}
	if reactivated {
		createDesiredDeployments(r.Context(), grant.CredentialID, grant.EndpointID, model.CredentialStatusActive)
	}

	RespondServiceSuccess(w, map[string]interface{}{
		"subscription_id":     grant.ID,
		"status":              grant.Status,
		"traffic_quota_bytes": grant.TrafficQuotaBytes,
		"traffic_used_bytes":  grant.TrafficUsedBytes,
	})
}

func ReleaseSubscription(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if id == "" {
		RespondServiceCodedError(w, http.StatusBadRequest, 40001, "INVALID_ARGUMENT", "id is required")
		return
	}
	var grant model.AccessGrant
	if err := db.DB.NewSelect().Model(&grant).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondServiceCodedError(w, http.StatusNotFound, 40402, "SUBSCRIPTION_NOT_FOUND", "subscription not found")
		return
	}
	if grant.Status == model.AccessGrantStatusRevoked {
		RespondServiceCodedError(w, http.StatusConflict, 40901, "SUBSCRIPTION_REVOKED", "subscription is revoked")
		return
	}
	var user model.User
	if err := db.DB.NewSelect().Model(&user).Where("id = ?", grant.UserID).Scan(r.Context()); err != nil {
		RespondServiceError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	var ep model.Node
	endpointActive := db.DB.NewSelect().Model(&ep).Where("id = ?", grant.EndpointID).Scan(r.Context()) == nil &&
		ep.IsActive && (ep.Status == "" || ep.Status == model.AccessGrantStatusActive)

	// Binding check: landing 必须有 agent 绑定，否则无法下发 (spec §7.1.1 BINDING_MISSING)
	// socks 直连落地机，无需 agent
	isSocks := grant.Format != nil && *grant.Format == "socks"
	if !isSocks {
		bindingCount, _ := db.DB.NewSelect().Model((*model.EndpointAgentBinding)(nil)).Where("endpoint_id = ? AND status = ?", grant.EndpointID, "active").Count(r.Context())
		if bindingCount == 0 {
			log.Printf("WARN release: no active binding endpoint_id=%d subscription_id=%s", grant.EndpointID, grant.ID)
			RespondServiceCodedError(w, http.StatusConflict, 40902, "BINDING_MISSING", fmt.Sprintf("endpoint %d has no active agent binding", grant.EndpointID))
			return
		}
	}

	if grant.Status == model.AccessGrantStatusActive {
		if isSocks {
			RespondServiceSuccess(w, activateResponse(r, &grant, &user, endpointActive, endpointActive))
			return
		}
		n := createDesiredDeployments(r.Context(), grant.CredentialID, grant.EndpointID, model.CredentialStatusActive)
		delivered := n > 0 && isAgentOnlineForEndpoint(r.Context(), grant.EndpointID) && endpointActive
		RespondServiceSuccess(w, activateResponse(r, &grant, &user, delivered, endpointActive))
		return
	}
	if grant.Status != model.AccessGrantStatusPendingActivation && grant.Status != model.AccessGrantStatusActivationFailed {
		RespondServiceCodedError(w, http.StatusConflict, 40902, "INVALID_SUBSCRIPTION_STATE", fmt.Sprintf("cannot release subscription in state %s", grant.Status))
		return
	}

	now := time.Now().UTC()
	grant.Status = model.AccessGrantStatusActive
	grant.UpdatedAt = now
	if _, err := db.DB.NewUpdate().Model(&grant).Column("status", "updated_at").WherePK().Exec(r.Context()); err != nil {
		log.Printf("release: update grant failed subscription_id=%s endpoint_id=%d error=%v", grant.ID, grant.EndpointID, err)
		RespondServiceError(w, http.StatusInternalServerError, "failed to release subscription")
		return
	}
	if _, err := db.DB.NewUpdate().Model((*model.Credential)(nil)).
		Set("status = ?", model.CredentialStatusActive).
		Set("updated_at = ?", now).
		Where("id = ?", grant.CredentialID).Exec(r.Context()); err != nil {
		log.Printf("release: update credential failed subscription_id=%s endpoint_id=%d error=%v", grant.ID, grant.EndpointID, err)
		RespondServiceError(w, http.StatusInternalServerError, "failed to activate credential")
		return
	}
	var delivered bool
	var n int
	if isSocks {
		n = 0
		delivered = endpointActive
	} else {
		n = createDesiredDeployments(r.Context(), grant.CredentialID, grant.EndpointID, model.CredentialStatusActive)
		delivered = n > 0 && isAgentOnlineForEndpoint(r.Context(), grant.EndpointID) && endpointActive
	}
	orderRef := ""
	if grant.ExternalRef != nil {
		orderRef = *grant.ExternalRef
	}
	log.Printf("release: subscription released subscription_id=%s endpoint_id=%d order_item_ref=%s deployments=%d delivered=%v", grant.ID, grant.EndpointID, orderRef, n, delivered)
	RespondServiceSuccess(w, activateResponse(r, &grant, &user, delivered, endpointActive))
}

func RevokeSubscription(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	var req revokeReq
	_ = json.NewDecoder(r.Body).Decode(&req) // reason optional
	if id == "" {
		RespondServiceCodedError(w, http.StatusBadRequest, 40001, "INVALID_ARGUMENT", "id is required")
		return
	}

	var grant model.AccessGrant
	if err := db.DB.NewSelect().Model(&grant).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondServiceCodedError(w, http.StatusNotFound, 40402, "SUBSCRIPTION_NOT_FOUND", "subscription not found")
		return
	}
	// Already revoked => 200 (refund flow retries).
	if grant.Status == model.AccessGrantStatusRevoked {
		RespondServiceSuccess(w, map[string]interface{}{"subscription_id": grant.ID, "status": model.AccessGrantStatusRevoked})
		return
	}

	now := time.Now().UTC()
	if _, err := db.DB.NewUpdate().Model(&grant).
		Set("status = ?", model.AccessGrantStatusRevoked).
		Set("updated_at = ?", now).
		WherePK().Exec(r.Context()); err != nil {
		log.Printf("revoke: update grant failed subscription_id=%s endpoint_id=%d error=%v", grant.ID, grant.EndpointID, err)
		RespondServiceError(w, http.StatusInternalServerError, "failed to revoke subscription")
		return
	}
	if _, err := db.DB.NewUpdate().Model((*model.Credential)(nil)).
		Set("status = ?", model.CredentialStatusRevoked).
		Set("revoked_at = ?", now).
		Set("updated_at = ?", now).
		Where("id = ?", grant.CredentialID).Exec(r.Context()); err != nil {
		log.Printf("revoke: update credential failed subscription_id=%s endpoint_id=%d credential_id=%s error=%v", grant.ID, grant.EndpointID, grant.CredentialID, err)
		RespondServiceError(w, http.StatusInternalServerError, "failed to revoke credential")
		return
	}
	createDesiredDeployments(r.Context(), grant.CredentialID, grant.EndpointID, model.CredentialStatusRevoked)
	orderRef2 := ""
	if grant.ExternalRef != nil {
		orderRef2 = *grant.ExternalRef
	}
	log.Printf("revoke: subscription revoked subscription_id=%s endpoint_id=%d order_item_ref=%s reason=%s", grant.ID, grant.EndpointID, orderRef2, strings.TrimSpace(req.Reason))

	RespondServiceSuccess(w, map[string]interface{}{"subscription_id": grant.ID, "status": model.AccessGrantStatusRevoked})
}
