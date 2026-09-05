package db

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"gopkg.in/yaml.v3"
	_ "modernc.org/sqlite"

	"noder/internal/config"
	"noder/internal/model"
)

var DB *bun.DB

func InitDB() (*bun.DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", config.DBPath)
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite: %w", err)
	}

	sqldb.SetMaxOpenConns(1) // SQLite 单写入连接最佳实践

	DB = bun.NewDB(sqldb, sqlitedialect.New())
	DB.RegisterModel((*model.UserNodeLink)(nil), (*model.User)(nil), (*model.Node)(nil))

	ctx := context.Background()
	if err := CreateDBAndTables(ctx); err != nil {
		return nil, fmt.Errorf("failed to migrate db: %w", err)
	}

	if err := SeedDefaultData(ctx); err != nil {
		return nil, fmt.Errorf("failed to seed data: %w", err)
	}

	return DB, nil
}

func CreateDBAndTables(ctx context.Context) error {
	models := []interface{}{
		(*model.UserNodeLink)(nil),
		(*model.Node)(nil),
		(*model.User)(nil),
		(*model.DistFile)(nil),
		(*model.Template)(nil),
		(*model.TemplateHistory)(nil),
		(*model.AppSetting)(nil),
	}

	for _, m := range models {
		if _, err := DB.NewCreateTable().Model(m).IfNotExists().Exec(ctx); err != nil {
			return err
		}
	}

	// 兼容旧库迁移 (ALTER TABLE)
	nodeCols := []string{"public_key", "short_id", "fingerprint", "flow", "remark", "tag", "congestion_control", "sort_order"}
	for _, col := range nodeCols {
		_, _ = DB.ExecContext(ctx, fmt.Sprintf("ALTER TABLE node ADD COLUMN %s VARCHAR", col))
	}
	_, _ = DB.ExecContext(ctx, "UPDATE node SET tag = node_name WHERE tag IS NULL OR tag = ''")
	_, _ = DB.ExecContext(ctx, "UPDATE node SET transport_type = NULL, path = NULL WHERE protocol = 'tuic'")

	userCols := []string{"remark", "config_override", "node_order"}
	for _, col := range userCols {
		_, _ = DB.ExecContext(ctx, fmt.Sprintf("ALTER TABLE user ADD COLUMN %s VARCHAR", col))
	}

	fileCols := []string{"source_url", "cached_at", "download_name"}
	for _, col := range fileCols {
		_, _ = DB.ExecContext(ctx, fmt.Sprintf("ALTER TABLE distfile ADD COLUMN %s VARCHAR", col))
	}

	return nil
}

