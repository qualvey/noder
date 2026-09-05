package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"noder/internal/config"
	"noder/internal/db"
	"noder/internal/model"
)

func RegisterUserRoutes(r chi.Router) {
	r.Route("/api/users", func(r chi.Router) {
		r.Use(AdminAuth)
		r.Post("/", CreateUser)
		r.Get("/", ListUsers)
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

type UserCreateReq struct {
	Name           string  `json:"name"`
	IsActive       *bool   `json:"is_active"`
	Token          *string `json:"token"`
	UUID           *string `json:"uuid"`
	Password       *string `json:"password"`
	Remark         *string `json:"remark"`
	ConfigOverride *string `json:"config_override"`
	NodeIDs        []int64 `json:"node_ids"`
}

type UserReadResp struct {
	ID             int64   `json:"id"`
	Name           string  `json:"name"`
	IsActive       bool    `json:"is_active"`
	Token          string  `json:"token"`
	UUID           *string `json:"uuid"`
	Password       *string `json:"password"`
	Remark         *string `json:"remark"`
	ConfigOverride *string `json:"config_override"`
	NodeOrder      *string `json:"node_order"`
	NodeIDs        []int64 `json:"node_ids"`
}

func toUserRead(u *model.User) UserReadResp {
	sortedIDs := u.GetSortedNodeIDs()
	if sortedIDs == nil {
		sortedIDs = []int64{}
	}
	return UserReadResp{
		ID:             u.ID,
		Name:           u.Name,
		IsActive:       u.IsActive,
		Token:          u.Token,
		UUID:           u.UUID,
		Password:       u.Password,
		Remark:         u.Remark,
		ConfigOverride: u.ConfigOverride,
		NodeOrder:      u.NodeOrder,
		NodeIDs:        sortedIDs,
	}
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

	user := &model.User{
		Name:           req.Name,
		Token:          token,
		IsActive:       isActive,
		UUID:           &userUUID,
		Password:       &userPassword,
		Remark:         req.Remark,
		ConfigOverride: configOverride,
		NodeOrder:      nodeOrder,
	}

	if _, err := db.DB.NewInsert().Model(user).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 绑定节点
	if req.NodeIDs != nil && len(req.NodeIDs) > 0 {
		var links []*model.UserNodeLink
		for _, nid := range req.NodeIDs {
			links = append(links, &model.UserNodeLink{UserID: user.ID, NodeID: nid})
		}
		_, _ = db.DB.NewInsert().Model(&links).Exec(r.Context())
	}

	// 重新加载节点用于组装返回值
	_ = db.DB.NewSelect().Model(user).Relation("Nodes").Where("id = ?", user.ID).Scan(r.Context())

	RespondJSON(w, http.StatusOK, toUserRead(user))
}

func ListUsers(w http.ResponseWriter, r *http.Request) {
	var users []*model.User
	err := db.DB.NewSelect().Model(&users).Relation("Nodes").Scan(r.Context())
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
	err = db.DB.NewSelect().Model(&user).Relation("Nodes").Where("user.id = ?", id).Scan(r.Context())
	if err != nil {
		RespondError(w, http.StatusNotFound, "User not found")
		return
	}

	RespondJSON(w, http.StatusOK, toUserRead(&user))
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

	// 节点更新
	if rawNodeIDs, exists := body["node_ids"]; exists {
		if rawNodeIDs == nil {
			_, _ = db.DB.NewDelete().Model((*model.UserNodeLink)(nil)).Where("user_id = ?", id).Exec(r.Context())
			user.NodeOrder = nil
		} else if idsList, ok := rawNodeIDs.([]interface{}); ok {
			var newIDs []int64
			for _, item := range idsList {
				if f, ok := item.(float64); ok {
					newIDs = append(newIDs, int64(f))
				}
			}

			// 重建 UserNodeLink
			_, _ = db.DB.NewDelete().Model((*model.UserNodeLink)(nil)).Where("user_id = ?", id).Exec(r.Context())
			if len(newIDs) > 0 {
				var links []*model.UserNodeLink
				for _, nid := range newIDs {
					links = append(links, &model.UserNodeLink{UserID: id, NodeID: nid})
				}
				_, _ = db.DB.NewInsert().Model(&links).Exec(r.Context())
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
	_ = db.DB.NewSelect().Model(&user).Relation("Nodes").Where("user.id = ?", id).Scan(r.Context())
	RespondJSON(w, http.StatusOK, toUserRead(&user))
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
	RespondJSON(w, http.StatusOK, map[string]string{
		"message": fmt.Sprintf("User %d deleted successfully", id),
	})
}
