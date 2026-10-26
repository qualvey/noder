package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
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

func RegisterUserRoutes(r chi.Router) {
	r.Route("/api/users", func(r chi.Router) {
		r.Use(AdminAuth)
		r.Post("/", CreateUser)
		r.Get("/", ListUsers)
		r.Get("/{id}/downloads", ListUserDownloads)
		r.Get("/{id}/endpoints", ListUserEndpoints)
		r.Post("/{id}/endpoints", AddUserEndpoint)
		r.Put("/{id}/endpoints/{epId}", UpdateUserEndpoint)
		r.Delete("/{id}/endpoints/{epId}", DeleteUserEndpoint)
		r.Get("/{id}", GetUser)
		r.Put("/{id}", UpdateUser)
		r.Delete("/{id}", DeleteUser)
	})
}

func parseAndValidateConfigOverride(raw string) (*string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &data); err != nil {
		return nil, fmt.Errorf("config_override 必须是合法 JSON: %v", err)
	}

	var invalidKeys []string
	for k := range data {
		if !config.AllowedOverrideKeys[k] {
			invalidKeys = append(invalidKeys, k)
		}
	}
	if len(invalidKeys) > 0 {
		var allowed []string
		for k := range config.AllowedOverrideKeys {
			allowed = append(allowed, k)
		}
		sort.Strings(allowed)
		return nil, fmt.Errorf("config_override 不允许覆盖字段 '%s'。允许的字段: %s", strings.Join(invalidKeys, ", "), strings.Join(allowed, ", "))
	}

	normalized, _ := json.Marshal(data)
	normStr := string(normalized)
	return &normStr, nil
}

type EndpointInput struct {
	ID        *int64  `json:"id,omitempty"`
	NodeID    *int64  `json:"node_id,omitempty"`
	Tag       string  `json:"tag"`
	UUID      *string `json:"uuid,omitempty"`
	Username  *string `json:"username,omitempty"`
	Password  *string `json:"password,omitempty"`
	Status    *string `json:"status,omitempty"`
	Country   *string `json:"country,omitempty"`
	IsActive  *bool   `json:"is_active,omitempty"`
	SortOrder *int    `json:"sort_order,omitempty"`
	Remark    *string `json:"remark,omitempty"`
}

type EndpointReadResp struct {
	ID        int64       `json:"id"`
	UserID    int64       `json:"user_id"`
	NodeID    *int64      `json:"node_id"`
	Tag       string      `json:"tag"`
	UUID      string      `json:"uuid"`
	Username  *string     `json:"username,omitempty"`
	Password  *string     `json:"password,omitempty"`
	Status    string      `json:"status"`
	Country   *string     `json:"country,omitempty"`
	IsActive  bool        `json:"is_active"`
	SortOrder int         `json:"sort_order"`
	Remark    *string     `json:"remark,omitempty"`
	Node      *model.Node `json:"node,omitempty"`
}

func resolveEndpointUUID(rawUUID *string, defaultUUID string) string {
	if rawUUID != nil && strings.TrimSpace(*rawUUID) != "" {
		return strings.TrimSpace(*rawUUID)
	}
	if trimmed := strings.TrimSpace(defaultUUID); trimmed != "" {
		return trimmed
	}
	return uuid.New().String()
}

func toEndpointRead(ep *model.Endpoint) EndpointReadResp {
	status := ep.Status
	if status == "" {
		status = "active"
	}
	return EndpointReadResp{
		ID:        ep.ID,
		UserID:    ep.UserID,
		NodeID:    ep.NodeID,
		Tag:       ep.Tag,
		UUID:      ep.UUID,
		Username:  ep.Username,
		Password:  ep.Password,
		Status:    status,
		Country:   ep.Country,
		IsActive:  ep.IsActive,
		SortOrder: ep.SortOrder,
		Remark:    ep.Remark,
		Node:      ep.Node,
	}
}

