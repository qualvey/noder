package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"
	"noder/internal/db"
	"noder/internal/model"
)

// RegisterServiceReleaseLogRoutes exposes admin-facing read APIs for service_release_logs.
// Best-practices:
//   - AdminAuth only (X-Admin-Token)
//   - Offset pagination (page/limit) with total + total_pages (compatible with AntD/ElementPlus)
//   - Strict validation, clamped limit, RFC3339/epoch time parsing
//   - Filter push-down via indexed columns (subscription_id, endpoint_id, created_at)
//   - Stable ordering: created_at DESC, id DESC
//   - Unified envelope {code, message, data}
func RegisterServiceReleaseLogRoutes(r chi.Router) {
	r.Route("/api/admin/service-release-logs", func(r chi.Router) {
		r.Use(AdminAuth)
		r.Get("/", ListServiceReleaseLogs)
		r.Get("/stats", GetServiceReleaseLogStats)
		r.Get("/{id}", GetServiceReleaseLog)
	})
}

type serviceReleaseLogItem struct {
	ID              int64     `json:"id"`
	SubscriptionID  string    `json:"subscription_id"`
	CredentialID    *string   `json:"credential_id,omitempty"`
	EndpointID      int64     `json:"endpoint_id"`
	ExternalRef     *string   `json:"external_ref,omitempty"`
	CallerIP        string    `json:"caller_ip"`
	RequestID       *string   `json:"request_id,omitempty"`
	HttpStatus      int       `json:"http_status"`
	BusinessCode    int       `json:"business_code"`
	ErrorCode       *string   `json:"error_code,omitempty"`
	Message         *string   `json:"message,omitempty"`
	Delivered       bool      `json:"delivered"`
	DeploymentCount int       `json:"deployment_count"`
	AgentOnline     bool      `json:"agent_online"`
	LatencyMs       int64     `json:"latency_ms"`
	CreatedAt       time.Time `json:"created_at"`
}

