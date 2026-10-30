package model

import (
	"encoding/json"
	"sort"
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
	Country           *string `bun:"country" json:"country,omitempty"`
	Remark            *string `bun:"remark" json:"remark,omitempty"`
	SortOrder         int     `bun:"sort_order,default:0" json:"sort_order"`
	// Status and Config are the canonical endpoint lifecycle/configuration
	// fields. The legacy columns above remain temporarily for compatibility
	// with existing SQLite installations and are not used for credentials.
	Status string  `bun:"status,notnull,default:'active'" json:"status"`
	Config *string `bun:"config" json:"config,omitempty"`

	Users []*User `bun:"m2m:usernodelink,join:Node=User" json:"-"`
}

const (
	CredentialStatusPendingBinding = "pending_binding"
	CredentialStatusActive         = "active"
	CredentialStatusDisabled       = "disabled"
	CredentialStatusRevoked        = "revoked"

	AccessGrantStatusActive   = "active"
	AccessGrantStatusDisabled = "disabled"
	AccessGrantStatusRevoked  = "revoked"
)

// Credential is the authoritative data-plane identity of a user. Password is
// intentionally not a database column: PasswordCiphertext is the persisted
// value and the transient Password field exists only for the current
// compatibility layer until an encryption key is configured.
type Credential struct {
	bun.BaseModel `bun:"table:credentials"`

	ID                 string     `bun:"id,pk" json:"id"`
	UserID             int64      `bun:"user_id,notnull" json:"user_id"`
	User               *User      `bun:"rel:belongs-to,join:user_id=id" json:"-"`
	Scheme             string     `bun:"scheme,notnull" json:"scheme"`
	DisplayName        *string    `bun:"display_name" json:"display_name,omitempty"`
	UUIDValue          *string    `bun:"uuid_value" json:"uuid_value,omitempty"`
	Username           *string    `bun:"username" json:"username,omitempty"`
	PasswordCiphertext *string    `bun:"password_ciphertext" json:"-"`
	Password           *string    `bun:"-" json:"-"`
	SecretConfig       string     `bun:"secret_config,notnull,default:'{}'" json:"secret_config"`
	Status             string     `bun:"status,notnull,default:'pending_binding'" json:"status"`
	IsDefault          bool       `bun:"is_default,notnull,default:false" json:"is_default"`
	ExpiresAt          *time.Time `bun:"expires_at" json:"expires_at,omitempty"`
	RevokedAt          *time.Time `bun:"revoked_at" json:"revoked_at,omitempty"`
	CreatedAt          time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt          time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

func (c *Credential) EffectivePassword() *string {
	if c == nil {
		return nil
	}
	if c.Password != nil {
		return c.Password
	}
	return c.PasswordCiphertext
}

func (c *Credential) IsUsable(now time.Time) bool {
	return c != nil && c.Status == CredentialStatusActive &&
		(c.ExpiresAt == nil || c.ExpiresAt.After(now)) && c.RevokedAt == nil
}

// AccessGrant is the authoritative user-to-resource authorization. It never
// copies UUID, username, or password values from Credential.
type AccessGrant struct {
	bun.BaseModel `bun:"table:access_grants"`

	ID             string      `bun:"id,pk" json:"id"`
	UserID         int64       `bun:"user_id,notnull" json:"user_id"`
	User           *User       `bun:"rel:belongs-to,join:user_id=id" json:"-"`
	EndpointID     int64       `bun:"endpoint_id,notnull" json:"endpoint_id"`
	Endpoint       *Node       `bun:"rel:belongs-to,join:endpoint_id=id" json:"endpoint,omitempty"`
	CredentialID   string      `bun:"credential_id,notnull" json:"credential_id"`
	Credential     *Credential `bun:"rel:belongs-to,join:credential_id=id" json:"-"`
	DisplayName    *string     `bun:"display_name" json:"display_name,omitempty"`
	TagOverride    *string     `bun:"tag_override" json:"tag_override,omitempty"`
	Status         string      `bun:"status,notnull,default:'active'" json:"status"`
	IsURLTest      bool        `bun:"is_urltest,notnull,default:true" json:"is_urltest"`
	SortOrder      int         `bun:"sort_order,notnull,default:0" json:"sort_order"`
	OverrideConfig string      `bun:"override_config,notnull,default:'{}'" json:"override_config"`
	Remark         *string     `bun:"remark" json:"remark,omitempty"`
	ExpiresAt      *time.Time  `bun:"expires_at" json:"expires_at,omitempty"`
	CreatedAt      time.Time   `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt      time.Time   `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

func (g *AccessGrant) IsUsable(now time.Time) bool {
	return g != nil && g.Status == AccessGrantStatusActive &&
		(g.ExpiresAt == nil || g.ExpiresAt.After(now))
}

// AgentNode and the two binding/deployment models are control-plane state.
// They deliberately do not change User, Credential, or AccessGrant ownership.
type AgentNode struct {
	bun.BaseModel `bun:"table:agent_nodes"`

	ID                     string     `bun:"id,pk" json:"id"`
	Name                   string     `bun:"name,notnull" json:"name"`
	ControlTokenCiphertext *string    `bun:"control_token_ciphertext" json:"-"`
	Status                 string     `bun:"status,notnull,default:'offline'" json:"status"`
	LastHeartbeatAt        *time.Time `bun:"last_heartbeat_at" json:"last_heartbeat_at,omitempty"`
	Metadata               string     `bun:"metadata,notnull,default:'{}'" json:"metadata"`
	CreatedAt              time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt              time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

type EndpointAgentBinding struct {
	bun.BaseModel `bun:"table:endpoint_agent_bindings"`
	EndpointID    int64      `bun:"endpoint_id,pk" json:"endpoint_id"`
	AgentNodeID   string     `bun:"agent_node_id,pk" json:"agent_node_id"`
	Status        string     `bun:"status,notnull,default:'active'" json:"status"`
	LastSyncAt    *time.Time `bun:"last_sync_at" json:"last_sync_at,omitempty"`
	LastError     *string    `bun:"last_error" json:"last_error,omitempty"`
	CreatedAt     time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
}

type CredentialDeployment struct {
	bun.BaseModel `bun:"table:credential_deployments"`
	CredentialID  string     `bun:"credential_id,pk" json:"credential_id"`
	EndpointID    int64      `bun:"endpoint_id,pk" json:"endpoint_id"`
	AgentNodeID   string     `bun:"agent_node_id,pk" json:"agent_node_id"`
	DesiredState  string     `bun:"desired_state,notnull" json:"desired_state"`
	AppliedState  string     `bun:"applied_state,notnull,default:'unknown'" json:"applied_state"`
	LastSyncAt    *time.Time `bun:"last_sync_at" json:"last_sync_at,omitempty"`
	LastError     *string    `bun:"last_error" json:"last_error,omitempty"`
}

// LegacyMigrationMap keeps the integer IDs needed to audit and replay an old
// SQLite import after canonical UUIDs have been assigned.
type LegacyMigrationMap struct {
	bun.BaseModel    `bun:"table:legacy_migration_map"`
	ID               int64     `bun:"id,pk,autoincrement" json:"id"`
	LegacyUserID     int64     `bun:"legacy_user_id,notnull" json:"legacy_user_id"`
	LegacyNodeID     *int64    `bun:"legacy_node_id" json:"legacy_node_id,omitempty"`
	LegacyEndpointID int64     `bun:"legacy_endpoint_id,notnull,unique" json:"legacy_endpoint_id"`
	CredentialID     string    `bun:"credential_id,notnull" json:"credential_id"`
	AccessGrantID    *string   `bun:"access_grant_id" json:"access_grant_id,omitempty"`
	Conflict         *string   `bun:"conflict" json:"conflict,omitempty"`
	CreatedAt        time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
}

// Endpoint 完整节点实体（入口节点 + 鉴权凭证 + 路由 Tag）
type Endpoint struct {
	bun.BaseModel `bun:"table:endpoint"`

	ID     int64  `bun:"id,pk,autoincrement" json:"id"`
	UserID int64  `bun:"user_id,notnull" json:"user_id"`
	User   *User  `bun:"rel:belongs-to,join:user_id=id" json:"-"`
	NodeID *int64 `bun:"node_id" json:"node_id"`
	Node   *Node  `bun:"rel:belongs-to,join:node_id=id" json:"node,omitempty"`

	Tag      string  `bun:"tag,notnull" json:"tag"`
	UUID     string  `bun:"uuid,notnull" json:"uuid"`
	Username *string `bun:"username" json:"username,omitempty"`
	Password *string `bun:"password" json:"password,omitempty"`
	Status   string  `bun:"status,notnull,default:'active'" json:"status"`
	Country  *string `bun:"country" json:"country,omitempty"`
	// Do not declare a Bun default here. Bun treats zero values on INSERT as
	// DEFAULT when the field has a SQL default, which turns an explicitly
	// disabled endpoint back on (the database default is true).
	IsActive  bool  `bun:"is_active,notnull" json:"is_active"`
	IsURLTest *bool `bun:"is_urltest,notnull,default:true" json:"is_urltest"`
	// Zero is a valid order. A Bun default would make multi-row inserts derive
	// the INSERT columns from the first row and drop later non-zero values.
	SortOrder int     `bun:"sort_order,notnull" json:"sort_order"`
	Remark    *string `bun:"remark" json:"remark,omitempty"`

	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

// User 用户模型
type User struct {
	bun.BaseModel `bun:"table:user"`

	ID                 int64   `bun:"id,pk,autoincrement" json:"id"`
	ExternalID         *string `bun:"external_id,unique" json:"external_id,omitempty"`
	Name               string  `bun:"name,notnull" json:"name"`
	Token              string  `bun:"token,notnull,unique" json:"token"`
	IsActive           bool    `bun:"is_active,notnull,default:true" json:"is_active"`
	Status             string  `bun:"status,notnull,default:'active'" json:"status"`
	UUID               *string `bun:"uuid" json:"uuid,omitempty"`
	Password           *string `bun:"password" json:"password,omitempty"`
	Remark             *string `bun:"remark" json:"remark,omitempty"`
	ConfigOverride     *string `bun:"config_override" json:"config_override,omitempty"`
	NodeOrder          *string `bun:"node_order" json:"node_order,omitempty"`
	TemplateBranchID   *int64  `bun:"template_branch_id" json:"template_branch_id,omitempty"`
	IndependentNodeIDs *string `bun:"independent_node_ids" json:"independent_node_ids,omitempty"`

	Endpoints []*Endpoint `bun:"rel:has-many,join:id=user_id" json:"endpoints,omitempty"`
	Nodes     []*Node     `bun:"m2m:usernodelink,join:User=Node" json:"-"`

	// 序列化时动态装配
	NodeIDs []int64 `bun:"-" json:"node_ids"`
}

func (u *User) GetIndependentNodeIDs() map[int64]bool {
	result := make(map[int64]bool)
	if u.IndependentNodeIDs == nil {
		return result
	}
	var ids []int64
	if json.Unmarshal([]byte(*u.IndependentNodeIDs), &ids) == nil {
		for _, id := range ids {
			result[id] = true
		}
	}
	return result
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

// GetActiveEndpoints 返回该用户已启用且对应物理入口节点也启用的端点列表（按 SortOrder 排序，排除 socks5 独立业务线）
func (u *User) GetActiveEndpoints() []*Endpoint {
	var res []*Endpoint
	for _, ep := range u.Endpoints {
		if ep.IsActive && ep.Node != nil && ep.Node.IsActive && ep.Node.Protocol != "socks5" {
			res = append(res, ep)
		}
	}
	sort.SliceStable(res, func(i, j int) bool {
		return res[i].SortOrder < res[j].SortOrder
	})
	return res
}

// DistFile 分发文件模型
type DistFile struct {
	bun.BaseModel `bun:"table:distfile"`

	ID           int64   `bun:"id,pk,autoincrement" json:"id"`
	Name         string  `bun:"name,notnull" json:"name"`
	FileType     string  `bun:"file_type,notnull" json:"file_type"` // apk | zip | text
	TemplateName *string `bun:"template_name" json:"template_name,omitempty"`
	OriginalName string  `bun:"original_name" json:"original_name"`
	StoredName   string  `bun:"stored_name,notnull" json:"stored_name"`
	DownloadName *string `bun:"download_name" json:"download_name,omitempty"`
	Size         int64   `bun:"size,notnull" json:"size"`
	IsActive     bool    `bun:"is_active,notnull,default:true" json:"is_active"`
	Remark       *string `bun:"remark" json:"remark,omitempty"`
	SourceURL    *string `bun:"source_url" json:"source_url,omitempty"`
	CachedAt     *string `bun:"cached_at" json:"cached_at,omitempty"`
	CreatedAt    string  `bun:"created_at" json:"created_at"`
}

// UserDownloadLog records a successful file download authenticated by a user token.
type UserDownloadLog struct {
	bun.BaseModel `bun:"table:user_download_log"`

	ID           int64     `bun:"id,pk,autoincrement" json:"id"`
	UserID       int64     `bun:"user_id,notnull" json:"user_id"`
	UserName     string    `bun:"user_name,notnull" json:"user_name"`
	FileID       int64     `bun:"file_id,notnull" json:"file_id"`
	FileName     string    `bun:"file_name,notnull" json:"file_name"`
	DownloadedAt time.Time `bun:"downloaded_at,nullzero,notnull,default:current_timestamp" json:"downloaded_at"`
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

// TemplateBranch 模板分支（独立副本，创建后与原模板无关）
type TemplateBranch struct {
	bun.BaseModel `bun:"table:template_branch"`

	ID            int64     `bun:"id,pk,autoincrement" json:"id"`
	Name          string    `bun:"name,notnull" json:"name"`
	Target        string    `bun:"target,notnull" json:"target"`                 // sing-box | mihomo
	ContentFormat string    `bun:"content_format,notnull" json:"content_format"` // json | yaml
	Content       string    `bun:"content,notnull" json:"content"`
	Remark        *string   `bun:"remark" json:"remark,omitempty"`
	IsActive      bool      `bun:"is_active,default:true" json:"is_active"`
	CreatedAt     time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

// AppSetting 全局设置模型
type AppSetting struct {
	bun.BaseModel `bun:"table:appsetting"`

	Key   string `bun:"key,pk" json:"key"`
	Value string `bun:"value,notnull" json:"value"`
}