type UserCreateReq struct {
	Name               string          `json:"name"`
	ExternalID         *string         `json:"external_id"`
	IsActive           *bool           `json:"is_active"`
	Token              *string         `json:"token"`
	UUID               *string         `json:"uuid"`
	Password           *string         `json:"password"`
	Remark             *string         `json:"remark"`
	ConfigOverride     *string         `json:"config_override"`
	NodeIDs            []int64         `json:"node_ids"`
	Endpoints          []EndpointInput `json:"endpoints"`
	TemplateBranchID   *int64          `json:"template_branch_id"`
	IndependentNodeIDs []int64         `json:"independent_node_ids"`
}

type UserReadResp struct {
	ID                 int64              `json:"id"`
	ExternalID         *string            `json:"external_id,omitempty"`
	Name               string             `json:"name"`
	IsActive           bool               `json:"is_active"`
	Token              string             `json:"token"`
	UUID               *string            `json:"uuid"`
	Password           *string            `json:"password"`
	Remark             *string            `json:"remark"`
	ConfigOverride     *string            `json:"config_override"`
	NodeOrder          *string            `json:"node_order"`
	NodeIDs            []int64            `json:"node_ids"`
	Endpoints          []EndpointReadResp `json:"endpoints"`
	TemplateBranchID   *int64             `json:"template_branch_id,omitempty"`
	DownloadCount      int                `json:"download_count"`
	LastDownloadAt     *time.Time         `json:"last_download_at,omitempty"`
	IndependentNodeIDs []int64            `json:"independent_node_ids"`
}

type UserDownloadReadResp struct {
	ID           int64     `json:"id"`
	FileID       int64     `json:"file_id"`
	FileName     string    `json:"file_name"`
	DownloadedAt time.Time `json:"downloaded_at"`
}

type userDownloadStats struct {
	Count int
	Last  *time.Time
}

func loadUserDownloadStats(ctx context.Context, userIDs []int64) map[int64]userDownloadStats {
	stats := make(map[int64]userDownloadStats, len(userIDs))
	if len(userIDs) == 0 {
		return stats
	}
	type downloadStatRow struct {
		UserID int64     `bun:"user_id"`
		Count  int       `bun:"count"`
		Last   time.Time `bun:"last"`
	}
	var rows []downloadStatRow
	if err := db.DB.NewSelect().TableExpr("user_download_log").
		ColumnExpr("user_id").
		ColumnExpr("COUNT(*) AS count").
		ColumnExpr("MAX(downloaded_at) AS last").
		Where("user_id IN (?)", bun.In(userIDs)).
		Group("user_id").Scan(ctx, &rows); err != nil {
		return stats
	}
	for _, row := range rows {
		last := row.Last
		stats[row.UserID] = userDownloadStats{Count: row.Count, Last: &last}
	}
	return stats
}

func attachDownloadStats(ctx context.Context, users []*model.User, responses []UserReadResp) {
	ids := make([]int64, 0, len(users))
	for _, user := range users {
		ids = append(ids, user.ID)
	}
	stats := loadUserDownloadStats(ctx, ids)
	for i, user := range users {
		stat := stats[user.ID]
		responses[i].DownloadCount = stat.Count
		responses[i].LastDownloadAt = stat.Last
	}
}

func toUserRead(u *model.User) UserReadResp {
	sortedIDs := u.GetSortedNodeIDs()
	if sortedIDs == nil {
		sortedIDs = []int64{}
	}
	var epResps []EndpointReadResp
	for _, ep := range u.Endpoints {
		epResps = append(epResps, toEndpointRead(ep))
	}
	if epResps == nil {
		epResps = []EndpointReadResp{}
	}
	return UserReadResp{
		ID:                 u.ID,
		ExternalID:         u.ExternalID,
		Name:               u.Name,
		IsActive:           u.IsActive,
		Token:              u.Token,
		UUID:               u.UUID,
		Password:           u.Password,
		Remark:             u.Remark,
		ConfigOverride:     u.ConfigOverride,
		NodeOrder:          u.NodeOrder,
		NodeIDs:            sortedIDs,
		Endpoints:          epResps,
		TemplateBranchID:   u.TemplateBranchID,
		IndependentNodeIDs: parseIDList(u.IndependentNodeIDs),
	}
}

