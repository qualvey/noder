package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"noder/internal/config"
	"noder/internal/db"
	"noder/internal/model"
)

// ServiceAuth 针对微服务接口专属的请求头鉴权中间件 (X-Service-Token: <MALL_SERVICE_SECRET>)
func ServiceAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var setting model.AppSetting
		expected := ""
		if db.DB != nil {
			if err := db.DB.NewSelect().Model(&setting).Where("key = ?", "mall_service_secret").Scan(r.Context()); err == nil && setting.Value != "" {
				expected = setting.Value
			}
		}
		if expected == "" {
			expected = config.MallServiceSecret
		}
		if expected == "" {
			expected = "service-secret"
		}

		token := r.Header.Get("X-Service-Token")
		if token == "" {
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				parts := strings.Fields(authHeader)
				if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
					token = parts[1]
				} else {
					token = authHeader
				}
			}
		}

		if token == "" || token != expected {
			RespondServiceError(w, http.StatusUnauthorized, "Invalid or missing X-Service-Token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RespondServiceSuccess 返回微服务统一规范成功响应
func RespondServiceSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	resp := map[string]interface{}{
		"code":    0,
		"message": "success",
	}
	if data != nil {
		resp["data"] = data
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// RespondServiceError 返回微服务统一规范错误响应
func RespondServiceError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"code":    status,
		"message": msg,
	})
}
