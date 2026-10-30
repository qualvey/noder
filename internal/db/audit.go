package db

import (
	"context"
	"fmt"
)

// LegacyConflict is deliberately descriptive rather than destructive. Audit
// output is intended to be reviewed before a production import.
type LegacyConflict struct {
	Kind        string `json:"kind"`
	UserID      int64  `json:"user_id"`
	NodeID      *int64 `json:"node_id,omitempty"`
	EndpointID  *int64 `json:"endpoint_id,omitempty"`
	Description string `json:"description"`
}

type LegacyAuditReport struct {
	UserNodeLinkCount      int              `json:"user_node_link_count"`
	EndpointCount          int              `json:"endpoint_count"`
	LinksWithoutEndpoint   int              `json:"links_without_endpoint"`
	EndpointsWithoutLink   int              `json:"endpoints_without_link"`
	OrphanEndpointCount    int              `json:"orphan_endpoint_count"`
	DuplicateUserNodeCount int              `json:"duplicate_user_node_count"`
	Conflicts              []LegacyConflict `json:"conflicts"`
}

// AuditLegacyData performs the read-only checks required before importing old
// SQLite records into the canonical PostgreSQL model.
func AuditLegacyData(ctx context.Context) (LegacyAuditReport, error) {
	var report LegacyAuditReport
	var err error
	if report.UserNodeLinkCount, err = DB.NewSelect().TableExpr("usernodelink").Count(ctx); err != nil {
		return report, err
	}
	if report.EndpointCount, err = DB.NewSelect().TableExpr("endpoint").Count(ctx); err != nil {
		return report, err
	}
	queries := []struct {
		name   string
		target *int
		sql    string
	}{
		{"links_without_endpoint", &report.LinksWithoutEndpoint, `SELECT COUNT(*) FROM usernodelink l WHERE NOT EXISTS (SELECT 1 FROM endpoint e WHERE e.user_id = l.user_id AND e.node_id = l.node_id)`},
		{"endpoints_without_link", &report.EndpointsWithoutLink, `SELECT COUNT(*) FROM endpoint e WHERE e.node_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM usernodelink l WHERE l.user_id = e.user_id AND l.node_id = e.node_id)`},
		{"orphan_endpoint_count", &report.OrphanEndpointCount, `SELECT COUNT(*) FROM endpoint e WHERE e.node_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM node n WHERE n.id = e.node_id)`},
		{"duplicate_user_node_count", &report.DuplicateUserNodeCount, `SELECT COUNT(*) FROM (SELECT user_id, node_id FROM endpoint WHERE node_id IS NOT NULL GROUP BY user_id, node_id HAVING COUNT(*) > 1)`},
	}
	for _, query := range queries {
		if err := DB.NewRaw(query.sql).Scan(ctx, query.target); err != nil {
			return report, fmt.Errorf("audit %s: %w", query.name, err)
		}
	}
	if report.Conflicts == nil {
		report.Conflicts = []LegacyConflict{}
	}
	return report, nil
}