// ListServiceReleaseLogs GET /api/admin/service-release-logs
// Query: page, limit|page_size, subscription_id, endpoint_id, credential_id, external_ref, caller_ip, request_id,
// http_status, business_code, error_code, delivered, agent_online, since, until, q
// since/until: RFC3339 / RFC3339Nano / YYYY-MM-DD / "2006-01-02 15:04:05" / unix millis|seconds
func ListServiceReleaseLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	page, _ := strconv.Atoi(strings.TrimSpace(q.Get("page")))
	limit, _ := strconv.Atoi(strings.TrimSpace(q.Get("limit")))
	if limit == 0 {
		if v := strings.TrimSpace(q.Get("page_size")); v != "" {
			limit, _ = strconv.Atoi(v)
		}
	}
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

	subscriptionID := strings.TrimSpace(q.Get("subscription_id"))
	endpointIDStr := strings.TrimSpace(q.Get("endpoint_id"))
	credentialID := strings.TrimSpace(q.Get("credential_id"))
	externalRef := strings.TrimSpace(q.Get("external_ref"))
	callerIP := strings.TrimSpace(q.Get("caller_ip"))
	requestID := strings.TrimSpace(q.Get("request_id"))
	httpStatusStr := strings.TrimSpace(q.Get("http_status"))
	businessCodeStr := strings.TrimSpace(q.Get("business_code"))
	errorCode := strings.TrimSpace(q.Get("error_code"))
	deliveredStr := strings.TrimSpace(q.Get("delivered"))
	agentOnlineStr := strings.TrimSpace(q.Get("agent_online"))
	keyword := strings.TrimSpace(q.Get("q"))
	sinceStr := strings.TrimSpace(q.Get("since"))
	untilStr := strings.TrimSpace(q.Get("until"))

	var endpointID int64
	if endpointIDStr != "" {
		if v, err := strconv.ParseInt(endpointIDStr, 10, 64); err == nil {
			endpointID = v
		} else {
			RespondError(w, http.StatusBadRequest, "invalid endpoint_id")
			return
		}
	}
	var httpStatus int
	if httpStatusStr != "" {
		if v, err := strconv.Atoi(httpStatusStr); err == nil {
			httpStatus = v
		} else {
			RespondError(w, http.StatusBadRequest, "invalid http_status")
			return
		}
	}
	var businessCode int
	hasBusinessCode := false
	if businessCodeStr != "" {
		if v, err := strconv.Atoi(businessCodeStr); err == nil {
			businessCode = v
			hasBusinessCode = true
		} else {
			RespondError(w, http.StatusBadRequest, "invalid business_code")
			return
		}
	}
	var deliveredFilter *bool
	if deliveredStr != "" {
		if !(strings.EqualFold(deliveredStr, "true") || strings.EqualFold(deliveredStr, "false") || deliveredStr == "1" || deliveredStr == "0") {
			RespondError(w, http.StatusBadRequest, "delivered must be true|false|1|0")
			return
		}
		b := strings.EqualFold(deliveredStr, "true") || deliveredStr == "1"
		deliveredFilter = &b
	}
	var agentOnlineFilter *bool
	if agentOnlineStr != "" {
		if !(strings.EqualFold(agentOnlineStr, "true") || strings.EqualFold(agentOnlineStr, "false") || agentOnlineStr == "1" || agentOnlineStr == "0") {
			RespondError(w, http.StatusBadRequest, "agent_online must be true|false|1|0")
			return
		}
		b := strings.EqualFold(agentOnlineStr, "true") || agentOnlineStr == "1"
		agentOnlineFilter = &b
	}

	since := parseReleaseLogTime(sinceStr, true)
	until := parseReleaseLogTime(untilStr, false)
	if sinceStr != "" && since == nil {
		RespondError(w, http.StatusBadRequest, "invalid since (expect RFC3339 or YYYY-MM-DD or epoch)")
		return
	}
	if untilStr != "" && until == nil {
		RespondError(w, http.StatusBadRequest, "invalid until (expect RFC3339 or YYYY-MM-DD or epoch)")
		return
	}
	if since != nil && until != nil && since.After(*until) {
		RespondError(w, http.StatusBadRequest, "since must be <= until")
		return
	}

	applyFilters := func(sel *bun.SelectQuery) *bun.SelectQuery {
		if subscriptionID != "" {
			sel = sel.Where("service_release_log.subscription_id = ?", subscriptionID)
		}
		if endpointID != 0 {
			sel = sel.Where("service_release_log.endpoint_id = ?", endpointID)
		}
		if credentialID != "" {
			sel = sel.Where("service_release_log.credential_id = ?", credentialID)
		}
		if externalRef != "" {
			sel = sel.Where("service_release_log.external_ref = ?", externalRef)
		}
		if callerIP != "" {
			sel = sel.Where("service_release_log.caller_ip LIKE ?", "%"+callerIP+"%")
		}
		if requestID != "" {
			sel = sel.Where("service_release_log.request_id = ?", requestID)
		}
		if httpStatusStr != "" {
			sel = sel.Where("service_release_log.http_status = ?", httpStatus)
		}
		if hasBusinessCode {
			sel = sel.Where("service_release_log.business_code = ?", businessCode)
		}
		if errorCode != "" {
			sel = sel.Where("service_release_log.error_code = ?", errorCode)
		}
		if deliveredFilter != nil {
			sel = sel.Where("service_release_log.delivered = ?", *deliveredFilter)
		}
		if agentOnlineFilter != nil {
			sel = sel.Where("service_release_log.agent_online = ?", *agentOnlineFilter)
		}
		if since != nil {
			sel = sel.Where("service_release_log.created_at >= ?", *since)
		}
		if until != nil {
			sel = sel.Where("service_release_log.created_at <= ?", *until)
		}
		if keyword != "" {
			kw := "%" + keyword + "%"
			sel = sel.Where("(service_release_log.subscription_id LIKE ? OR service_release_log.error_code LIKE ? OR service_release_log.message LIKE ?)", kw, kw, kw)
		}
		return sel
	}

	qCount := applyFilters(db.DB.NewSelect().Model((*model.ServiceReleaseLog)(nil)))
	qItems := applyFilters(db.DB.NewSelect().Model((*model.ServiceReleaseLog)(nil)))

	total, err := qCount.Count(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var rows []*model.ServiceReleaseLog
	if err := qItems.OrderExpr("service_release_log.created_at DESC, service_release_log.id DESC").Limit(limit).Offset(offset).Scan(r.Context(), &rows); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]serviceReleaseLogItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, serviceReleaseLogItem{
			ID:              row.ID,
			SubscriptionID:  row.SubscriptionID,
			CredentialID:    row.CredentialID,
			EndpointID:      row.EndpointID,
			ExternalRef:     row.ExternalRef,
			CallerIP:        row.CallerIP,
			RequestID:       row.RequestID,
			HttpStatus:      row.HttpStatus,
			BusinessCode:    row.BusinessCode,
			ErrorCode:       row.ErrorCode,
			Message:         row.Message,
			Delivered:       row.Delivered,
			DeploymentCount: row.DeploymentCount,
			AgentOnline:     row.AgentOnline,
			LatencyMs:       row.LatencyMs,
			CreatedAt:       row.CreatedAt,
		})
	}
	if items == nil {
		items = []serviceReleaseLogItem{}
	}

	totalPages := 0
	if limit > 0 {
		totalPages = (total + limit - 1) / limit
	}

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"items":       items,
			"total":       total,
			"page":        page,
			"limit":       limit,
			"page_size":   limit,
			"total_pages": totalPages,
		},
	})
}

