package database

import (
    "fmt"
    "os"
    "tab-sync-backend-refactor/pkg/models"
)

// DatabaseInterface 定义数据库访问接口
type DatabaseInterface interface {
    // 用户管理
    CreateUser(user *models.User) error
    GetUserByEmail(email string) (*models.User, error)
    GetUserByID(id string) (*models.User, error)
    UpdateUser(user *models.User) error
    DeleteUser(id string) error

    // 用户订阅信息
    GetUserWithSubscription(userID string) (*models.UserWithSubscription, error)

    // Organizations & Memberships
    CreateOrganization(org *models.Organization) error
    UpdateOrganization(org *models.Organization) error
    ListUserOrganizations(userID string) ([]models.Organization, error)
    GetOrganization(orgID string) (*models.Organization, error)
    AddOrganizationMember(m *models.OrganizationMembership) error
    ListOrganizationMembers(orgID string) ([]models.OrganizationMembership, error)

    // Spaces
    CreateSpace(space *models.Space) error
    ListSpacesByOrganization(orgID string) ([]models.Space, error)
    UpdateSpace(space *models.Space) error
    GetSpaceByID(spaceID string) (*models.Space, error)
    DeleteSpace(spaceID string) error
    SetSpacePermission(spaceID, userID string, canEdit bool) error
    GetSpacePermissions(spaceID string) ([]models.SpacePermission, error)

    // Collections
    CreateCollection(c *models.Collection) error
    UpdateCollection(c *models.Collection) error
    DeleteCollection(id string) error
    ListCollectionsBySpace(spaceID string) ([]models.Collection, error)
    GetCollection(id string) (*models.Collection, error)

    // Collection Items
    CreateCollectionItem(it *models.CollectionItem) error
    // UpdateCollectionItem updates all provided fields on item; kept for backward compatibility.
    // Prefer UpdateCollectionItemPartial to avoid overwriting unspecified fields.
    UpdateCollectionItem(it *models.CollectionItem) error
    // UpdateCollectionItemPartial performs a partial update using the provided patch map.
    // Allowed keys: "collection_id","title","url","fav_icon_url","original_title",
    // "ai_generated_title","domain","metadata","position".
    UpdateCollectionItemPartial(itemID string, patch map[string]interface{}) error
    DeleteCollectionItem(id string) error
    ListItemsByCollection(collectionID string) ([]models.CollectionItem, error)
    // Idempotency helpers
    FindItemByCollectionAndNormalizedURL(collectionID, normalizedURL string) (*models.CollectionItem, error)

    // Invitations
    CreateInvitation(inv *models.OrganizationInvitation) error
    GetInvitationByToken(token string) (*models.OrganizationInvitation, error)
    ListInvitationsByEmail(email string) ([]models.OrganizationInvitation, error)
    UpdateInvitation(inv *models.OrganizationInvitation) error

    // 快照管理
    SaveSnapshot(userID, name string, tabGroups []models.TabGroup) error
    ListSnapshots(userID string) ([]SnapshotInfo, error)
    LoadSnapshot(userID, name string) (*LoadSnapshotResponse, error)
    DeleteSnapshot(userID, name string) error

    // 订阅管理
    CreateSubscription(subscription *models.UserSubscription) error
    GetUserSubscription(userID string) (*models.UserSubscription, error)
    UpdateSubscription(subscription *models.UserSubscription) error
    CancelSubscription(userID string) error

    // AI 额度管理
    GetUserAICredits(userID string) (*models.AICredits, error)
    UpdateAICredits(credits *models.AICredits) error
    ConsumeAICredits(userID string, amount int) error

    // 健康检查
    HealthCheck() error

    // 关闭连接
    Close() error
}

// SnapshotInfo 列表信息（用于本地/远程统一返回）
type SnapshotInfo struct {
    Name       string `json:"name"`
    CreatedAt  string `json:"created_at"`
    UpdatedAt  string `json:"updated_at"`
    TabCount   int    `json:"tab_count"`
    GroupCount int    `json:"group_count"`
}

// LoadSnapshotResponse 加载响应结构
type LoadSnapshotResponse struct {
    Name      string            `json:"name"`
    TabGroups []models.TabGroup `json:"tabGroups"`
    CreatedAt string            `json:"createdAt"`
    UpdatedAt string            `json:"updatedAt"`
}

// DatabaseConfig 数据库配置（仅保留外部数据库）
type DatabaseConfig struct {
    PostgresDSN string
    SupabaseURL string
    SupabaseKey string
    Debug       bool
}

// NewDatabase 根据环境与配置选择数据库实现
// 已移除本地文件数据库的支持
func NewDatabase(config DatabaseConfig) DatabaseInterface {
    // 是否在 Vercel 生产环境
    isVercelProduction := isVercelEnvironment()

    if isVercelProduction {
        fmt.Printf("🧭 Detected Vercel production environment\n")

        // Vercel 优先使用 Supabase（避免 IPv6）
        if config.SupabaseURL != "" && config.SupabaseKey != "" {
            fmt.Printf("🚀  Using Supabase REST API (Vercel optimized)\n")
            return NewSupabaseDatabase(config.SupabaseURL, config.SupabaseKey)
        }

        // 次选 PostgreSQL
        if config.PostgresDSN != "" {
            fmt.Printf("🌐  Using PostgreSQL in Vercel (may have IPv6 issues)\n")
            return NewPostgresDatabase(config.PostgresDSN)
        }

        // 未配置受支持的数据库，直接失败
        panic("No valid database configured for Vercel environment. Please set SUPABASE_URL+SUPABASE_SERVICE_KEY or POSTGRES_DSN")
    }

    // 非 Vercel 环境：PostgreSQL > Supabase
    if config.PostgresDSN != "" {
        fmt.Printf("🗄️  Using PostgreSQL database\n")
        return NewPostgresDatabase(config.PostgresDSN)
    }

    if config.SupabaseURL != "" && config.SupabaseKey != "" {
        fmt.Printf("🧰  Using Supabase REST API\n")
        return NewSupabaseDatabase(config.SupabaseURL, config.SupabaseKey)
    }

    panic("No valid database configuration found. Please configure POSTGRES_DSN or SUPABASE_URL+SUPABASE_SERVICE_KEY")
}

// isVercelEnvironment 内部检查 Vercel 环境
func isVercelEnvironment() bool {
    vercelEnv := os.Getenv("VERCEL_ENV")
    vercelURL := os.Getenv("VERCEL_URL")
    awsLambda := os.Getenv("AWS_LAMBDA_FUNCTION_NAME")
    return vercelEnv != "" || vercelURL != "" || awsLambda != ""
}

// NewDatabaseFromConfig 已弃用：避免误用本地数据库
func NewDatabaseFromConfig(cfg interface{}) DatabaseInterface {
    panic("NewDatabaseFromConfig is deprecated. Please construct DatabaseConfig with Postgres or Supabase settings.")
}
