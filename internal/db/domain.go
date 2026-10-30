package db

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"noder/internal/model"
)

// ProjectLegacyData creates the canonical credential/grant projection for
// records stored in the pre-refactor tables. It is intentionally additive:
// existing canonical rows are never overwritten, so a disabled or revoked
// canonical grant remains authoritative after migration.
func ProjectLegacyData(ctx context.Context) error {
	var users []*model.User
	if err := DB.NewSelect().Model(&users).Column("id", "uuid", "password").Scan(ctx); err != nil {
		return err
	}
	for _, user := range users {
		if err := ProjectLegacyUser(ctx, user.ID); err != nil {
			return err
		}
	}
	return nil
}

// ProjectLegacyUser is also used by compatibility management handlers after a
// legacy endpoint is bound to a resource.
func ProjectLegacyUser(ctx context.Context, userID int64) error {
	var user model.User
	if err := DB.NewSelect().Model(&user).Where("id = ?", userID).Scan(ctx); err != nil {
		return err
	}

	var endpoints []*model.Endpoint
	if err := DB.NewSelect().Model(&endpoints).Relation("Node").Where("endpoint.user_id = ?", userID).Order("endpoint.sort_order ASC", "endpoint.id ASC").Scan(ctx); err != nil {
		return err
	}
	for _, legacy := range endpoints {
		if err := projectLegacyEndpoint(ctx, &user, legacy); err != nil {
			return err
		}
	}

	// A legacy user-level UUID/password is only a fallback source. Keep it as a
	// pending default credential until an endpoint is actually bound.
	if len(endpoints) == 0 && user.UUID != nil && strings.TrimSpace(*user.UUID) != "" {
		var count int
		count, _ = DB.NewSelect().Model((*model.Credential)(nil)).Where("user_id = ?", userID).Count(ctx)
		if count == 0 {
			credential := &model.Credential{
				ID:                 uuid.NewString(),
				UserID:             userID,
				Scheme:             "legacy",
				UUIDValue:          user.UUID,
				PasswordCiphertext: user.Password,
				Status:             model.CredentialStatusPendingBinding,
				IsDefault:          true,
				SecretConfig:       "{}",
			}
			if _, err := DB.NewInsert().Model(credential).Exec(ctx); err != nil {
				return fmt.Errorf("create default credential for user %d: %w", userID, err)
			}
		}
	}
	return nil
}

