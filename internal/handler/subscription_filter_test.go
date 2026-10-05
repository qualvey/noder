package handler

import (
	"testing"

	"noder/internal/model"
)

func TestUserGetActiveEndpointsExcludesSocks5(t *testing.T) {
	nodeVless := &model.Node{ID: 1, Protocol: "vless", IsActive: true}
	nodeSocks5 := &model.Node{ID: 2, Protocol: "socks5", IsActive: true}
	nodeInactive := &model.Node{ID: 3, Protocol: "vless", IsActive: false}

	nid1 := int64(1)
	nid2 := int64(2)
	nid3 := int64(3)

	user := &model.User{
		ID:       1,
		Name:     "TestUser",
		IsActive: true,
		Endpoints: []*model.Endpoint{
			{ID: 1, UserID: 1, NodeID: &nid1, Node: nodeVless, IsActive: true, SortOrder: 0},
			{ID: 2, UserID: 1, NodeID: &nid2, Node: nodeSocks5, IsActive: true, SortOrder: 1},
			{ID: 3, UserID: 1, NodeID: &nid3, Node: nodeInactive, IsActive: true, SortOrder: 2},
			{ID: 4, UserID: 1, NodeID: nil, Node: nil, IsActive: true, SortOrder: 3}, // unbound
		},
	}

	active := user.GetActiveEndpoints()
	if len(active) != 1 {
		t.Fatalf("expected 1 active endpoint, got %d", len(active))
	}
	if active[0].Node.Protocol != "vless" {
		t.Fatalf("expected protocol vless, got %s", active[0].Node.Protocol)
	}
}
