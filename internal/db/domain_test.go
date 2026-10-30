package db

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"noder/internal/config"
	"noder/internal/model"
)

func TestGetActiveDomainEndpointsEnforcesLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "noder-domain-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)
	config.Init(tmpDir)
	database, err := InitDB()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if report, err := AuditLegacyData(t.Context()); err != nil {
		t.Fatalf("legacy audit failed: %v", err)
	} else if report.UserNodeLinkCount == 0 || report.EndpointCount == 0 {
		t.Fatalf("expected seeded legacy records in audit: %+v", report)
	}

	user := &model.User{Name: "domain-user", Token: "domain-token", IsActive: true}
	if _, err := DB.NewInsert().Model(user).Exec(t.Context()); err != nil {
		t.Fatal(err)
	}
	node := &model.Node{Tag: "domain-node", NodeName: "domain-node", Protocol: "vless", ServerAddress: "example.com", ServerPort: 443, Security: "tls", IsActive: true}
	if _, err := DB.NewInsert().Model(node).Exec(t.Context()); err != nil {
		t.Fatal(err)
	}
	credential := &model.Credential{
		ID:           uuid.NewString(),
		UserID:       user.ID,
		Scheme:       "vless",
		UUIDValue:    stringPtr("00000000-0000-0000-0000-000000000001"),
		Status:       model.CredentialStatusActive,
		SecretConfig: "{}",
	}
	if _, err := DB.NewInsert().Model(credential).Exec(t.Context()); err != nil {
		t.Fatal(err)
	}
	grant := &model.AccessGrant{
		ID:             uuid.NewString(),
		UserID:         user.ID,
		EndpointID:     node.ID,
		CredentialID:   credential.ID,
		Status:         model.AccessGrantStatusActive,
		IsURLTest:      true,
		OverrideConfig: "{}",
	}
	if _, err := DB.NewInsert().Model(grant).Exec(t.Context()); err != nil {
		t.Fatal(err)
	}

	endpoints, found, err := GetActiveDomainEndpoints(t.Context(), user.ID)
	if err != nil || !found || len(endpoints) != 1 {
		t.Fatalf("expected one active domain endpoint, found=%v endpoints=%d err=%v", found, len(endpoints), err)
	}
	if endpoints[0].UUID != *credential.UUIDValue {
		t.Fatalf("credential was not resolved into node: %q", endpoints[0].UUID)
	}

	if _, err := DB.NewUpdate().Model(credential).Set("status = ?", model.CredentialStatusRevoked).WherePK().Exec(t.Context()); err != nil {
		t.Fatal(err)
	}
	endpoints, found, err = GetActiveDomainEndpoints(t.Context(), user.ID)
	if err != nil || !found || len(endpoints) != 0 {
		t.Fatalf("revoked credential must not be exported: found=%v endpoints=%d err=%v", found, len(endpoints), err)
	}

	if _, err := DB.NewUpdate().Model(credential).Set("status = ?", model.CredentialStatusActive).Set("expires_at = ?", time.Now().Add(-time.Minute)).WherePK().Exec(t.Context()); err != nil {
		t.Fatal(err)
	}
	endpoints, _, err = GetActiveDomainEndpoints(t.Context(), user.ID)
	if err != nil || len(endpoints) != 0 {
		t.Fatalf("expired credential must not be exported: endpoints=%d err=%v", len(endpoints), err)
	}

	if _, err := DB.NewUpdate().Model(credential).Set("status = ?", model.CredentialStatusActive).Set("expires_at = NULL").WherePK().Exec(t.Context()); err != nil {
		t.Fatal(err)
	}
	agent := &model.AgentNode{ID: uuid.NewString(), Name: "agent-01", Status: "online", Metadata: "{}"}
	if _, err := DB.NewInsert().Model(agent).Exec(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := DB.NewInsert().Model(&model.EndpointAgentBinding{EndpointID: node.ID, AgentNodeID: agent.ID, Status: "active"}).Exec(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := RevokeCredential(t.Context(), credential.ID); err != nil {
		t.Fatal(err)
	}
	deploymentCount, err := DB.NewSelect().Model((*model.CredentialDeployment)(nil)).Where("credential_id = ?", credential.ID).Count(t.Context())
	if err != nil || deploymentCount != 1 {
		t.Fatalf("expected one revocation deployment, count=%d err=%v", deploymentCount, err)
	}
}