func projectLegacyEndpoint(ctx context.Context, user *model.User, legacy *model.Endpoint) error {
	if legacy == nil {
		return nil
	}
	scheme := "legacy"
	if legacy.Node != nil && strings.TrimSpace(legacy.Node.Protocol) != "" {
		scheme = strings.ToLower(strings.TrimSpace(legacy.Node.Protocol))
	}

	var credential model.Credential
	query := DB.NewSelect().Model(&credential).
		Where("user_id = ? AND (scheme = ? OR scheme = 'generic')", user.ID, scheme)
	if legacy.UUID != "" {
		query = query.Where("uuid_value = ?", legacy.UUID)
	} else {
		query = query.Where("uuid_value IS NULL")
	}
	if legacy.Password != nil {
		query = query.Where("password_ciphertext = ?", *legacy.Password)
	} else {
		query = query.Where("password_ciphertext IS NULL")
	}
	err := query.Scan(ctx)
	if err != nil {
		credential = model.Credential{
			ID:                 uuid.NewString(),
			UserID:             user.ID,
			Scheme:             scheme,
			DisplayName:        stringPtr(legacy.Tag),
			UUIDValue:          nullableString(legacy.UUID),
			Username:           legacy.Username,
			PasswordCiphertext: legacy.Password,
			Status:             model.CredentialStatusPendingBinding,
			SecretConfig:       "{}",
		}
		if _, insertErr := DB.NewInsert().Model(&credential).Exec(ctx); insertErr != nil {
			return fmt.Errorf("create credential for legacy endpoint %d: %w", legacy.ID, insertErr)
		}
	}

	if legacy.NodeID == nil {
		migration := &model.LegacyMigrationMap{
			LegacyUserID:     user.ID,
			LegacyEndpointID: legacy.ID,
			CredentialID:     credential.ID,
		}
		if _, err := DB.NewInsert().Model(migration).
			On("CONFLICT (legacy_endpoint_id) DO UPDATE").
			Set("credential_id = ?", credential.ID).
			Set("access_grant_id = NULL").Exec(ctx); err != nil {
			return err
		}
		return nil
	}

	var grant model.AccessGrant
	grantErr := DB.NewSelect().Model(&grant).
		Where("user_id = ? AND endpoint_id = ?", user.ID, *legacy.NodeID).Scan(ctx)
	var grantID *string
	if grantErr == nil {
		grantID = &grant.ID
	} else {
		status := model.AccessGrantStatusActive
		if legacy.Status != "" && legacy.Status != model.AccessGrantStatusActive || !legacy.IsActive {
			status = model.AccessGrantStatusDisabled
		}
		grant = model.AccessGrant{
			ID:             uuid.NewString(),
			UserID:         user.ID,
			EndpointID:     *legacy.NodeID,
			CredentialID:   credential.ID,
			DisplayName:    nullableString(legacy.Tag),
			Status:         status,
			IsURLTest:      legacy.IsURLTest == nil || *legacy.IsURLTest,
			SortOrder:      legacy.SortOrder,
			Remark:         legacy.Remark,
			OverrideConfig: "{}",
		}
		if _, err := DB.NewInsert().Model(&grant).Exec(ctx); err != nil {
			return fmt.Errorf("create access grant for legacy endpoint %d: %w", legacy.ID, err)
		}
		grantID = &grant.ID

		// A bound legacy endpoint activates only its own credential. Do not
		// revive an already disabled/revoked canonical credential.
		if credential.Status == model.CredentialStatusPendingBinding && status == model.AccessGrantStatusActive {
			_, err := DB.NewUpdate().Model((*model.Credential)(nil)).
				Set("status = ?", model.CredentialStatusActive).
				Where("id = ?", credential.ID).Exec(ctx)
			if err != nil {
				return err
			}
		}
	}

	// Keep the integer source IDs so a partial import can be diagnosed without
	// guessing which canonical UUID came from which legacy row.
	migration := &model.LegacyMigrationMap{
		LegacyUserID:     user.ID,
		LegacyNodeID:     legacy.NodeID,
		LegacyEndpointID: legacy.ID,
		CredentialID:     credential.ID,
		AccessGrantID:    grantID,
	}
	if _, err := DB.NewInsert().Model(migration).
		On("CONFLICT (legacy_endpoint_id) DO UPDATE").
		Set("credential_id = ?", credential.ID).
		Set("access_grant_id = ?", grantID).Exec(ctx); err != nil {
		return err
	}
	return nil
}