// GetServiceReleaseLog GET /api/admin/service-release-logs/{id}
func GetServiceReleaseLog(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		RespondError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var row model.ServiceReleaseLog
	if err := db.DB.NewSelect().Model(&row).Where("id = ?", id).Scan(r.Context()); err != nil {
		RespondJSON(w, http.StatusNotFound, map[string]interface{}{"code": 40402, "error": "LOG_NOT_FOUND", "message": "log not found"})
		return
	}
	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": serviceReleaseLogItem{
			ID:              row.ID,
			SubscriptionID:  row.SubscriptionID,
			CredentialID:    row.CredentialID,
			EndpointID:      row.EndpointID,
			ExternalRef:     row.ExternalRef,
			CallerIP:        row.CallerIP,
			RequestID:       row.RequestID,
			HttpStatus:      row.HttpStatus,
			BusinessCode:    row.BusinessCode,
			ErrorCode:       row.ErrorCode,
			Message:         row.Message,
			Delivered:       row.Delivered,
			DeploymentCount: row.DeploymentCount,
			AgentOnline:     row.AgentOnline,
			LatencyMs:       row.LatencyMs,
			CreatedAt:       row.CreatedAt,
		},
	})
}

// GetServiceReleaseLogStats GET /api/admin/service-release-logs/stats?since=&until=
// Returns aggregated counts for dashboard.
func GetServiceReleaseLogStats(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	since := parseReleaseLogTime(strings.TrimSpace(q.Get("since")), true)
	until := parseReleaseLogTime(strings.TrimSpace(q.Get("until")), false)

	baseFilters := func(sel *bun.SelectQuery) *bun.SelectQuery {
		if since != nil {
			sel = sel.Where("service_release_log.created_at >= ?", *since)
		}
		if until != nil {
			sel = sel.Where("service_release_log.created_at <= ?", *until)
		}
		return sel
	}

	base := baseFilters(db.DB.NewSelect().Model((*model.ServiceReleaseLog)(nil)))
	total, _ := base.Count(r.Context())

	successCnt, _ := baseFilters(db.DB.NewSelect().Model((*model.ServiceReleaseLog)(nil))).Where("service_release_log.http_status = ?", 200).Where("service_release_log.business_code = ?", 0).Count(r.Context())
	bindingMissingCnt, _ := baseFilters(db.DB.NewSelect().Model((*model.ServiceReleaseLog)(nil))).Where("service_release_log.error_code = ?", "BINDING_MISSING").Count(r.Context())
	notFoundCnt, _ := baseFilters(db.DB.NewSelect().Model((*model.ServiceReleaseLog)(nil))).Where("service_release_log.business_code = ?", 40402).Count(r.Context())
	deliveredCnt, _ := baseFilters(db.DB.NewSelect().Model((*model.ServiceReleaseLog)(nil))).Where("service_release_log.delivered = ?", true).Count(r.Context())
	agentOfflineCnt, _ := baseFilters(db.DB.NewSelect().Model((*model.ServiceReleaseLog)(nil))).Where("service_release_log.agent_online = ?", false).Count(r.Context())

	RespondJSON(w, http.StatusOK, map[string]interface{}{
		"code":    0,
		"message": "success",
		"data": map[string]interface{}{
			"total":           total,
			"success":         successCnt,
			"binding_missing": bindingMissingCnt,
			"not_found":       notFoundCnt,
			"delivered":       deliveredCnt,
			"agent_offline":   agentOfflineCnt,
		},
	})
}

func parseReleaseLogTime(s string, isStart bool) *time.Time {
	if s == "" {
		return nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		u := t.UTC()
		return &u
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		u := t.UTC()
		return &u
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		u := t.UTC()
		if !isStart {
			u = u.Add(24*time.Hour - time.Nanosecond)
		}
		return &u
	}
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		u := t.UTC()
		return &u
	}
	if ms, err := strconv.ParseInt(s, 10, 64); err == nil {
		var t time.Time
		if ms > 1e12 {
			t = time.UnixMilli(ms).UTC()
		} else {
			t = time.Unix(ms, 0).UTC()
		}
		return &t
	}
	return nil
}
