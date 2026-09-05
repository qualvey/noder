package db

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"noder/internal/config"
	"noder/internal/model"
)

func TestInitDB(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "noder-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	config.Init(tmpDir)

	db, err := InitDB()
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// 检查种子节点
	var nodes []*model.Node
	if err := db.NewSelect().Model(&nodes).Scan(ctx); err != nil {
		t.Fatalf("Failed to query nodes: %v", err)
	}
	if len(nodes) != 3 {
		t.Errorf("Expected 3 seed nodes, got %d", len(nodes))
	}

	// 检查种子用户
	var users []*model.User
	if err := db.NewSelect().Model(&users).Relation("Nodes").Scan(ctx); err != nil {
		t.Fatalf("Failed to query users: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("Expected 1 seed user, got %d", len(users))
	}
	if len(users[0].Nodes) != 3 {
		t.Errorf("Expected user to have 3 nodes, got %d", len(users[0].Nodes))
	}

	// 检查 DB 文件是否存在
	if _, err := os.Stat(filepath.Join(tmpDir, "data.db")); err != nil {
		t.Errorf("data.db file not created: %v", err)
	}
}
