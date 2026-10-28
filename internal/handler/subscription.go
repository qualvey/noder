package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"
	"noder/internal/contract"
	"noder/internal/db"
	"noder/internal/model"
	"noder/internal/service/mihomo"
	"noder/internal/service/singbox"
	"noder/internal/service/template"
	"noder/internal/service/xray"
)

func RegisterSubscriptionRoutes(r chi.Router) {
	r.Get("/sub", GetSingboxConfig)
	r.Get("/node", GetUserNodes)
	r.Get("/mihomo", GetMihomoConfig)
	r.Get("/xray", GetXrayConfig)
	r.Get("/api/user/verify", VerifyUserToken)
	r.Get("/api/user/nodes", GetNodesForUser)
}

func getActiveUser(ctx context.Context, token string) (*model.User, error) {
	if token == "" {
		return nil, &contract.APIError{StatusCode: http.StatusUnauthorized, Detail: "Invalid or inactive user token"}
	}

	var user model.User
	err := db.DB.NewSelect().
		Model(&user).
		Relation("Endpoints", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Order("endpoint.sort_order ASC", "endpoint.id ASC")
		}).
		Relation("Endpoints.Node").
		Relation("Nodes").
		Where("user.token = ?", token).
		Scan(ctx)
	if err != nil || !user.IsActive {
		return nil, &contract.APIError{StatusCode: http.StatusUnauthorized, Detail: "Invalid or inactive user token"}
	}
	return &user, nil
}

func getUserActiveEndpoints(user *model.User) []*model.Endpoint {
	// Once endpoint records exist, they are the source of truth. Do not fall
	// back to the legacy user-node links when all endpoints are disabled:
	// an explicitly disabled endpoint must not be rendered.
	if len(user.Endpoints) > 0 {
		var res []*model.Endpoint
		for _, ep := range user.Endpoints {
			if ep.IsActive && ep.Node != nil && ep.Node.IsActive && ep.Node.Protocol != "socks5" {
				res = append(res, ep)
			}
		}
		return res
	}

	// 兜底：兼容只有 usernodelink 的存量情况
	nodes := getUserOrderedActiveNodes(user)
	userUUID := user.Token
	if user.UUID != nil && *user.UUID != "" {
		userUUID = *user.UUID
	}
	var pwd *string
	if user.Password != nil && *user.Password != "" {
		pwd = user.Password
	} else if user.Token != "" {
		pwd = &user.Token
	}

	var res []*model.Endpoint
	for i, n := range nodes {
		if n.Protocol == "socks5" {
			continue
		}
		tag := n.Tag
		if tag == "" {
			tag = n.NodeName
		}
		res = append(res, &model.Endpoint{
			Node:      n,
			Tag:       tag,
			UUID:      userUUID,
			Password:  pwd,
			IsActive:  n.IsActive,
			SortOrder: i,
		})
	}
	return res
}

func getUserOrderedActiveNodes(user *model.User) []*model.Node {
	orderedIDs := user.GetSortedNodeIDs()
	activeMap := make(map[int64]*model.Node)
	for _, n := range user.Nodes {
		if n.IsActive && n.Protocol != "socks5" {
			activeMap[n.ID] = n
		}
	}

	var res []*model.Node
	for _, id := range orderedIDs {
		if n, ok := activeMap[id]; ok {
			res = append(res, n)
		}
	}
	return res
}

// getKnownTokens 收集节点中已存在的 token，用于模板渲染时的兜底替换
func getKnownTokens(endpoints []*model.Endpoint) []string {
	var tokens []string
	for _, ep := range endpoints {
		if ep == nil || ep.Node == nil {
			continue
		}
		switch ep.Node.Protocol {
		case "vless":
			if ep.Node.Flow != nil && *ep.Node.Flow != "" {
				tokens = append(tokens, *ep.Node.Flow)
			}
		case "tuic":
			// TUIC 节点没有独立 token，使用用户的 uuid/password
		}
	}
	return tokens
}

func GetSingboxConfig(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	user, err := getActiveUser(r.Context(), token)
	if err != nil {
		HandleAPIError(w, err)
		return
	}

	activeEndpoints := getUserActiveEndpoints(user)
	if len(activeEndpoints) == 0 {
		RespondError(w, http.StatusNotFound, "No active nodes associated with this user")
		return
	}

	// 如果用户有关联的模板分支，使用分支内容渲染
	if user.TemplateBranchID != nil && *user.TemplateBranchID > 0 {
		var branch model.TemplateBranch
		if err := db.DB.NewSelect().Model(&branch).Where("id = ? AND is_active = ?", *user.TemplateBranchID, true).Scan(r.Context()); err == nil && branch.ID > 0 {
			// 使用模板分支渲染
			knownTokens := getKnownTokens(activeEndpoints)
			rendered := template.RenderTemplateTextFromEndpoints(branch.Content, user, activeEndpoints, knownTokens)
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(rendered))
			return
		}
	}

	cfg, err := singbox.GenerateSingboxConfigFromEndpoints(activeEndpoints, user)
	if err != nil {
		HandleAPIError(w, err)
		return
	}

	RespondJSON(w, http.StatusOK, cfg)
}

