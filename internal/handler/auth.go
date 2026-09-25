package handler

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"noder/internal/db"
	"noder/internal/model"
)

// RegisterAuthRoutes registers the server-side admin token check used by the UI.
func RegisterAuthRoutes(r *chi.Mux) {
	r.With(AdminAuth).Get("/api/auth/check", func(w http.ResponseWriter, _ *http.Request) {
		RespondJSON(w, http.StatusOK, map[string]bool{"valid": true})
	})
	r.With(AdminAuth).Post("/api/auth/change", func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Token == "" {
			RespondError(w, http.StatusBadRequest, "token is required")
			return
		}
		setting := &model.AppSetting{Key: "admin_secret_token", Value: payload.Token}
		if _, err := db.DB.NewInsert().Model(setting).On("CONFLICT (key) DO UPDATE").Set("value = EXCLUDED.value").Exec(r.Context()); err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		RespondJSON(w, http.StatusOK, map[string]bool{"updated": true})
	})
	r.Post("/api/auth/force-change", func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil || !(host == "127.0.0.1" || host == "::1" || strings.EqualFold(host, "localhost")) {
			RespondError(w, http.StatusForbidden, "force change is only available from localhost")
			return
		}
		var payload struct {
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil || payload.Token == "" {
			RespondError(w, http.StatusBadRequest, "token is required")
			return
		}
		setting := &model.AppSetting{Key: "admin_secret_token", Value: payload.Token}
		if _, err := db.DB.NewInsert().Model(setting).On("CONFLICT (key) DO UPDATE").Set("value = EXCLUDED.value").Exec(r.Context()); err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		RespondJSON(w, http.StatusOK, map[string]bool{"updated": true})
	})
}
