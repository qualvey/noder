package handler

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"noder/internal/contract"
	"noder/internal/db"
	"noder/internal/model"
	"noder/internal/service/mihomo"
	"noder/internal/service/singbox"
)

func RegisterSubscriptionRoutes(r chi.Router) {
	r.Get("/sub", GetSingboxConfig)
	r.Get("/node", GetUserNodes)
	r.Get("/mihomo", GetMihomoConfig)
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
		Relation("Nodes").
		Where("user.token = ?", token).
		Scan(ctx)
	if err != nil || !user.IsActive {
		return nil, &contract.APIError{StatusCode: http.StatusUnauthorized, Detail: "Invalid or inactive user token"}
	}
	return &user, nil
}

func getUserOrderedActiveNodes(user *model.User) []*model.Node {
	orderedIDs := user.GetSortedNodeIDs()
	activeMap := make(map[int64]*model.Node)
	for _, n := range user.Nodes {
		if n.IsActive {
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

func GetSingboxConfig(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	user, err := getActiveUser(r.Context(), token)
	if err != nil {
		HandleAPIError(w, err)
		return
	}

	activeNodes := getUserOrderedActiveNodes(user)
	if len(activeNodes) == 0 {
		RespondError(w, http.StatusNotFound, "No active nodes associated with this user")
		return
	}

	cfg, err := singbox.GenerateSingboxConfig(activeNodes, user)
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

	activeNodes := getUserOrderedActiveNodes(user)
	if len(activeNodes) == 0 {
		RespondError(w, http.StatusNotFound, "No active nodes associated with this user")
		return
	}

	var items []map[string]interface{}
	for _, n := range activeNodes {
		ob, err := singbox.BuildSingboxOutbound(n, user)
		if err != nil {
			HandleAPIError(w, err)
			return
		}
		items = append(items, map[string]interface{}{
			"id":             n.ID,
			"node_name":      n.NodeName,
			"protocol":       n.Protocol,
			"server_address": n.ServerAddress,
			"server_port":    n.ServerPort,
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

	activeNodes := getUserOrderedActiveNodes(user)
	if len(activeNodes) == 0 {
		RespondError(w, http.StatusNotFound, "No active nodes associated with this user")
		return
	}

	yamlBytes, err := mihomo.BuildMihomoConfigYAML(activeNodes, user)
	if err != nil {
		HandleAPIError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/yaml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(yamlBytes)
}

func VerifyUserToken(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	user, err := getActiveUser(r.Context(), token)
	if err != nil {
		HandleAPIError(w, err)
		return
	}

	activeNodes := getUserOrderedActiveNodes(user)
	if len(activeNodes) == 0 {
		RespondError(w, http.StatusNotFound, "用户绑定的节点不存在或已被禁用")
		return
	}

	cfg, err := singbox.GenerateSingboxConfig(activeNodes, user)
	if err != nil {
		HandleAPIError(w, err)
		return
	}

	var nodeIDs []int64
	var nodesSummary []map[string]interface{}
	for _, n := range activeNodes {
		nodeIDs = append(nodeIDs, n.ID)
		nodesSummary = append(nodesSummary, map[string]interface{}{
			"id":        n.ID,
			"node_name": n.NodeName,
			"protocol":  n.Protocol,
		})
	}

	res := map[string]interface{}{
		"valid":          true,
		"user_name":      user.Name,
		"token":          user.Token,
		"node_ids":       nodeIDs,
		"nodes":          nodesSummary,
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

	activeNodes := getUserOrderedActiveNodes(user)
	if len(activeNodes) == 0 {
		RespondError(w, http.StatusNotFound, "No active nodes associated with this user")
		return
	}

	var outbounds []map[string]interface{}
	for _, n := range activeNodes {
		ob, err := singbox.BuildSingboxOutbound(n, user)
		if err != nil {
			HandleAPIError(w, err)
			return
		}
		outbounds = append(outbounds, ob)
	}

	RespondJSON(w, http.StatusOK, outbounds)
}
