package model

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/uptrace/bun"
)

// UserNodeLink 多对多关联表
type UserNodeLink struct {
	bun.BaseModel `bun:"table:usernodelink"`
	UserID        int64 `bun:"user_id,pk"`
	User          *User `bun:"rel:belongs-to,join:user_id=id"`
	NodeID        int64 `bun:"node_id,pk"`
	Node          *Node `bun:"rel:belongs-to,join:node_id=id"`
}

// Node 节点模型
type Node struct {
	bun.BaseModel `bun:"table:node"`

	ID                int64   `bun:"id,pk,autoincrement" json:"id"`
	Tag               string  `bun:"tag,notnull" json:"tag"`
	NodeName          string  `bun:"node_name" json:"node_name"`
	Protocol          string  `bun:"protocol,notnull" json:"protocol"`
	ServerAddress     string  `bun:"server_address,notnull" json:"server_address"`
	ServerPort        int     `bun:"server_port,notnull" json:"server_port"`
	Method            *string `bun:"method" json:"method,omitempty"`
	Security          string  `bun:"security" json:"security"`
	SNI               *string `bun:"sni" json:"sni,omitempty"`
	TransportType     string  `bun:"transport_type" json:"transport_type"`
	Path              *string `bun:"path" json:"path,omitempty"`
	IsActive          bool    `bun:"is_active,notnull,default:true" json:"is_active"`
	PublicKey         *string `bun:"public_key" json:"public_key,omitempty"`
	ShortID           *string `bun:"short_id" json:"short_id,omitempty"`
	Fingerprint       *string `bun:"fingerprint" json:"fingerprint,omitempty"`
	Flow              *string `bun:"flow" json:"flow,omitempty"`
	CongestionControl *string `bun:"congestion_control" json:"congestion_control,omitempty"`
	Remark            *string `bun:"remark" json:"remark,omitempty"`
	SortOrder         int     `bun:"sort_order,default:0" json:"sort_order"`

	Users []*User `bun:"m2m:usernodelink,join:Node=User" json:"-"`
}

// User 用户模型
type User struct {
	bun.BaseModel `bun:"table:user"`

	ID             int64   `bun:"id,pk,autoincrement" json:"id"`
	Name           string  `bun:"name,notnull" json:"name"`
	Token          string  `bun:"token,notnull,unique" json:"token"`
	IsActive       bool    `bun:"is_active,notnull,default:true" json:"is_active"`
	UUID           *string `bun:"uuid" json:"uuid,omitempty"`
	Password       *string `bun:"password" json:"password,omitempty"`
	Remark         *string `bun:"remark" json:"remark,omitempty"`
	ConfigOverride *string `bun:"config_override" json:"config_override,omitempty"`
	NodeOrder      *string `bun:"node_order" json:"node_order,omitempty"`

	Nodes []*Node `bun:"m2m:usernodelink,join:User=Node" json:"-"`

	// 序列化时动态装配
	NodeIDs []int64 `bun:"-" json:"node_ids"`
}

// GetSortedNodeIDs 返回该用户实际关联的、按 user.NodeOrder 优先排定的节点 ID 列表
func (u *User) GetSortedNodeIDs() []int64 {
	actualMap := make(map[int64]bool)
	for _, n := range u.Nodes {
		actualMap[n.ID] = true
	}

	var orderedList []int64
	if u.NodeOrder != nil && *u.NodeOrder != "" {
		var parsed []interface{}
		if err := json.Unmarshal([]byte(*u.NodeOrder), &parsed); err == nil {
			for _, item := range parsed {
				switch v := item.(type) {
				case float64:
					orderedList = append(orderedList, int64(v))
				case string:
					if id, err := strconv.ParseInt(v, 10, 64); err == nil {
						orderedList = append(orderedList, id)
					}
				}
			}
		}
	}

	seen := make(map[int64]bool)
	var result []int64
	for _, id := range orderedList {
		if actualMap[id] && !seen[id] {
			result = append(result, id)
			seen[id] = true
		}
	}
	for _, n := range u.Nodes {
		if !seen[n.ID] {
			result = append(result, n.ID)
			seen[n.ID] = true
		}
	}
	return result
}

// DistFile 分发文件模型
type DistFile struct {
	bun.BaseModel `bun:"table:distfile"`

	ID           int64   `bun:"id,pk,autoincrement" json:"id"`
	Name         string  `bun:"name,notnull" json:"name"`
	FileType     string  `bun:"file_type,notnull" json:"file_type"` // apk | zip | text
	TemplateName *string `bun:"template_name" json:"template_name,omitempty"`
	OriginalName string  `bun:"original_name" json:"original_name"`
	DownloadName *string `bun:"download_name" json:"download_name,omitempty"`
	Size         int64   `bun:"size,notnull" json:"size"`
	IsActive     bool    `bun:"is_active,notnull,default:true" json:"is_active"`
	Remark       *string `bun:"remark" json:"remark,omitempty"`
	SourceURL    *string `bun:"source_url" json:"source_url,omitempty"`
	CachedAt     *string `bun:"cached_at" json:"cached_at,omitempty"`
	CreatedAt    string  `bun:"created_at" json:"created_at"`
}

// Template 模板模型
type Template struct {
	bun.BaseModel `bun:"table:template"`

	ID            int64     `bun:"id,pk,autoincrement" json:"id"`
	Target        string    `bun:"target,notnull" json:"target"` // sing-box | mihomo
	Name          string    `bun:"name,notnull" json:"name"`
	ContentFormat string    `bun:"content_format,notnull" json:"content_format"` // json | yaml
	Content       string    `bun:"content,notnull" json:"content"`
	Version       int       `bun:"version,notnull" json:"version"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

// TemplateHistory 模板历史记录
type TemplateHistory struct {
	bun.BaseModel `bun:"table:templatehistory"`

	ID         int64     `bun:"id,pk,autoincrement" json:"id"`
	TemplateID int64     `bun:"template_id,notnull" json:"template_id"`
	Target     string    `bun:"target,notnull" json:"target"`
	Version    int       `bun:"version,notnull" json:"version"`
	Content    string    `bun:"content,notnull" json:"content"`
	Remark     string    `bun:"remark" json:"remark"`
	CreatedAt  time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
}

// AppSetting 全局设置模型
type AppSetting struct {
	bun.BaseModel `bun:"table:appsetting"`

	Key   string `bun:"key,pk" json:"key"`
	Value string `bun:"value,notnull" json:"value"`
}
