package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"noder/internal/contract"
	"noder/internal/db"
	"noder/internal/model"
	"noder/internal/service/singbox"
	"noder/internal/service/xray"
)

func getBaseURL(r *http.Request) string {
	var setting model.AppSetting
	if db.DB != nil {
		if err := db.DB.NewSelect().Model(&setting).Where("key = ?", "sub_base_url").Scan(r.Context()); err == nil && strings.TrimSpace(setting.Value) != "" {
			return strings.TrimRight(strings.TrimSpace(setting.Value), "/")
		}
	}
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := r.Host
	if fHost := r.Header.Get("X-Forwarded-Host"); fHost != "" {
		host = fHost
	}
	return fmt.Sprintf("%s://%s", scheme, host)
}

type ServiceUserRegisterReq struct {
	ExternalID string  `json:"external_id"`
	Name       string  `json:"name"`
	Token      *string `json:"token"`
	Remark     *string `json:"remark"`
}

type ServiceUserRegisterRespData struct {
	UserID           int64             `json:"user_id"`
	Token            string            `json:"token"`
	ExternalID       string            `json:"external_id"`
	SubscriptionURLs map[string]string `json:"subscription_urls"`
}

// RegisterServiceUser 商城用户注册 API (POST /api/service/users/register)
func RegisterServiceUser(w http.ResponseWriter, r *http.Request) {
	var req ServiceUserRegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondServiceError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	extID := strings.TrimSpace(req.ExternalID)
	if extID == "" {
		RespondServiceError(w, http.StatusBadRequest, "注册失败：external_id 不能为空")
		return
	}

	// 检查 external_id 唯一性
	exists, _ := db.DB.NewSelect().Model((*model.User)(nil)).Where("external_id = ?", extID).Exists(r.Context())
	if exists {
		RespondServiceError(w, http.StatusBadRequest, fmt.Sprintf("注册失败：external_id '%s' 已被使用", extID))
		return
	}

	token := randomHex(16)
	if req.Token != nil && strings.TrimSpace(*req.Token) != "" {
		tok := strings.TrimSpace(*req.Token)
		tokExists, _ := db.DB.NewSelect().Model((*model.User)(nil)).Where("token = ?", tok).Exists(r.Context())
		if tokExists {
			RespondServiceError(w, http.StatusBadRequest, fmt.Sprintf("注册失败：token '%s' 已被使用", tok))
			return
		}
		token = tok
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = extID
	}

	userUUID := uuid.New().String()
	userPwd := randomHex(8)

	user := &model.User{
		ExternalID: &extID,
		Name:       name,
		Token:      token,
		UUID:       &userUUID,
		Password:   &userPwd,
		Remark:     req.Remark,
		IsActive:   true,
	}

	if _, err := db.DB.NewInsert().Model(user).Exec(r.Context()); err != nil {
		RespondServiceError(w, http.StatusInternalServerError, fmt.Sprintf("注册失败：%v", err))
		return
	}

	baseURL := getBaseURL(r)
	subURLs := map[string]string{
		"singbox":   fmt.Sprintf("%s/sub?token=%s", baseURL, token),
		"xray":      fmt.Sprintf("%s/xray?token=%s", baseURL, token),
		"raw_nodes": fmt.Sprintf("%s/node?token=%s", baseURL, token),
	}

	RespondServiceSuccess(w, ServiceUserRegisterRespData{
		UserID:           user.ID,
		Token:            token,
		ExternalID:       extID,
		SubscriptionURLs: subURLs,
	})
}

type ServiceAddCredentialReq struct {
	NodeTag  string  `json:"node_tag"`
	UUID     string  `json:"uuid"`
	Password *string `json:"password"`
	Remark   *string `json:"remark"`
}

// AddUserServiceCredential 添加用户子凭证 API (POST /api/service/users/{token}/credentials)
func AddUserServiceCredential(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		RespondServiceError(w, http.StatusBadRequest, "token is required")
		return
	}

	var user model.User
	if err := db.DB.NewSelect().Model(&user).Where("token = ? AND is_active = true", token).Scan(r.Context()); err != nil {
		RespondServiceError(w, http.StatusNotFound, "添加凭证失败：目标用户不存在或未激活")
		return
	}

	var req ServiceAddCredentialReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondServiceError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	nodeTag := strings.TrimSpace(req.NodeTag)
	if nodeTag == "" {
		RespondServiceError(w, http.StatusBadRequest, "添加凭证失败：node_tag 不能为空")
		return
	}

	credUUID := strings.TrimSpace(req.UUID)
	if credUUID == "" {
		RespondServiceError(w, http.StatusBadRequest, "添加凭证失败：uuid 不能为空")
		return
	}

	// 校验同名凭证去重
	tagExists, _ := db.DB.NewSelect().
		Model((*model.Endpoint)(nil)).
		Where("user_id = ? AND tag = ?", user.ID, nodeTag).
		Exists(r.Context())
	if tagExists {
		RespondServiceError(w, http.StatusBadRequest, fmt.Sprintf("添加凭证失败：该用户已存在同名节点凭证 '%s'", nodeTag))
		return
	}

	count, _ := db.DB.NewSelect().Model((*model.Endpoint)(nil)).Where("user_id = ?", user.ID).Count(r.Context())

	ep := &model.Endpoint{
		UserID:    user.ID,
		NodeID:    nil, // 待绑定物理机
		Tag:       nodeTag,
		UUID:      credUUID,
		Password:  req.Password,
		Status:    "pending_binding",
		IsActive:  true,
		SortOrder: count,
		Remark:    req.Remark,
	}

	if _, err := db.DB.NewInsert().Model(ep).Exec(r.Context()); err != nil {
		RespondServiceError(w, http.StatusInternalServerError, fmt.Sprintf("添加凭证失败：%v", err))
		return
	}

	RespondServiceSuccess(w, nil)
}