func parseIDList(raw *string) []int64 {
	if raw == nil {
		return []int64{}
	}
	var ids []int64
	if json.Unmarshal([]byte(*raw), &ids) != nil || ids == nil {
		return []int64{}
	}
	return ids
}

func randomHex(bytesLen int) string {
	b := make([]byte, bytesLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func CreateUser(w http.ResponseWriter, r *http.Request) {
	var req UserCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		RespondError(w, http.StatusBadRequest, "User name is required")
		return
	}

	token := uuid.New().String()
	if req.Token != nil && strings.TrimSpace(*req.Token) != "" {
		token = strings.TrimSpace(*req.Token)
	}

	// 检查 token 唯一性
	exists, _ := db.DB.NewSelect().Model((*model.User)(nil)).Where("token = ?", token).Exists(r.Context())
	if exists {
		RespondError(w, http.StatusBadRequest, "Token already exists")
		return
	}

	if req.ExternalID != nil && strings.TrimSpace(*req.ExternalID) != "" {
		extID := strings.TrimSpace(*req.ExternalID)
		req.ExternalID = &extID
		exists, _ := db.DB.NewSelect().Model((*model.User)(nil)).Where("external_id = ?", extID).Exists(r.Context())
		if exists {
			RespondError(w, http.StatusBadRequest, fmt.Sprintf("external_id '%s' already exists", extID))
			return
		}
	}

	userUUID := uuid.New().String()
	if req.UUID != nil && strings.TrimSpace(*req.UUID) != "" {
		userUUID = strings.TrimSpace(*req.UUID)
	}

	userPassword := randomHex(8)
	if req.Password != nil && strings.TrimSpace(*req.Password) != "" {
		userPassword = strings.TrimSpace(*req.Password)
	}

	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}

	var configOverride *string
	if req.ConfigOverride != nil {
		norm, err := parseAndValidateConfigOverride(*req.ConfigOverride)
		if err != nil {
			RespondError(w, http.StatusBadRequest, err.Error())
			return
		}
		configOverride = norm
	}

	var nodeOrder *string
	if req.NodeIDs != nil && len(req.NodeIDs) > 0 {
		b, _ := json.Marshal(req.NodeIDs)
		s := string(b)
		nodeOrder = &s
	}
	var independentNodeIDs *string
	if req.IndependentNodeIDs != nil {
		b, _ := json.Marshal(req.IndependentNodeIDs)
		s := string(b)
		independentNodeIDs = &s
	}

	user := &model.User{
		Name:               req.Name,
		ExternalID:         req.ExternalID,
		Token:              token,
		IsActive:           isActive,
		UUID:               &userUUID,
		Password:           &userPassword,
		Remark:             req.Remark,
		ConfigOverride:     configOverride,
		NodeOrder:          nodeOrder,
		TemplateBranchID:   req.TemplateBranchID,
		IndependentNodeIDs: independentNodeIDs,
	}

	if _, err := db.DB.NewInsert().Model(user).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 绑定节点与端点
	if req.Endpoints != nil && len(req.Endpoints) > 0 {
		var endpoints []*model.Endpoint
		var nodeIDs []int64
		nodeSeen := make(map[int64]bool)
		for idx, epIn := range req.Endpoints {
			var epNodeID *int64
			if epIn.NodeID != nil && *epIn.NodeID > 0 {
				epNodeID = epIn.NodeID
			}
			tag := strings.TrimSpace(epIn.Tag)
			if tag == "" && epNodeID != nil {
				var n model.Node
				if err := db.DB.NewSelect().Model(&n).Where("id = ?", *epNodeID).Scan(r.Context()); err == nil {
					tag = n.Tag
					if tag == "" {
						tag = n.NodeName
					}
				}
			}
			epUUID := resolveEndpointUUID(epIn.UUID, userUUID)
			epActive := true
			if epIn.IsActive != nil {
				epActive = *epIn.IsActive
			}
			sortOrder := idx
			if epIn.SortOrder != nil {
				sortOrder = *epIn.SortOrder
			}
			epStatus := "active"
			if epIn.Status != nil && *epIn.Status != "" {
				epStatus = *epIn.Status
			} else if epNodeID == nil {
				epStatus = "pending_binding"
			}
			endpoints = append(endpoints, &model.Endpoint{
				UserID:    user.ID,
				NodeID:    epNodeID,
				Tag:       tag,
				UUID:      epUUID,
				Username:  epIn.Username,
				Password:  epIn.Password,
				Status:    epStatus,
				Country:   epIn.Country,
				IsActive:  epActive,
				SortOrder: sortOrder,
				Remark:    epIn.Remark,
			})
			if epNodeID != nil && !nodeSeen[*epNodeID] {
				nodeIDs = append(nodeIDs, *epNodeID)
				nodeSeen[*epNodeID] = true
			}
		}
		if len(endpoints) > 0 {
			_, _ = db.DB.NewInsert().Model(&endpoints).Exec(r.Context())
		}
		if len(nodeIDs) > 0 {
			var links []*model.UserNodeLink
			for _, nid := range nodeIDs {
				links = append(links, &model.UserNodeLink{UserID: user.ID, NodeID: nid})
			}
			_, _ = db.DB.NewInsert().Model(&links).Exec(r.Context())
			b, _ := json.Marshal(nodeIDs)
			s := string(b)
			user.NodeOrder = &s
			_, _ = db.DB.NewUpdate().Model(user).Column("node_order").Where("id = ?", user.ID).Exec(r.Context())
		}
	} else if req.NodeIDs != nil && len(req.NodeIDs) > 0 {
		var links []*model.UserNodeLink
		var endpoints []*model.Endpoint
		for idx, nid := range req.NodeIDs {
			links = append(links, &model.UserNodeLink{UserID: user.ID, NodeID: nid})
			var n model.Node
			tag := fmt.Sprintf("Node-%d", nid)
			if err := db.DB.NewSelect().Model(&n).Where("id = ?", nid).Scan(r.Context()); err == nil {
				tag = n.Tag
				if tag == "" {
					tag = n.NodeName
				}
			}
			nidCopy := nid
			endpoints = append(endpoints, &model.Endpoint{
				UserID:    user.ID,
				NodeID:    &nidCopy,
				Tag:       tag,
				UUID:      userUUID,
				Password:  &userPassword,
				Status:    "active",
				IsActive:  true,
				SortOrder: idx,
			})
		}
		_, _ = db.DB.NewInsert().Model(&links).Exec(r.Context())
		_, _ = db.DB.NewInsert().Model(&endpoints).Exec(r.Context())
	}

	// 重新加载节点与端点用于组装返回值
	_ = db.DB.NewSelect().Model(user).
		Relation("Nodes").
		Relation("Endpoints", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Order("endpoint.sort_order ASC", "endpoint.id ASC")
		}).
		Relation("Endpoints.Node").
		Where("id = ?", user.ID).
		Scan(r.Context())

	RespondJSON(w, http.StatusOK, toUserRead(user))
}

