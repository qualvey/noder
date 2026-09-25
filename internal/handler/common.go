package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"noder/internal/config"
	"noder/internal/contract"
	"noder/internal/db"
	"noder/internal/model"
)

func RespondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func RespondError(w http.ResponseWriter, status int, detail string) {
	RespondJSON(w, status, map[string]interface{}{"detail": detail})
}

func HandleAPIError(w http.ResponseWriter, err error) {
	if apiErr, ok := err.(*contract.APIError); ok {
		RespondError(w, apiErr.StatusCode, apiErr.Detail)
		return
	}
	RespondError(w, http.StatusInternalServerError, err.Error())
}

func AdminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var setting model.AppSetting
		expected := ""
		if err := db.DB.NewSelect().Model(&setting).Where("key = ?", "admin_secret_token").Scan(r.Context()); err == nil {
			expected = setting.Value
		} else {
			// 兼容数据库初始化失败或旧运行模式。
			expected = config.AdminSecretToken
		}
		if expected == "" {
			next.ServeHTTP(w, r)
			return
		}

		token := r.Header.Get("X-Admin-Token")
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
			RespondError(w, http.StatusUnauthorized, "Invalid or missing admin token")
			return
		}

		next.ServeHTTP(w, r)
	})
}