type StatelessNodeReq struct {
	Tag               string  `json:"tag"`
	Protocol          string  `json:"protocol"`
	ServerAddress     string  `json:"server_address"`
	ServerPort        int     `json:"server_port"`
	UUID              string  `json:"uuid"`
	Password          *string `json:"password,omitempty"`
	Security          string  `json:"security"`
	SNI               *string `json:"sni,omitempty"`
	PublicKey         *string `json:"public_key,omitempty"`
	ShortID           *string `json:"short_id,omitempty"`
	Fingerprint       *string `json:"fingerprint,omitempty"`
	Flow              *string `json:"flow,omitempty"`
	CongestionControl *string `json:"congestion_control,omitempty"`
	TransportType     string  `json:"transport_type,omitempty"`
	Path              *string `json:"path,omitempty"`
}

type StatelessRenderReq struct {
	Platform   string             `json:"platform"`
	TemplateID *int64             `json:"template_id,omitempty"`
	Nodes      []StatelessNodeReq `json:"nodes"`
}

// RenderConfigStateless 即席无状态配置渲染 (POST /api/service/render-config)
func RenderConfigStateless(w http.ResponseWriter, r *http.Request) {
	var req StatelessRenderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondServiceError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	if platform == "" {
		platform = "sing-box"
	}
	if platform != "sing-box" && platform != "xray" {
		RespondServiceError(w, http.StatusBadRequest, fmt.Sprintf("不支持的目标平台 '%s'，仅支持 'sing-box' 或 'xray'", platform))
		return
	}

	var customTemplateContent string
	if req.TemplateID != nil && *req.TemplateID > 0 {
		var tpl model.Template
		if err := db.DB.NewSelect().Model(&tpl).Where("id = ?", *req.TemplateID).Scan(r.Context()); err != nil {
			RespondServiceError(w, http.StatusBadRequest, fmt.Sprintf("未找到指定 template_id: %d", *req.TemplateID))
			return
		}
		customTemplateContent = tpl.Content
	} else if db.DB != nil {
		var tpl model.Template
		if err := db.DB.NewSelect().Model(&tpl).Where("target = ?", platform).Scan(r.Context()); err == nil && tpl.Content != "" {
			customTemplateContent = tpl.Content
		}
	}

	var endpoints []*model.Endpoint
	for i, nr := range req.Nodes {
		proto := strings.ToLower(strings.TrimSpace(nr.Protocol))
		if proto == "" {
			proto = "vless"
		}
		transportType := contract.NormalizeTransportType(nr.TransportType)
		sec := nr.Security
		if sec == "" && proto == "vless" {
			if transportType == "xhttp" {
				sec = "tls"
			} else {
				sec = "reality"
			}
		}
		n := &model.Node{
			Tag:               nr.Tag,
			NodeName:          nr.Tag,
			Protocol:          proto,
			ServerAddress:     nr.ServerAddress,
			ServerPort:        nr.ServerPort,
			Security:          sec,
			SNI:               nr.SNI,
			PublicKey:         nr.PublicKey,
			ShortID:           nr.ShortID,
			Fingerprint:       nr.Fingerprint,
			Flow:              nr.Flow,
			CongestionControl: nr.CongestionControl,
			TransportType:     transportType,
			Path:              nr.Path,
			IsActive:          true,
		}
		ep := &model.Endpoint{
			Node:      n,
			Tag:       nr.Tag,
			UUID:      nr.UUID,
			Password:  nr.Password,
			Status:    "active",
			IsActive:  true,
			SortOrder: i,
		}
		endpoints = append(endpoints, ep)
	}

	var renderedConfig map[string]interface{}
	var err error

	switch platform {
	case "sing-box":
		if customTemplateContent != "" {
			var tplMap map[string]interface{}
			if err := json.Unmarshal([]byte(customTemplateContent), &tplMap); err != nil {
				RespondServiceError(w, http.StatusBadRequest, fmt.Sprintf("解析自定义模板 JSON 失败: %v", err))
				return
			}
			renderedConfig, err = singbox.GenerateSingboxConfigFromTemplate(tplMap, endpoints, nil)
		} else {
			renderedConfig, err = singbox.GenerateSingboxConfigFromEndpoints(endpoints, nil)
		}
	case "xray":
		if customTemplateContent != "" {
			var tplMap map[string]interface{}
			if err := json.Unmarshal([]byte(customTemplateContent), &tplMap); err != nil {
				RespondServiceError(w, http.StatusBadRequest, fmt.Sprintf("解析自定义模板 JSON 失败: %v", err))
				return
			}
			renderedConfig, err = xray.GenerateXrayConfigFromTemplate(tplMap, endpoints, nil)
		} else {
			renderedConfig, err = xray.GenerateXrayConfigFromEndpoints(endpoints, nil)
		}
	}

	if err != nil {
		RespondServiceError(w, http.StatusBadRequest, fmt.Sprintf("渲染失败: %v", err))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":     0,
		"platform": platform,
		"config":   renderedConfig,
	})
}
