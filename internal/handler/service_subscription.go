package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
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

func grantReadiness(grant *model.AccessGrant, endpointActive bool) map[string]bool {
	// Agent/probe not wired in this phase; report what is knowable.
	agentOnline := false
	return map[string]bool{
		"endpoint_active": endpointActive,
		"agent_online":    agentOnline,
		"route_ok":        endpointActive && grant != nil && (grant.Format == nil || *grant.Format != "landing"),
		"egress_ip_match": endpointActive && grant != nil && (grant.Format == nil || *grant.Format != "landing"),
	}
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
		"user_id":         user.ID,
		"credential_id":   grant.CredentialID,
		"sub_urls":        subURLsForToken(r, user.Token),
		"readiness":       grantReadiness(grant, endpointActive),
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
	if req.ExternalID == "" || req.OrderItemID == "" || req.EndpointID <= 0 || (req.Format != "socks" && req.Format != "landing") {
		RespondServiceCodedError(w, http.StatusBadRequest, 40001, "INVALID_ARGUMENT", "external_id, order_item_id, endpoint_id, format(socks|landing) are required")
		return
	}
	if req.Format == "socks" {
		req.ExitRouteTag = ""
	}
	if req.DurationSeconds < 0 {
		req.DurationSeconds = 0
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
		RespondServiceSuccess(w, activateResponse(r, &existing, &user, existing.Status == model.AccessGrantStatusActive, endpointActive))
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
		Status:       model.CredentialStatusActive,
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
		DisplayName:       &email,
		Status:            model.AccessGrantStatusActive,
		Format:            &req.Format,
		ExitRouteTag:      &req.ExitRouteTag,
		IsURLTest:         true,
		OverrideConfig:    "{}",
		DeliveredAt:       &deliveredAt,
		ExpiresAt:         expiresAt,
		TrafficQuotaBytes: req.TrafficQuotaBytes,
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
			RespondServiceSuccess(w, activateResponse(r, &existing, &u, existing.Status == model.AccessGrantStatusActive, endpointActive))
			return
		}
		log.Printf("activate: create grant failed order_item_id=%s error=%v", req.OrderItemID, err)
		RespondServiceError(w, http.StatusInternalServerError, "failed to create grant")
		return
	}

	// Best-effort desired_state for agent bindings (Agent may be offline).
	createDesiredDeployments(r.Context(), grant.CredentialID, grant.EndpointID, model.CredentialStatusActive)

	// Readiness per §4.6: endpoint+credential+grant ready => delivered.
	delivered := endpointActive && cred.Status == model.CredentialStatusActive && grant.Status == model.AccessGrantStatusActive
	if !delivered {
		reason := "endpoint_inactive"
		RespondServiceJSON(w, http.StatusAccepted, map[string]interface{}{
			"code":            202,
			"message":         "not ready",
			"delivered":       false,
			"subscription_id": grant.ID,
			"reason":          reason,
			"retryable":       true,
		})
		return
	}
	RespondServiceSuccess(w, activateResponse(r, grant, user, true, endpointActive))
}

func createDesiredDeployments(ctx context.Context, credentialID string, endpointID int64, desired string) {
	var bindings []*model.EndpointAgentBinding
	if err := db.DB.NewSelect().Model(&bindings).
		Where("endpoint_id = ? AND status = ?", endpointID, "active").Scan(ctx); err != nil {
		return
	}
	for _, b := range bindings {
		dep := &model.CredentialDeployment{
			CredentialID: credentialID, EndpointID: endpointID, AgentNodeID: b.AgentNodeID,
			DesiredState: desired, AppliedState: "unknown",
		}
		_, _ = db.DB.NewInsert().Model(dep).
			On("CONFLICT (credential_id, endpoint_id, agent_node_id) DO UPDATE").
			Set("desired_state = ?", desired).Exec(ctx)
	}
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
		"readiness": grantReadiness(&grant, endpointActive),
		"sub_urls":  subURLsForToken(r, user.Token),
		"traffic": map[string]interface{}{
			"quota_bytes": grant.TrafficQuotaBytes,
			"used_bytes":  grant.TrafficUsedBytes,
			"updated_at":  nil,
		},
	}
	if grant.ExpiresAt != nil {
		data["expires_at"] = grant.ExpiresAt.UTC().Format(time.RFC3339)
	} else {
		data["expires_at"] = nil
	}
	if grant.TrafficUpdatedAt != nil {
		data["traffic"].(map[string]interface{})["updated_at"] = grant.TrafficUpdatedAt.UTC().Format(time.RFC3339)
	}
	RespondServiceSuccess(w, data)
}

func ExtendSubscription(w http.ResponseWriter, r *http.Request) {
	RespondServiceCodedError(w, http.StatusNotImplemented, 50001, "NOT_IMPLEMENTED", "not implemented")
}

func QuotaSubscription(w http.ResponseWriter, r *http.Request) {
	RespondServiceCodedError(w, http.StatusNotImplemented, 50001, "NOT_IMPLEMENTED", "not implemented")
}

func RevokeSubscription(w http.ResponseWriter, r *http.Request) {
	RespondServiceCodedError(w, http.StatusNotImplemented, 50001, "NOT_IMPLEMENTED", "not implemented")
}