// GetActiveDomainEndpoints resolves the dynamic Node object from canonical
// AccessGrant + Credential rows. The bool distinguishes "no canonical rows"
// from "canonical rows exist but none are currently usable", which prevents a
// revoked grant from falling back to legacy data.
func GetActiveDomainEndpoints(ctx context.Context, userID int64) ([]*model.Endpoint, bool, error) {
	grantCount, err := DB.NewSelect().Model((*model.AccessGrant)(nil)).Where("user_id = ?", userID).Count(ctx)
	if err != nil {
		return nil, false, err
	} else if grantCount == 0 {
		return nil, false, nil
	}

	var grants []*model.AccessGrant
	if err := DB.NewSelect().Model(&grants).
		Relation("Endpoint").
		Relation("Credential").
		Where("access_grant.user_id = ?", userID).
		Where("access_grant.status = ?", model.AccessGrantStatusActive).
		Where("(access_grant.expires_at IS NULL OR access_grant.expires_at > CURRENT_TIMESTAMP)").
		Order("access_grant.sort_order ASC", "access_grant.id ASC").Scan(ctx); err != nil {
		return nil, true, err
	}

	now := time.Now()
	result := make([]*model.Endpoint, 0, len(grants))
	for _, grant := range grants {
		if grant == nil || grant.Endpoint == nil || grant.Credential == nil ||
			!grant.IsUsable(now) || !grant.Credential.IsUsable(now) ||
			!legacyNodeIsActive(grant.Endpoint) {
			continue
		}
		credential := grant.Credential
		uuidValue := ""
		if credential.UUIDValue != nil {
			uuidValue = *credential.UUIDValue
		}
		tag := grant.Endpoint.Tag
		if grant.DisplayName != nil && strings.TrimSpace(*grant.DisplayName) != "" {
			tag = *grant.DisplayName
		}
		if grant.TagOverride != nil && strings.TrimSpace(*grant.TagOverride) != "" {
			tag = *grant.TagOverride
		}
		result = append(result, &model.Endpoint{
			// The compatibility response historically exposed an integer
			// endpoint ID. A user can have only one grant per endpoint, so the
			// resource ID is a stable substitute while the API migrates to UUIDs.
			ID:        grant.EndpointID,
			UserID:    userID,
			NodeID:    &grant.EndpointID,
			Node:      grant.Endpoint,
			Tag:       tag,
			UUID:      uuidValue,
			Username:  credential.Username,
			Password:  credential.EffectivePassword(),
			Status:    model.AccessGrantStatusActive,
			IsActive:  true,
			IsURLTest: boolPtr(grant.IsURLTest),
			SortOrder: grant.SortOrder,
			Remark:    grant.Remark,
		})
	}
	return result, true, nil
}

// RevokeCredential changes control-plane state first, then creates an
// explicit revocation desired-state record for every bound AgentNode. Without
// an AgentNode there is no false claim of remote completion; an operator
// warning is emitted instead.
func RevokeCredential(ctx context.Context, credentialID string) error {
	now := time.Now()
	if _, err := DB.NewUpdate().Model((*model.Credential)(nil)).
		Set("status = ?", model.CredentialStatusRevoked).
		Set("revoked_at = ?", now).
		Where("id = ?", credentialID).Exec(ctx); err != nil {
		return err
	}

	var grants []*model.AccessGrant
	if err := DB.NewSelect().Model(&grants).Where("credential_id = ?", credentialID).Scan(ctx); err != nil {
		return err
	}
	createdDeployment := false
	for _, grant := range grants {
		var bindings []*model.EndpointAgentBinding
		if err := DB.NewSelect().Model(&bindings).Where("endpoint_id = ? AND status = ?", grant.EndpointID, "active").Scan(ctx); err != nil {
			return err
		}
		for _, binding := range bindings {
			deployment := &model.CredentialDeployment{
				CredentialID: credentialID,
				EndpointID:   grant.EndpointID,
				AgentNodeID:  binding.AgentNodeID,
				DesiredState: model.CredentialStatusRevoked,
				AppliedState: "unknown",
			}
			if _, err := DB.NewInsert().Model(deployment).On("CONFLICT (credential_id, endpoint_id, agent_node_id) DO UPDATE").
				Set("desired_state = ?", model.CredentialStatusRevoked).Exec(ctx); err != nil {
				return err
			}
			createdDeployment = true
		}
	}
	if !createdDeployment {
		log.Printf("warning: credential %s revoked without an AgentNode; manual proxy revocation is required", credentialID)
	}
	return nil
}

func legacyNodeIsActive(node *model.Node) bool {
	return node != nil && node.IsActive && (node.Status == "" || node.Status == model.AccessGrantStatusActive) && node.Protocol != "socks5"
}

func nullableString(value string) *string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return &value
}

func stringPtr(value string) *string { return &value }

func boolPtr(value bool) *bool { return &value }
