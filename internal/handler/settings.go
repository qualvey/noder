package handler

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"noder/internal/db"
	"noder/internal/model"
)

func RegisterSettingsRoutes(r chi.Router) {
	r.Route("/api/settings", func(r chi.Router) {
		r.Use(AdminAuth)
		r.Get("/", GetAllSettings)
		r.Put("/", UpdateSettings)
		r.Get("/shared-token", GetSharedToken)
		r.Post("/shared-token/reset", ResetSharedToken)
	})
}

func GetAllSettings(w http.ResponseWriter, r *http.Request) {
	var settings []*model.AppSetting
	if err := db.DB.NewSelect().Model(&settings).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	res := make(map[string]string)
	for _, s := range settings {
		res[s.Key] = s.Value
	}
	RespondJSON(w, http.StatusOK, res)
}

func UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var payload map[string]string
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}

	for k, v := range payload {
		setting := &model.AppSetting{Key: k, Value: v}
		_, _ = db.DB.NewInsert().Model(setting).On("CONFLICT (key) DO UPDATE").Set("value = EXCLUDED.value").Exec(r.Context())
	}

	RespondJSON(w, http.StatusOK, map[string]string{"message": "Settings updated"})
}