func GetUserNodes(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	user, err := getActiveUser(r.Context(), token)
	if err != nil {
		HandleAPIError(w, err)
		return
	}

	activeEndpoints := getUserActiveEndpoints(user)
	if len(activeEndpoints) == 0 {
		RespondError(w, http.StatusNotFound, "No active nodes associated with this user")
		return
	}

	var items []map[string]interface{}
	for _, ep := range activeEndpoints {
		ob, err := singbox.BuildSingboxEndpointOutbound(ep)
		if err != nil {
			HandleAPIError(w, err)
			return
		}
		items = append(items, map[string]interface{}{
			"id":             ep.ID,
			"node_id":        ep.NodeID,
			"tag":            ep.Tag,
			"node_name":      ep.Node.NodeName,
			"protocol":       ep.Node.Protocol,
			"server_address": ep.Node.ServerAddress,
			"server_port":    ep.Node.ServerPort,
			"outbound":       ob,
		})
	}

	RespondJSON(w, http.StatusOK, items)
}

func GetMihomoConfig(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	user, err := getActiveUser(r.Context(), token)
	if err != nil {
		HandleAPIError(w, err)
		return
	}

	activeEndpoints := getUserActiveEndpoints(user)
	if len(activeEndpoints) == 0 {
		RespondError(w, http.StatusNotFound, "No active nodes associated with this user")
		return
	}

	// 如果用户有关联的模板分支，使用分支内容渲染
	if user.TemplateBranchID != nil && *user.TemplateBranchID > 0 {
		var branch model.TemplateBranch
		if err := db.DB.NewSelect().Model(&branch).Where("id = ? AND is_active = ?", *user.TemplateBranchID, true).Scan(r.Context()); err == nil && branch.ID > 0 {
			// 使用模板分支渲染
			knownTokens := getKnownTokens(activeEndpoints)
			rendered := template.RenderTemplateTextFromEndpoints(branch.Content, user, activeEndpoints, knownTokens)
			w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(rendered))
			return
		}
	}

	yamlBytes, err := mihomo.BuildMihomoConfigYAMLFromEndpoints(activeEndpoints, user)
	if err != nil {
		HandleAPIError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(yamlBytes)
}

func GetXrayConfig(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	user, err := getActiveUser(r.Context(), token)
	if err != nil {
		HandleAPIError(w, err)
		return
	}

	activeEndpoints := getUserActiveEndpoints(user)
	if len(activeEndpoints) == 0 {
		RespondError(w, http.StatusNotFound, "No active nodes associated with this user")
		return
	}

	cfg, err := xray.GenerateXrayConfigFromEndpoints(activeEndpoints, user)
	if err != nil {
		HandleAPIError(w, err)
		return
	}

	RespondJSON(w, http.StatusOK, cfg)
}

func VerifyUserToken(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	user, err := getActiveUser(r.Context(), token)
	if err != nil {
		HandleAPIError(w, err)
		return
	}

	activeEndpoints := getUserActiveEndpoints(user)
	if len(activeEndpoints) == 0 {
		RespondError(w, http.StatusNotFound, "用户绑定的节点不存在或已被禁用")
		return
	}

	cfg, err := singbox.GenerateSingboxConfigFromEndpoints(activeEndpoints, user)
	if err != nil {
		HandleAPIError(w, err)
		return
	}

	var endpointIDs []int64
	var endpointsSummary []map[string]interface{}
	for _, ep := range activeEndpoints {
		endpointIDs = append(endpointIDs, ep.ID)
		endpointsSummary = append(endpointsSummary, map[string]interface{}{
			"id":        ep.ID,
			"node_id":   ep.NodeID,
			"tag":       ep.Tag,
			"node_name": ep.Node.NodeName,
			"protocol":  ep.Node.Protocol,
		})
	}

	res := map[string]interface{}{
		"valid":          true,
		"user_name":      user.Name,
		"token":          user.Token,
		"endpoint_ids":   endpointIDs,
		"endpoints":      endpointsSummary,
		"singbox_config": cfg,
	}

	RespondJSON(w, http.StatusOK, res)
}

func GetNodesForUser(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	user, err := getActiveUser(r.Context(), token)
	if err != nil {
		HandleAPIError(w, err)
		return
	}

	activeEndpoints := getUserActiveEndpoints(user)
	if len(activeEndpoints) == 0 {
		RespondError(w, http.StatusNotFound, "No active nodes associated with this user")
		return
	}

	var outbounds []map[string]interface{}
	for _, ep := range activeEndpoints {
		ob, err := singbox.BuildSingboxEndpointOutbound(ep)
		if err != nil {
			HandleAPIError(w, err)
			return
		}
		outbounds = append(outbounds, ob)
	}

	RespondJSON(w, http.StatusOK, outbounds)
}
