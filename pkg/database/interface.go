package database

import (
	"fmt"
	"os"
	"tab-sync-backend-refactor/pkg/models"
)

// DatabaseInterface 定义数据库操作接口
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
    ListUserOrganizations(userID string) ([]models.Organization, error)
    GetOrganization(orgID string) (*models.Organization, error)
    AddOrganizationMember(m *models.OrganizationMembership) error
    ListOrganizationMembers(orgID string) ([]models.OrganizationMembership, error)

    // Spaces
    CreateSpace(space *models.Space) error
    ListSpacesByOrganization(orgID string) ([]models.Space, error)
    UpdateSpace(space *models.Space) error
    SetSpacePermission(spaceID, userID string, canEdit bool) error
    GetSpacePermissions(spaceID string) ([]models.SpacePermission, error)

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

	// AI积分管理
	GetUserAICredits(userID string) (*models.AICredits, error)
	UpdateAICredits(credits *models.AICredits) error
	ConsumeAICredits(userID string, amount int) error

	// 健康检查
	HealthCheck() error

	// 关闭连接
	Close() error
}

// SnapshotInfo 快照信息结构（为了向后兼容）
type SnapshotInfo struct {
	Name       string `json:"name"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	TabCount   int    `json:"tab_count"`
	GroupCount int    `json:"group_count"`
}

// LoadSnapshotResponse 加载快照响应结构（为了向后兼容）
type LoadSnapshotResponse struct {
	Name      string            `json:"name"`
	TabGroups []models.TabGroup `json:"tabGroups"`
	CreatedAt string            `json:"createdAt"`
	UpdatedAt string            `json:"updatedAt"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	UseLocalDB  bool
	PostgresDSN string
	SupabaseURL string
	SupabaseKey string
	Debug       bool
}

// NewDatabase 创建数据库实例
// 根据配置和环境自动选择合适的数据库实现
func NewDatabase(config DatabaseConfig) DatabaseInterface {
	// 检查是否在Vercel生产环境
	isVercelProduction := isVercelEnvironment()

	if isVercelProduction {
		fmt.Printf("🌐 Detected Vercel production environment\n")

		// 在Vercel环境中，优先使用Supabase REST API避免IPv6问题
		if config.SupabaseURL != "" && config.SupabaseKey != "" {
			fmt.Printf("🗄️  Using Supabase REST API (Vercel optimized)\n")
			return NewSupabaseDatabase(config.SupabaseURL, config.SupabaseKey)
		}

		// 如果没有Supabase配置，尝试PostgreSQL（可能会有IPv6问题）
		if config.PostgresDSN != "" {
			fmt.Printf("⚠️  Using PostgreSQL in Vercel (may have IPv6 issues)\n")
			return NewPostgresDatabase(config.PostgresDSN)
		}

		// 最后回退到本地文件数据库（在Vercel中使用临时存储）
		fmt.Printf("🗄️  Using temporary file database in Vercel\n")
		return NewLocalDatabase()
	}

	// 非Vercel环境：优先级 PostgreSQL > Supabase > 本地文件数据库

	// 1. 如果配置了PostgreSQL，优先使用
	if config.PostgresDSN != "" {
		fmt.Printf("🗄️  Using PostgreSQL database\n")
		return NewPostgresDatabase(config.PostgresDSN)
	}

	// 2. 如果配置了Supabase，使用Supabase
	if config.SupabaseURL != "" && config.SupabaseKey != "" {
		fmt.Printf("🗄️  Using Supabase REST API\n")
		return NewSupabaseDatabase(config.SupabaseURL, config.SupabaseKey)
	}

	// 3. 如果明确设置使用本地数据库，或者没有其他配置
	if config.UseLocalDB || (config.PostgresDSN == "" && config.SupabaseURL == "") {
		fmt.Printf("🗄️  Using local file database\n")
		return NewLocalDatabase()
	}

	// 4. 默认回退到本地文件数据库
	fmt.Printf("⚠️  No valid database configuration found, falling back to local file database\n")
	return NewLocalDatabase()
}

// isVercelEnvironment 检查是否在Vercel环境中运行
func isVercelEnvironment() bool {
	// Vercel设置的环境变量
	vercelEnv := os.Getenv("VERCEL_ENV")
	vercelURL := os.Getenv("VERCEL_URL")

	// 检查AWS Lambda环境变量（Vercel使用AWS Lambda）
	awsLambda := os.Getenv("AWS_LAMBDA_FUNCTION_NAME")

	return vercelEnv != "" || vercelURL != "" || awsLambda != ""
}

// NewDatabaseFromConfig 从配置对象创建数据库实例
func NewDatabaseFromConfig(cfg interface{}) DatabaseInterface {
	// 这里需要根据实际的配置结构进行类型断言
	// 为了简化，我们先使用默认配置
	return NewDatabase(DatabaseConfig{
		UseLocalDB: true,
	})
}