func ListUsers(w http.ResponseWriter, r *http.Request) {
	var users []*model.User
	err := db.DB.NewSelect().Model(&users).
		Relation("Nodes").
		Relation("Endpoints", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Order("endpoint.sort_order ASC", "endpoint.id ASC")
		}).
		Relation("Endpoints.Node").
		Scan(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var res []UserReadResp
	for _, u := range users {
		res = append(res, toUserRead(u))
	}
	if res == nil {
		res = []UserReadResp{}
	}
	attachDownloadStats(r.Context(), users, res)
	RespondJSON(w, http.StatusOK, res)
}

func GetUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var user model.User
	err = db.DB.NewSelect().Model(&user).
		Relation("Nodes").
		Relation("Endpoints", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Order("endpoint.sort_order ASC", "endpoint.id ASC")
		}).
		Relation("Endpoints.Node").
		Where("user.id = ?", id).
		Scan(r.Context())
	if err != nil {
		RespondError(w, http.StatusNotFound, "User not found")
		return
	}

	response := []UserReadResp{toUserRead(&user)}
	attachDownloadStats(r.Context(), []*model.User{&user}, response)
	RespondJSON(w, http.StatusOK, response[0])
}

func ListUserDownloads(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	exists, err := db.DB.NewSelect().Model((*model.User)(nil)).Where("id = ?", id).Exists(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !exists {
		RespondError(w, http.StatusNotFound, "User not found")
		return
	}

	var logs []model.UserDownloadLog
	err = db.DB.NewSelect().Model(&logs).Where("user_id = ?", id).
		OrderExpr("downloaded_at DESC, id DESC").Limit(100).Scan(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	responses := make([]UserDownloadReadResp, 0, len(logs))
	for _, entry := range logs {
		responses = append(responses, UserDownloadReadResp{
			ID:           entry.ID,
			FileID:       entry.FileID,
			FileName:     entry.FileName,
			DownloadedAt: entry.DownloadedAt,
		})
	}
	RespondJSON(w, http.StatusOK, responses)
}

func UpdateUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var user model.User
	if err := db.DB.NewSelect().Model(&user).Relation("Nodes").Where("user.id = ?", id).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "User not found")
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if tok, ok := body["token"].(string); ok && tok != "" && tok != user.Token {
		exists, _ := db.DB.NewSelect().Model((*model.User)(nil)).Where("token = ? AND id != ?", tok, id).Exists(r.Context())
		if exists {
			RespondError(w, http.StatusBadRequest, "Token already exists")
			return
		}
		user.Token = tok
	}

	if name, ok := body["name"].(string); ok {
		user.Name = name
	}
	if active, ok := body["is_active"].(bool); ok {
		user.IsActive = active
	}
	if u, ok := body["uuid"].(string); ok {
		user.UUID = &u
	}
	if p, ok := body["password"].(string); ok {
		user.Password = &p
	}
	if r, ok := body["remark"].(string); ok {
		user.Remark = &r
	}
	if raw, exists := body["independent_node_ids"]; exists {
		var ids []int64
		if list, ok := raw.([]interface{}); ok {
			for _, item := range list {
				if f, ok := item.(float64); ok {
					ids = append(ids, int64(f))
				}
			}
		}
		if len(ids) == 0 {
			user.IndependentNodeIDs = nil
		} else {
			b, _ := json.Marshal(ids)
			s := string(b)
			user.IndependentNodeIDs = &s
		}
	}

	if rawExtID, exists := body["external_id"]; exists {
		if rawExtID == nil {
			user.ExternalID = nil
		} else if extStr, ok := rawExtID.(string); ok {
			trimmed := strings.TrimSpace(extStr)
			if trimmed == "" {
				user.ExternalID = nil
			} else {
				exists, _ := db.DB.NewSelect().Model((*model.User)(nil)).Where("external_id = ? AND id != ?", trimmed, id).Exists(r.Context())
				if exists {
					RespondError(w, http.StatusBadRequest, fmt.Sprintf("external_id '%s' already exists", trimmed))
					return
				}
				user.ExternalID = &trimmed
			}
		}
	}

	if rawOverride, exists := body["config_override"]; exists {
		if rawOverride == nil {
			user.ConfigOverride = nil
		} else if str, ok := rawOverride.(string); ok {
			norm, err := parseAndValidateConfigOverride(str)
			if err != nil {
				RespondError(w, http.StatusBadRequest, err.Error())
				return
			}
			user.ConfigOverride = norm
		}
	}

	// 模板分支更新
	if rawBranchID, exists := body["template_branch_id"]; exists {
		if rawBranchID == nil {
			user.TemplateBranchID = nil
		} else if f, ok := rawBranchID.(float64); ok {
			id := int64(f)
			if id > 0 {
				// 验证分支存在
				exists, _ := db.DB.NewSelect().Model((*model.TemplateBranch)(nil)).Where("id = ?", id).Exists(r.Context())
				if !exists {
					RespondError(w, http.StatusBadRequest, "Template branch not found")
					return
				}
				user.TemplateBranchID = &id
			} else {
				user.TemplateBranchID = nil
			}
		}
	}

	// 端点与节点更新
	if rawEndpoints, exists := body["endpoints"]; exists {
		_, _ = db.DB.NewDelete().Model((*model.Endpoint)(nil)).Where("user_id = ?", id).Exec(r.Context())
		_, _ = db.DB.NewDelete().Model((*model.UserNodeLink)(nil)).Where("user_id = ?", id).Exec(r.Context())
		user.NodeOrder = nil

		if epList, ok := rawEndpoints.([]interface{}); ok && len(epList) > 0 {
			var endpoints []*model.Endpoint
			var nodeIDs []int64
			nodeSeen := make(map[int64]bool)

			for idx, item := range epList {
				m, ok := item.(map[string]interface{})
				if !ok {
					continue
				}
				var nodeID int64
				if f, ok := m["node_id"].(float64); ok {
					nodeID = int64(f)
				}
				if nodeID <= 0 {
					continue
				}

				tag, _ := m["tag"].(string)
				tag = strings.TrimSpace(tag)
				if tag == "" {
					var n model.Node
					if err := db.DB.NewSelect().Model(&n).Where("id = ?", nodeID).Scan(r.Context()); err == nil {
						tag = n.Tag
						if tag == "" {
							tag = n.NodeName
						}
					}
				}

				var rawUUID *string
				if uStr, ok := m["uuid"].(string); ok && strings.TrimSpace(uStr) != "" {
					trimmed := strings.TrimSpace(uStr)
					rawUUID = &trimmed
				}
				uuidDefault := user.Token
				if user.UUID != nil && strings.TrimSpace(*user.UUID) != "" {
					uuidDefault = *user.UUID
				}
				epUUID := resolveEndpointUUID(rawUUID, uuidDefault)

				var epPwd *string
				if pStr, ok := m["password"].(string); ok && strings.TrimSpace(pStr) != "" {
					trimmed := strings.TrimSpace(pStr)
					epPwd = &trimmed
				}

				isActive := true
				if act, ok := m["is_active"].(bool); ok {
					isActive = act
				}

				sortOrder := idx
				if so, ok := m["sort_order"].(float64); ok {
					sortOrder = int(so)
				}

				var epRemark *string
				if rm, ok := m["remark"].(string); ok && strings.TrimSpace(rm) != "" {
					trimmed := strings.TrimSpace(rm)
					epRemark = &trimmed
				}

				var epNodeID *int64
				if nodeID > 0 {
					epNodeID = &nodeID
				}
				endpoints = append(endpoints, &model.Endpoint{
					UserID:    id,
					NodeID:    epNodeID,
					Tag:       tag,
					UUID:      epUUID,
					Password:  epPwd,
					Status:    "active",
					IsActive:  isActive,
					SortOrder: sortOrder,
					Remark:    epRemark,
				})

				if !nodeSeen[nodeID] {
					nodeIDs = append(nodeIDs, nodeID)
					nodeSeen[nodeID] = true
				}
			}

			if len(endpoints) > 0 {
				_, _ = db.DB.NewInsert().Model(&endpoints).Exec(r.Context())
			}
			if len(nodeIDs) > 0 {
				var links []*model.UserNodeLink
				for _, nid := range nodeIDs {
					links = append(links, &model.UserNodeLink{UserID: id, NodeID: nid})
				}
				_, _ = db.DB.NewInsert().Model(&links).Exec(r.Context())
				b, _ := json.Marshal(nodeIDs)
				s := string(b)
				user.NodeOrder = &s
			}
		}
	} else if rawNodeIDs, exists := body["node_ids"]; exists {
		if rawNodeIDs == nil {
			_, _ = db.DB.NewDelete().Model((*model.UserNodeLink)(nil)).Where("user_id = ?", id).Exec(r.Context())
			_, _ = db.DB.NewDelete().Model((*model.Endpoint)(nil)).Where("user_id = ?", id).Exec(r.Context())
			user.NodeOrder = nil
		} else if idsList, ok := rawNodeIDs.([]interface{}); ok {
			var newIDs []int64
			for _, item := range idsList {
				if f, ok := item.(float64); ok {
					newIDs = append(newIDs, int64(f))
				}
			}

			// 重建 UserNodeLink 与 Endpoint
			_, _ = db.DB.NewDelete().Model((*model.UserNodeLink)(nil)).Where("user_id = ?", id).Exec(r.Context())
			_, _ = db.DB.NewDelete().Model((*model.Endpoint)(nil)).Where("user_id = ?", id).Exec(r.Context())
			if len(newIDs) > 0 {
				var links []*model.UserNodeLink
				var endpoints []*model.Endpoint
				uUUID := user.Token
				if user.UUID != nil && *user.UUID != "" {
					uUUID = *user.UUID
				}
				for idx, nid := range newIDs {
					links = append(links, &model.UserNodeLink{UserID: id, NodeID: nid})
					var n model.Node
					tag := fmt.Sprintf("Node-%d", nid)
					if err := db.DB.NewSelect().Model(&n).Where("id = ?", nid).Scan(r.Context()); err == nil {
						tag = n.Tag
						if tag == "" {
							tag = n.NodeName
						}
					}
					nidCopy := nid
					endpoints = append(endpoints, &model.Endpoint{
						UserID:    id,
						NodeID:    &nidCopy,
						Tag:       tag,
						UUID:      uUUID,
						Password:  user.Password,
						Status:    "active",
						IsActive:  true,
						SortOrder: idx,
					})
				}
				_, _ = db.DB.NewInsert().Model(&links).Exec(r.Context())
				_, _ = db.DB.NewInsert().Model(&endpoints).Exec(r.Context())
				b, _ := json.Marshal(newIDs)
				s := string(b)
				user.NodeOrder = &s
			} else {
				user.NodeOrder = nil
			}
		}
	}

	if _, err := db.DB.NewUpdate().Model(&user).Where("id = ?", id).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 重新拉取关系
	_ = db.DB.NewSelect().Model(&user).
		Relation("Nodes").
		Relation("Endpoints", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Order("endpoint.sort_order ASC", "endpoint.id ASC")
		}).
		Relation("Endpoints.Node").
		Where("user.id = ?", id).
		Scan(r.Context())
	response := []UserReadResp{toUserRead(&user)}
	attachDownloadStats(r.Context(), []*model.User{&user}, response)
	RespondJSON(w, http.StatusOK, response[0])
}

func DeleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	res, err := db.DB.NewDelete().Model((*model.User)(nil)).Where("id = ?", id).Exec(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		RespondError(w, http.StatusNotFound, "User not found")
		return
	}

	_, _ = db.DB.NewDelete().Model((*model.UserNodeLink)(nil)).Where("user_id = ?", id).Exec(r.Context())
	_, _ = db.DB.NewDelete().Model((*model.Endpoint)(nil)).Where("user_id = ?", id).Exec(r.Context())
	RespondJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("User %d deleted successfully", id),
	})
}

func ListUserEndpoints(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	var endpoints []*model.Endpoint
	err = db.DB.NewSelect().Model(&endpoints).
		Relation("Node").
		Where("user_id = ?", id).
		Order("sort_order ASC", "id ASC").
		Scan(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var res []EndpointReadResp
	for _, ep := range endpoints {
		res = append(res, toEndpointRead(ep))
	}
	if res == nil {
		res = []EndpointReadResp{}
	}
	RespondJSON(w, http.StatusOK, res)
}

func AddUserEndpoint(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	userID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	var user model.User
	if err := db.DB.NewSelect().Model(&user).Where("id = ?", userID).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "User not found")
		return
	}

	var req EndpointInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	var nodeIDPtr *int64
	var defaultTag string
	if req.NodeID != nil && *req.NodeID > 0 {
		var node model.Node
		if err := db.DB.NewSelect().Model(&node).Where("id = ?", *req.NodeID).Scan(r.Context()); err != nil {
			RespondError(w, http.StatusBadRequest, "Node not found")
			return
		}
		nodeIDPtr = req.NodeID
		defaultTag = node.Tag
		if defaultTag == "" {
			defaultTag = node.NodeName
		}
	}

	tag := strings.TrimSpace(req.Tag)
	if tag == "" {
		tag = defaultTag
	}
	if tag == "" {
		RespondError(w, http.StatusBadRequest, "tag is required")
		return
	}

	uuidDefault := user.Token
	if user.UUID != nil && strings.TrimSpace(*user.UUID) != "" {
		uuidDefault = *user.UUID
	}
	epUUID := resolveEndpointUUID(req.UUID, uuidDefault)
	isActive := true
	if req.IsActive != nil {
		isActive = *req.IsActive
	}
	sortOrder := 0
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	} else {
		count, _ := db.DB.NewSelect().Model((*model.Endpoint)(nil)).Where("user_id = ?", userID).Count(r.Context())
		sortOrder = count
	}

	status := "active"
	if req.Status != nil && *req.Status != "" {
		status = *req.Status
	} else if nodeIDPtr == nil {
		status = "pending_binding"
	}

	ep := &model.Endpoint{
		UserID:    userID,
		NodeID:    nodeIDPtr,
		Tag:       tag,
		UUID:      epUUID,
		Username:  req.Username,
		Password:  req.Password,
		Status:    status,
		Country:   req.Country,
		IsActive:  isActive,
		SortOrder: sortOrder,
		Remark:    req.Remark,
	}

	if _, err := db.DB.NewInsert().Model(ep).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = db.DB.NewSelect().Model(ep).Relation("Node").Where("endpoint.id = ?", ep.ID).Scan(r.Context())
	RespondJSON(w, http.StatusOK, toEndpointRead(ep))
}

