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
	"noder/internal/db"
	"noder/internal/model"
)

// RegisterDomainRoutes exposes the refactored resource-boundary API. The old
// /api/users/*/endpoints routes remain available as a compatibility facade.
func RegisterDomainRoutes(r chi.Router) {
	r.Route("/api/credentials", func(r chi.Router) {
		r.Use(AdminAuth)
		r.Get("/", ListCredentials)
		r.Put("/{id}", UpdateCredential)
	})
	r.Route("/api/access-grants", func(r chi.Router) {
		r.Use(AdminAuth)
		r.Post("/", CreateAccessGrant)
		r.Get("/", ListAccessGrants)
		r.Put("/{id}", UpdateAccessGrant)
	})
}

func ListCredentials(w http.ResponseWriter, r *http.Request) {
	query := db.DB.NewSelect().Model((*model.Credential)(nil)).Order("id ASC")
	if raw := strings.TrimSpace(r.URL.Query().Get("user_id")); raw != "" {
		userID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			RespondError(w, http.StatusBadRequest, "invalid user_id")
			return
		}
		query = query.Where("user_id = ?", userID)
	}
	var credentials []*model.Credential
	if err := query.Scan(r.Context(), &credentials); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if credentials == nil {
		credentials = []*model.Credential{}
	}
	RespondJSON(w, http.StatusOK, credentials)
}

type CredentialUpdate struct {
	Status    *string    `json:"status"`
	ExpiresAt *time.Time `json:"expires_at"`
}

func UpdateCredential(w http.ResponseWriter, r *http.Request) {
	credentialID := chi.URLParam(r, "id")
	var credential model.Credential
	if err := db.DB.NewSelect().Model(&credential).Where("id = ?", credentialID).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "Credential not found")
		return
	}
	var req CredentialUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if req.Status != nil {
		switch *req.Status {
		case model.CredentialStatusPendingBinding, model.CredentialStatusActive, model.CredentialStatusDisabled, model.CredentialStatusRevoked:
			credential.Status = *req.Status
		default:
			RespondError(w, http.StatusBadRequest, "invalid credential status")
			return
		}
	}
	if req.ExpiresAt != nil {
		credential.ExpiresAt = req.ExpiresAt
	}
	if credential.Status == model.CredentialStatusRevoked {
		if err := db.RevokeCredential(r.Context(), credential.ID); err != nil {
			RespondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		_ = db.DB.NewSelect().Model(&credential).Where("id = ?", credential.ID).Scan(r.Context())
	} else if _, err := db.DB.NewUpdate().Model(&credential).WherePK().Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, credential)
}

type AccessGrantInput struct {
	UserID         int64          `json:"user_id"`
	EndpointID     int64          `json:"endpoint_id"`
	CredentialID   string         `json:"credential_id"`
	DisplayName    *string        `json:"display_name"`
	TagOverride    *string        `json:"tag_override"`
	IsURLTest      *bool          `json:"is_urltest"`
	SortOrder      *int           `json:"sort_order"`
	OverrideConfig map[string]any `json:"override_config"`
	Remark         *string        `json:"remark"`
	ExpiresAt      *time.Time     `json:"expires_at"`
}

