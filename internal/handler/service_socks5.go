package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"noder/internal/config"
	"noder/internal/db"
	"noder/internal/model"
)

type Socks5AllocateReq struct {
	Country string  `json:"country"`
	Count   int     `json:"count"`
	Remark  *string `json:"remark"`
}

type Socks5ItemResp struct {
	EndpointID  int64  `json:"endpoint_id"`
	Status      string `json:"status"`
	Country     string `json:"country"`
	IP          string `json:"ip"`
	Port        int    `json:"port"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	ProxyFormat string `json:"proxy_format"`
	ProxyURL    string `json:"proxy_url"`
}

// AllocateUserSocks5 申请分配 Socks5 节点 (支持批量 & 自动排重反亲和调度)
func AllocateUserSocks5(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		RespondServiceError(w, http.StatusBadRequest, "token is required")
		return
	}

	var user model.User
	if err := db.DB.NewSelect().Model(&user).Where("token = ? AND is_active = true", token).Scan(r.Context()); err != nil {
		RespondServiceError(w, http.StatusNotFound, "目标用户不存在或未激活")
		return
	}

	var req Socks5AllocateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondServiceError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	country := strings.ToUpper(strings.TrimSpace(req.Country))
	if country == "" {
		RespondServiceError(w, http.StatusBadRequest, "country is required")
		return
	}

	count := req.Count
	if count <= 0 {
		count = 1
	}
	if count > 100 {
		count = 100
	}

	// 1. 查询该用户在该国家已经使用过的物理 IP
	var existingIPs []string
	_ = db.DB.NewSelect().
		TableExpr("endpoint AS ep").
		ColumnExpr("DISTINCT n.server_address").
		Join("JOIN node AS n ON ep.node_id = n.id").
		Where("ep.user_id = ? AND UPPER(COALESCE(n.country, '')) = ? AND ep.is_active = true AND ep.node_id IS NOT NULL", user.ID, country).
		Scan(r.Context(), &existingIPs)

	excludedIPs := make(map[string]bool)
	for _, ip := range existingIPs {
		excludedIPs[ip] = true
	}

	// 查询已有 Socks5 节点数用以生成友好编号
	existingSocks5Count, _ := db.DB.NewSelect().
		Model((*model.Endpoint)(nil)).
		Where("user_id = ? AND UPPER(COALESCE(country, '')) = ?", user.ID, country).
		Count(r.Context())

	var items []Socks5ItemResp

	for i := 0; i < count; i++ {
		seq := existingSocks5Count + i + 1

		// 构建排除 IP 列表
		var excludedList []string
		for ip := range excludedIPs {
			excludedList = append(excludedList, ip)
		}

		type NodeWithLoad struct {
			model.Node
			CurrentLoad int `bun:"current_load"`
		}
		var candidate NodeWithLoad

		q := db.DB.NewSelect().
			Model((*model.Node)(nil)).
			ColumnExpr("node.*").
			ColumnExpr("(SELECT COUNT(1) FROM endpoint ep WHERE ep.node_id = node.id AND ep.is_active = true) AS current_load").
			Where("node.protocol = 'socks5' AND UPPER(COALESCE(node.country, '')) = ? AND node.is_active = true", country)

		if len(excludedList) > 0 {
			q = q.Where("node.server_address NOT IN (?)", bun.In(excludedList))
		}

		err := q.OrderExpr("current_load ASC, node.id ASC").Limit(1).Scan(r.Context(), &candidate)

		username := fmt.Sprintf("u%d_%s_%02d", user.ID, strings.ToLower(country), seq)
		pwd := fmt.Sprintf("pwd_%s", randomHex(8))

		tag := fmt.Sprintf("%s-Shop%02d", country, seq)
		if req.Remark != nil && strings.TrimSpace(*req.Remark) != "" {
			tag = fmt.Sprintf("%s-%02d", strings.TrimSpace(*req.Remark), seq)
		}

		if err == nil && candidate.ID > 0 {
			// 成功找到最优候选母鸡
			excludedIPs[candidate.ServerAddress] = true

			ep := &model.Endpoint{
				UserID:    user.ID,
				NodeID:    &candidate.ID,
				Tag:       tag,
				UUID:      uuid.New().String(),
				Username:  &username,
				Password:  &pwd,
				Status:    "active",
				Country:   &country,
				IsActive:  true,
				SortOrder: int(seq),
				Remark:    req.Remark,
			}
			_, _ = db.DB.NewInsert().Model(ep).Exec(r.Context())

			proxyFormat := fmt.Sprintf("%s:%d:%s:%s", candidate.ServerAddress, candidate.ServerPort, username, pwd)
			proxyURL := fmt.Sprintf("socks5://%s:%s@%s:%d#%s", username, pwd, candidate.ServerAddress, candidate.ServerPort, tag)

			items = append(items, Socks5ItemResp{
				EndpointID:  ep.ID,
				Status:      "active",
				Country:     country,
				IP:          candidate.ServerAddress,
				Port:        candidate.ServerPort,
				Username:    username,
				Password:    pwd,
				ProxyFormat: proxyFormat,
				ProxyURL:    proxyURL,
			})
		} else {
			// 触发库存耗尽保护：创建凭证存根，等待运维开机绑定
			ep := &model.Endpoint{
				UserID:    user.ID,
				NodeID:    nil,
				Tag:       tag,
				UUID:      uuid.New().String(),
				Username:  &username,
				Password:  &pwd,
				Status:    "pending_pool_exhausted",
				Country:   &country,
				IsActive:  true,
				SortOrder: int(seq),
				Remark:    req.Remark,
			}
			_, _ = db.DB.NewInsert().Model(ep).Exec(r.Context())

			items = append(items, Socks5ItemResp{
				EndpointID:  ep.ID,
				Status:      "pending_pool_exhausted",
				Country:     country,
				IP:          "",
				Port:        0,
				Username:    username,
				Password:    pwd,
				ProxyFormat: "",
				ProxyURL:    "",
			})
		}
	}

	RespondServiceSuccess(w, map[string]interface{}{
		"allocated_count": len(items),
		"items":           items,
	})
}

// socks5GrantBlocked reports whether a canonical grant for (user, node)
// exists and is not currently usable (expired/revoked/disabled). A missing
// grant means the row is legacy/unprojected and is not filtered here.
func socks5GrantBlocked(ctx interface{ Deadline() (time.Time, bool); Done() <-chan struct{}; Err() error; Value(any) any }, userID, nodeID int64, now time.Time) bool {
	var grants []*model.AccessGrant
	if err := db.DB.NewSelect().Model(&grants).
		Relation("Credential").
		Where("access_grant.user_id = ? AND access_grant.endpoint_id = ?", userID, nodeID).
		Scan(ctx); err != nil || len(grants) == 0 {
		return false
	}
	for _, g := range grants {
		if g == nil || g.Credential == nil {
			continue
		}
		if g.IsUsable(now) && g.Credential.IsUsable(now) {
			return false
		}
	}
	return true
}

// GetUserSocks5List 获取用户 Socks5 节点列表 (纯净直出)
func GetUserSocks5List(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		RespondServiceError(w, http.StatusBadRequest, "token is required")
		return
	}

	var user model.User
	if err := db.DB.NewSelect().Model(&user).Where("token = ? AND is_active = true", token).Scan(r.Context()); err != nil {
		RespondServiceError(w, http.StatusNotFound, "目标用户不存在或未激活")
		return
	}

	// 鉴权：允许 X-Service-Token 或者直接拥有用户 token
	serviceToken := r.Header.Get("X-Service-Token")
	if serviceToken != "" {
		expected := config.MallServiceSecret
		var setting model.AppSetting
		if err := db.DB.NewSelect().Model(&setting).Where("key = ?", "mall_service_secret").Scan(r.Context()); err == nil && setting.Value != "" {
			expected = setting.Value
		}
		if expected != "" && serviceToken != expected {
			RespondServiceError(w, http.StatusUnauthorized, "Invalid X-Service-Token")
			return
		}
	}

	var endpoints []*model.Endpoint
	err := db.DB.NewSelect().
		Model(&endpoints).
		Relation("Node").
		Where("endpoint.user_id = ? AND endpoint.is_active = true AND endpoint.status = 'active' AND endpoint.node_id IS NOT NULL", user.ID).
		Order("endpoint.sort_order ASC", "endpoint.id ASC").
		Scan(r.Context())
	if err != nil {
		RespondServiceError(w, http.StatusInternalServerError, err.Error())
		return
	}

	now := time.Now()
	format := strings.ToLower(r.URL.Query().Get("format"))
	if format == "text" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		for _, ep := range endpoints {
			if ep.Node != nil && ep.Node.Protocol == "socks5" && ep.Node.IsActive && ep.Username != nil && ep.Password != nil {
				if ep.NodeID != nil && socks5GrantBlocked(r.Context(), user.ID, *ep.NodeID, now) {
					continue
				}
				_, _ = fmt.Fprintf(w, "%s:%d:%s:%s\n", ep.Node.ServerAddress, ep.Node.ServerPort, *ep.Username, *ep.Password)
			}
		}
		return
	}

	var items []Socks5ItemResp
	for _, ep := range endpoints {
		if ep.Node != nil && ep.Node.Protocol == "socks5" && ep.Node.IsActive && ep.Username != nil && ep.Password != nil {
			if ep.NodeID != nil && socks5GrantBlocked(r.Context(), user.ID, *ep.NodeID, now) {
				continue
			}
			proxyFormat := fmt.Sprintf("%s:%d:%s:%s", ep.Node.ServerAddress, ep.Node.ServerPort, *ep.Username, *ep.Password)
			proxyURL := fmt.Sprintf("socks5://%s:%s@%s:%d#%s", *ep.Username, *ep.Password, ep.Node.ServerAddress, ep.Node.ServerPort, ep.Tag)
			ctry := ""
			if ep.Country != nil {
				ctry = *ep.Country
			} else if ep.Node.Country != nil {
				ctry = *ep.Node.Country
			}
			items = append(items, Socks5ItemResp{
				EndpointID:  ep.ID,
				Status:      ep.Status,
				Country:     ctry,
				IP:          ep.Node.ServerAddress,
				Port:        ep.Node.ServerPort,
				Username:    *ep.Username,
				Password:    *ep.Password,
				ProxyFormat: proxyFormat,
				ProxyURL:    proxyURL,
			})
		}
	}

	RespondServiceSuccess(w, map[string]interface{}{
		"total": len(items),
		"items": items,
	})
}

// GetGostNodeAuthList GOST 母鸡端 HTTP 账密热加载接口 (零 Agent + 本地纯内存验证)
func GetGostNodeAuthList(w http.ResponseWriter, r *http.Request) {
	nodeIDStr := chi.URLParam(r, "nodeId")
	nodeID, err := strconv.ParseInt(nodeIDStr, 10, 64)
	if err != nil || nodeID <= 0 {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	var node model.Node
	if err := db.DB.NewSelect().Model(&node).Where("id = ? AND is_active = true AND protocol = 'socks5'", nodeID).Scan(r.Context()); err != nil {
		http.Error(w, "Node not found or not active socks5 node", http.StatusNotFound)
		return
	}

	// 鉴权：若提供 X-Service-Token 则必须有效
	serviceToken := r.Header.Get("X-Service-Token")
	if serviceToken != "" {
		expected := config.MallServiceSecret
		var setting model.AppSetting
		if err := db.DB.NewSelect().Model(&setting).Where("key = ?", "mall_service_secret").Scan(r.Context()); err == nil && setting.Value != "" {
			expected = setting.Value
		}
		if expected != "" && serviceToken != expected {
			http.Error(w, "Invalid X-Service-Token", http.StatusUnauthorized)
			return
		}
	}

	type authRow struct {
		Username string  `bun:"username"`
		Password *string `bun:"password"`
	}
	var rows []authRow
	// Canonical path (D10): only usable grant+credential pairs, same semantics as /sub.
	now := time.Now()
	if err := db.DB.NewSelect().
		TableExpr("access_grants AS g").
		Join("JOIN credentials AS c ON c.id = g.credential_id").
		Join("JOIN \"user\" AS u ON u.id = g.user_id").
		ColumnExpr("c.username AS username").
		ColumnExpr("c.password_ciphertext AS password").
		Where("g.endpoint_id = ? AND g.status = ? AND (g.expires_at IS NULL OR g.expires_at > ?)", nodeID, model.AccessGrantStatusActive, now).
		Where("c.status = ? AND c.revoked_at IS NULL AND (c.expires_at IS NULL OR c.expires_at > ?)", model.CredentialStatusActive, now).
		Where("c.username IS NOT NULL AND c.username <> ''").
		Where("u.is_active = true").
		Scan(r.Context(), &rows); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Legacy fallback: include unprojected legacy endpoint auth only when no
	// grant exists at all for (user,node). If any grant exists, its expiry/
	// status is authoritative and must not be bypassed.
	var legacyRows []authRow
	if err := db.DB.NewSelect().
		TableExpr("endpoint AS ep").
		Join("JOIN \"user\" AS u ON u.id = ep.user_id").
		ColumnExpr("ep.username AS username").
		ColumnExpr("ep.password AS password").
		Where("ep.node_id = ? AND ep.is_active = true AND ep.status = 'active'", nodeID).
		Where("ep.username IS NOT NULL AND ep.username <> '' AND ep.password IS NOT NULL AND ep.password <> ''").
		Where("u.is_active = true").
		Where("NOT EXISTS (SELECT 1 FROM access_grants ag WHERE ag.user_id = ep.user_id AND ag.endpoint_id = ep.node_id)").
		Scan(r.Context(), &legacyRows); err == nil {
		rows = append(rows, legacyRows...)
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintln(w, "# username password")
	seen := map[string]bool{}
	for _, row := range rows {
		if row.Password == nil || strings.TrimSpace(*row.Password) == "" {
			continue
		}
		key := row.Username + "\x00" + *row.Password
		if seen[key] {
			continue
		}
		seen[key] = true
		_, _ = fmt.Fprintf(w, "%s %s\n", row.Username, *row.Password)
	}
}

// RegisterServiceRoutes 注册全部微服务接口路由
func RegisterServiceRoutes(r chi.Router) {
	r.Route("/api/service", func(r chi.Router) {
		// 1. GOST 母鸡端热同步账密名单
		r.Get("/socks5/nodes/{nodeId}/auth-list", GetGostNodeAuthList)

		// 2. 获取用户 Socks5 列表 (支持直连导入文本或商城查询)
		r.Get("/users/{token}/socks5/list", GetUserSocks5List)

		// 3. 商城专用操作 API (严格鉴权 X-Service-Token)
		r.Group(func(r chi.Router) {
			r.Use(ServiceAuth)

			r.Post("/users/register", RegisterServiceUser)
			r.Post("/users/{token}/credentials", AddUserServiceCredential)
			r.Post("/render-config", RenderConfigStateless)
			r.Post("/users/{token}/socks5/allocate", AllocateUserSocks5)
		})
	})
}