func UpdateUserEndpoint(w http.ResponseWriter, r *http.Request) {
	epIDStr := chi.URLParam(r, "epId")
	epID, err := strconv.ParseInt(epIDStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid endpoint ID")
		return
	}

	var ep model.Endpoint
	if err := db.DB.NewSelect().Model(&ep).Where("id = ?", epID).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "Endpoint not found")
		return
	}

	var body map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	if nidVal, exists := body["node_id"]; exists {
		if nidVal == nil {
			ep.NodeID = nil
		} else if f, ok := nidVal.(float64); ok {
			nid := int64(f)
			if nid > 0 {
				ep.NodeID = &nid
			} else {
				ep.NodeID = nil
			}
		}
	}
	if status, ok := body["status"].(string); ok && status != "" {
		ep.Status = status
	}
	if uname, ok := body["username"].(string); ok {
		ep.Username = &uname
	}
	if ctry, ok := body["country"].(string); ok {
		ep.Country = &ctry
	}
	if tag, ok := body["tag"].(string); ok && strings.TrimSpace(tag) != "" {
		ep.Tag = strings.TrimSpace(tag)
	}
	if u, ok := body["uuid"].(string); ok && strings.TrimSpace(u) != "" {
		ep.UUID = strings.TrimSpace(u)
	}
	if pwd, ok := body["password"].(string); ok {
		ep.Password = &pwd
	}
	if active, ok := body["is_active"].(bool); ok {
		ep.IsActive = active
	}
	if so, ok := body["sort_order"].(float64); ok {
		ep.SortOrder = int(so)
	}
	if r, ok := body["remark"].(string); ok {
		ep.Remark = &r
	}

	if _, err := db.DB.NewUpdate().Model(&ep).Where("id = ?", epID).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	_ = db.DB.NewSelect().Model(&ep).Relation("Node").Where("endpoint.id = ?", epID).Scan(r.Context())
	RespondJSON(w, http.StatusOK, toEndpointRead(&ep))
}

func DeleteUserEndpoint(w http.ResponseWriter, r *http.Request) {
	epIDStr := chi.URLParam(r, "epId")
	epID, err := strconv.ParseInt(epIDStr, 10, 64)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid endpoint ID")
		return
	}

	res, err := db.DB.NewDelete().Model((*model.Endpoint)(nil)).Where("id = ?", epID).Exec(r.Context())
	if err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		RespondError(w, http.StatusNotFound, "Endpoint not found")
		return
	}

	RespondJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("Endpoint %d deleted successfully", epID),
	})
}