func CreateAccessGrant(w http.ResponseWriter, r *http.Request) {
	var req AccessGrantInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if req.UserID <= 0 || req.EndpointID <= 0 || strings.TrimSpace(req.CredentialID) == "" {
		RespondError(w, http.StatusBadRequest, "user_id, endpoint_id and credential_id are required")
		return
	}

	var credential model.Credential
	if err := db.DB.NewSelect().Model(&credential).Where("id = ?", req.CredentialID).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "Credential not found")
		return
	}
	if credential.UserID != req.UserID {
		RespondError(w, http.StatusBadRequest, "credential does not belong to user")
		return
	}
	var endpoint model.Node
	if err := db.DB.NewSelect().Model(&endpoint).Where("id = ?", req.EndpointID).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "Endpoint not found")
		return
	}

	var existing model.AccessGrant
	if err := db.DB.NewSelect().Model(&existing).
		Where("user_id = ? AND endpoint_id = ?", req.UserID, req.EndpointID).Scan(r.Context()); err == nil {
		// Binding is idempotent. Do not silently replace the credential attached
		// to an existing authorization.
		RespondJSON(w, http.StatusOK, existing)
		return
	}

	status := model.AccessGrantStatusActive
	isURLTest := true
	if req.IsURLTest != nil {
		isURLTest = *req.IsURLTest
	}
	sortOrder := 0
	if req.SortOrder != nil {
		sortOrder = *req.SortOrder
	}
	override := "{}"
	if req.OverrideConfig != nil {
		data, err := json.Marshal(req.OverrideConfig)
		if err != nil {
			RespondError(w, http.StatusBadRequest, "override_config must be valid JSON")
			return
		}
		override = string(data)
	}
	grant := &model.AccessGrant{
		ID:             uuid.NewString(),
		UserID:         req.UserID,
		EndpointID:     req.EndpointID,
		CredentialID:   credential.ID,
		DisplayName:    req.DisplayName,
		TagOverride:    req.TagOverride,
		Status:         status,
		IsURLTest:      isURLTest,
		SortOrder:      sortOrder,
		OverrideConfig: override,
		Remark:         req.Remark,
		ExpiresAt:      req.ExpiresAt,
	}
	if _, err := db.DB.NewInsert().Model(grant).Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, fmt.Sprintf("create access grant: %v", err))
		return
	}
	if credential.Status == model.CredentialStatusPendingBinding {
		_, _ = db.DB.NewUpdate().Model(&credential).Set("status = ?", model.CredentialStatusActive).WherePK().Exec(r.Context())
	}
	RespondJSON(w, http.StatusOK, grant)
}

func ListAccessGrants(w http.ResponseWriter, r *http.Request) {
	query := db.DB.NewSelect().Model((*model.AccessGrant)(nil)).Order("id ASC")
	if raw := strings.TrimSpace(r.URL.Query().Get("user_id")); raw != "" {
		userID, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			RespondError(w, http.StatusBadRequest, "invalid user_id")
			return
		}
		query = query.Where("user_id = ?", userID)
	}
	var grants []*model.AccessGrant
	if err := query.Scan(r.Context(), &grants); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if grants == nil {
		grants = []*model.AccessGrant{}
	}
	RespondJSON(w, http.StatusOK, grants)
}

type AccessGrantUpdate struct {
	Status      *string    `json:"status"`
	DisplayName *string    `json:"display_name"`
	TagOverride *string    `json:"tag_override"`
	IsURLTest   *bool      `json:"is_urltest"`
	SortOrder   *int       `json:"sort_order"`
	ExpiresAt   *time.Time `json:"expires_at"`
}

func UpdateAccessGrant(w http.ResponseWriter, r *http.Request) {
	var grant model.AccessGrant
	if err := db.DB.NewSelect().Model(&grant).Where("id = ?", chi.URLParam(r, "id")).Scan(r.Context()); err != nil {
		RespondError(w, http.StatusNotFound, "AccessGrant not found")
		return
	}
	var req AccessGrantUpdate
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid JSON body")
		return
	}
	if req.Status != nil {
		switch *req.Status {
		case model.AccessGrantStatusActive, model.AccessGrantStatusDisabled, model.AccessGrantStatusRevoked:
			grant.Status = *req.Status
		default:
			RespondError(w, http.StatusBadRequest, "invalid access grant status")
			return
		}
	}
	if req.DisplayName != nil {
		grant.DisplayName = req.DisplayName
	}
	if req.TagOverride != nil {
		grant.TagOverride = req.TagOverride
	}
	if req.IsURLTest != nil {
		grant.IsURLTest = *req.IsURLTest
	}
	if req.SortOrder != nil {
		grant.SortOrder = *req.SortOrder
	}
	if req.ExpiresAt != nil {
		grant.ExpiresAt = req.ExpiresAt
	}
	if _, err := db.DB.NewUpdate().Model(&grant).WherePK().Exec(r.Context()); err != nil {
		RespondError(w, http.StatusInternalServerError, err.Error())
		return
	}
	RespondJSON(w, http.StatusOK, grant)
}