func randomHex(bytesLen int) string {
	b := make([]byte, bytesLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func SeedDefaultData(ctx context.Context) error {
	// 确保 shared_download_token 存在
	var setting model.AppSetting
	err := DB.NewSelect().Model(&setting).Where("key = ?", "shared_download_token").Scan(ctx)
	if err != nil {
		token := randomHex(16)
		_, _ = DB.NewInsert().Model(&model.AppSetting{Key: "shared_download_token", Value: token}).Exec(ctx)
	}

	// 确保默认模板存在
	seedTemplates(ctx)

	// 检查是否有节点
	count, _ := DB.NewSelect().Model((*model.Node)(nil)).Count(ctx)
	if count > 0 {
		return nil
	}

	tuicNode := &model.Node{
		NodeName:      "TUIC 高速专线 01",
		Tag:           "tuic-01",
		Protocol:      "tuic",
		ServerAddress: "tuic.example.com",
		ServerPort:    8443,
		Security:      "tls",
		SNI:           strPtr("tuic.example.com"),
		CongestionControl: strPtr("bbr"),
		IsActive:      true,
	}

	vlessNode := &model.Node{
		NodeName:      "VLESS REALITY 专线 02",
		Tag:           "vless-02",
		Protocol:      "vless",
		ServerAddress: "example.com",
		ServerPort:    8443,
		Security:      "reality",
		SNI:           strPtr("aws.amazon.com"),
		PublicKey:     strPtr("99BZ0JCnaSB55YEQYOCV66GhKTiK2ZGMPR3b6D_Q3wo"),
		ShortID:       strPtr("1a91"),
		Fingerprint:   strPtr("chrome"),
		Flow:          strPtr("xtls-rprx-vision"),
		IsActive:      true,
	}

	anytlsNode := &model.Node{
		NodeName:      "AnyTLS 极速专线 03",
		Tag:           "anytls-03",
		Protocol:      "anytls",
		ServerAddress: "anytls.example.com",
		ServerPort:    8443,
		Security:      "tls",
		SNI:           strPtr("anytls.example.com"),
		IsActive:      true,
	}

	if _, err := DB.NewInsert().Model(&[]*model.Node{tuicNode, vlessNode, anytlsNode}).Exec(ctx); err != nil {
		return err
	}

	testUser := &model.User{
		Name:     "张三",
		Token:    "my-secret-token",
		UUID:     strPtr(uuid.New().String()),
		Password: strPtr(randomHex(4)),
		IsActive: true,
	}
	if _, err := DB.NewInsert().Model(testUser).Exec(ctx); err != nil {
		return err
	}

	// 关联用户与节点
	links := []*model.UserNodeLink{
		{UserID: testUser.ID, NodeID: tuicNode.ID},
		{UserID: testUser.ID, NodeID: vlessNode.ID},
		{UserID: testUser.ID, NodeID: anytlsNode.ID},
	}
	_, _ = DB.NewInsert().Model(&links).Exec(ctx)

	// 默认模板
	seedTemplates(ctx)
	return nil
}

func seedTemplates(ctx context.Context) {
	defaultTpls := []struct {
		Target        string
		Name          string
		ContentFormat string
		Content       string
	}{
		{
			Target:        "sing-box",
			Name:          "sing-box 基础分流模板",
			ContentFormat: "json",
			Content:       loadSingboxTemplateJSON(),
		},
		{
			Target:        "mihomo",
			Name:          "Mihomo (Clash Meta) 标准模板",
			ContentFormat: "yaml",
			Content:       loadMihomoTemplateYAML(),
		},
	}

	for _, tpl := range defaultTpls {
		exists, _ := DB.NewSelect().Model((*model.Template)(nil)).Where("target = ?", tpl.Target).Exists(ctx)
		if !exists {
			newTpl := &model.Template{
				Target:        tpl.Target,
				Name:          tpl.Name,
				ContentFormat: tpl.ContentFormat,
				Content:       tpl.Content,
				Version:       1,
				UpdatedAt:     time.Now(),
			}
			if _, err := DB.NewInsert().Model(newTpl).Exec(ctx); err == nil {
				_, _ = DB.NewInsert().Model(&model.TemplateHistory{
					TemplateID: newTpl.ID,
					Target:     newTpl.Target,
					Version:    1,
					Content:    newTpl.Content,
					Remark:     "系统初始内置版本",
					CreatedAt:  time.Now(),
				}).Exec(ctx)
			}
		}
	}
}

func loadSingboxTemplateJSON() string {
	if data, err := os.ReadFile(config.SingBoxTemplatePath); err == nil && len(data) > 0 {
		return string(data)
	}
	return `{
  "log": {"level": "info", "timestamp": true},
  "outbounds": [
    {"type": "direct", "tag": "direct"},
    {"tag": "Proxy", "type": "selector", "outbounds": ["urltest"]},
    {"tag": "urltest", "type": "urltest", "outbounds": []}
  ]
}`
}

func loadMihomoTemplateYAML() string {
	if data, err := os.ReadFile(config.MihomoTemplatePath); err == nil && len(data) > 0 {
		return string(data)
	}
	basic := map[string]interface{}{
		"mixed-port": 7890,
		"log-level":  "info",
		"mode":       "rule",
		"proxies":    []interface{}{},
		"proxy-groups": []interface{}{
			map[string]interface{}{"name": "Proxy", "type": "select", "proxies": []string{"auto"}},
			map[string]interface{}{
				"name": "auto", "type": "url-test", "proxies": []string{},
				"url": "http://www.gstatic.com/generate_204", "interval": 300,
			},
		},
		"rules": []string{"MATCH,Proxy"},
	}
	out, _ := yaml.Marshal(basic)
	return string(out)
}

func strPtr(s string) *string {
	return &s
}
