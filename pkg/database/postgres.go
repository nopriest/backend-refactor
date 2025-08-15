package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"tab-sync-backend-refactor/pkg/models"

	_ "github.com/lib/pq"
)

// PostgresDatabase PostgreSQL数据库实现
type PostgresDatabase struct {
	db *sql.DB
}

// NewPostgresDatabase 创建PostgreSQL数据库实例
func NewPostgresDatabase(dsn string) DatabaseInterface {
	// 尝试多种连接策略来解决Vercel Lambda的IPv6问题
	strategies := []string{
		addConnectionParams(dsn, "prefer_simple_protocol=true"),
		addConnectionParams(dsn, "prefer_simple_protocol=true&connect_timeout=10"),
		addConnectionParams(dsn, "sslmode=require&prefer_simple_protocol=true"),
		dsn, // 最后尝试原始DSN
	}

	var db *sql.DB
	var err error

	for i, strategy := range strategies {
		fmt.Printf("🔄 Trying connection strategy %d...\n", i+1)

		db, err = sql.Open("postgres", strategy)
		if err != nil {
			fmt.Printf("❌ Strategy %d failed to open: %v\n", i+1, err)
			continue
		}

		// 设置连接池参数，适合无服务器环境
		db.SetMaxOpenConns(5)                  // 限制最大连接数
		db.SetMaxIdleConns(2)                  // 限制空闲连接数
		db.SetConnMaxLifetime(5 * time.Minute) // 连接最大生命周期

		// 测试连接
		if err = db.Ping(); err != nil {
			fmt.Printf("❌ Strategy %d failed to ping: %v\n", i+1, err)
			db.Close()
			continue
		}

		fmt.Printf("✅ PostgreSQL connection established successfully with strategy %d\n", i+1)
		return &PostgresDatabase{db: db}
	}

	// 所有策略都失败了
	panic(fmt.Sprintf("Failed to connect to PostgreSQL with all strategies. Last error: %v", err))
}

// addConnectionParams 添加连接参数到DSN
func addConnectionParams(dsn, params string) string {
	if params == "" {
		return dsn
	}

	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}

	return dsn + separator + params
}

// CreateUser 创建用户
func (db *PostgresDatabase) CreateUser(user *models.User) error {
	// TODO: 实现PostgreSQL用户创建
	return fmt.Errorf("CreateUser not implemented for PostgreSQL")
}

// GetUserByEmail 根据邮箱获取用户
func (db *PostgresDatabase) GetUserByEmail(email string) (*models.User, error) {
	// TODO: 实现PostgreSQL用户查询
	return nil, fmt.Errorf("GetUserByEmail not implemented for PostgreSQL")
}

// GetUserByID 根据ID获取用户
func (db *PostgresDatabase) GetUserByID(id string) (*models.User, error) {
	query := `
		SELECT id, email, created_at, updated_at
		FROM users
		WHERE id = $1
	`

	var user models.User
	err := db.db.QueryRow(query, id).Scan(
		&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// 设置默认值
	user.Provider = "email"

	return &user, nil
}

// UpdateUser 更新用户
func (db *PostgresDatabase) UpdateUser(user *models.User) error {
	// TODO: 实现PostgreSQL用户更新
	return fmt.Errorf("UpdateUser not implemented for PostgreSQL")
}

// DeleteUser 删除用户
func (db *PostgresDatabase) DeleteUser(id string) error {
	// TODO: 实现PostgreSQL用户删除
	return fmt.Errorf("DeleteUser not implemented for PostgreSQL")
}

// GetUserWithSubscription 获取用户及订阅信息
func (db *PostgresDatabase) GetUserWithSubscription(userID string) (*models.UserWithSubscription, error) {
	// 查询用户及其订阅信息（匹配现有数据库结构）
	query := `
		SELECT
			u.id, u.email, u.created_at, u.updated_at,
			COALESCE(u.tier::text, 'free') as tier,
			u.paddle_customer_id,
			u.trial_ends_at,
			COALESCE(u.is_lifetime_member, false) as is_lifetime_member,
			u.lifetime_member_type
		FROM users u
		WHERE u.id = $1
	`

	var userWithSub models.UserWithSubscription
	var tierStr string

	err := db.db.QueryRow(query, userID).Scan(
		&userWithSub.ID, &userWithSub.Email, &userWithSub.CreatedAt, &userWithSub.UpdatedAt,
		&tierStr, &userWithSub.PaddleCustomerID, &userWithSub.TrialEndsAt,
		&userWithSub.IsLifetimeMember, &userWithSub.LifetimeMemberType,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to get user with subscription: %w", err)
	}

	// 设置默认值
	userWithSub.Provider = "email"

	// 转换tier
	switch tierStr {
	case "pro":
		userWithSub.Tier = models.TierPro
	case "power":
		userWithSub.Tier = models.TierPower
	default:
		userWithSub.Tier = models.TierFree
	}

	fmt.Printf("📋 GetUserWithSubscription (PostgreSQL): user=%s, tier=%s\n", userWithSub.Email, userWithSub.Tier)
	return &userWithSub, nil
}

// SaveSnapshot 保存快照
func (db *PostgresDatabase) SaveSnapshot(userID, name string, tabGroups []models.TabGroup) error {
	// 计算统计信息
	groupCount := len(tabGroups)
	tabCount := 0
	for _, group := range tabGroups {
		tabCount += len(group.Tabs)
	}

	// 将tabGroups转换为JSON
	tabGroupsJSON, err := json.Marshal(tabGroups)
	if err != nil {
		return fmt.Errorf("failed to marshal tab groups: %w", err)
	}

	// 使用UPSERT语句（INSERT ... ON CONFLICT）
	query := `
		INSERT INTO snapshots (user_id, name, tab_groups, group_count, tab_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
		ON CONFLICT (user_id, name)
		DO UPDATE SET
			tab_groups = EXCLUDED.tab_groups,
			group_count = EXCLUDED.group_count,
			tab_count = EXCLUDED.tab_count,
			updated_at = NOW()
	`

	_, err = db.db.Exec(query, userID, name, tabGroupsJSON, groupCount, tabCount)
	if err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	fmt.Printf("💾 Saved snapshot '%s' for user %s (%d groups, %d tabs)\n", name, userID, groupCount, tabCount)
	return nil
}

// ListSnapshots 列出快照
func (db *PostgresDatabase) ListSnapshots(userID string) ([]SnapshotInfo, error) {
	query := `
		SELECT name, created_at, updated_at, group_count, tab_count
		FROM snapshots
		WHERE user_id = $1
		ORDER BY updated_at DESC
	`

	rows, err := db.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}
	defer rows.Close()

	var snapshots []SnapshotInfo
	for rows.Next() {
		var snapshot SnapshotInfo
		var createdAt, updatedAt time.Time

		err := rows.Scan(
			&snapshot.Name, &createdAt, &updatedAt,
			&snapshot.GroupCount, &snapshot.TabCount,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan snapshot: %w", err)
		}

		snapshot.CreatedAt = createdAt.Format(time.RFC3339)
		snapshot.UpdatedAt = updatedAt.Format(time.RFC3339)
		snapshots = append(snapshots, snapshot)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating snapshots: %w", err)
	}

	return snapshots, nil
}

// LoadSnapshot 加载快照
func (db *PostgresDatabase) LoadSnapshot(userID, name string) (*LoadSnapshotResponse, error) {
	query := `
		SELECT name, tab_groups, created_at, updated_at
		FROM snapshots
		WHERE user_id = $1 AND name = $2
	`

	var response LoadSnapshotResponse
	var tabGroupsJSON []byte
	var createdAt, updatedAt time.Time

	err := db.db.QueryRow(query, userID, name).Scan(
		&response.Name, &tabGroupsJSON, &createdAt, &updatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("snapshot not found")
		}
		return nil, fmt.Errorf("failed to load snapshot: %w", err)
	}

	// 解析JSON
	err = json.Unmarshal(tabGroupsJSON, &response.TabGroups)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal tab groups: %w", err)
	}

	response.CreatedAt = createdAt.Format(time.RFC3339)
	response.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &response, nil
}

// DeleteSnapshot 删除快照
func (db *PostgresDatabase) DeleteSnapshot(userID, name string) error {
	query := `DELETE FROM snapshots WHERE user_id = $1 AND name = $2`

	result, err := db.db.Exec(query, userID, name)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("snapshot not found")
	}

	fmt.Printf("🗑️ Deleted snapshot '%s' for user %s\n", name, userID)
	return nil
}

// CreateSubscription 创建订阅
func (db *PostgresDatabase) CreateSubscription(subscription *models.UserSubscription) error {
	// TODO: 实现PostgreSQL订阅创建
	return fmt.Errorf("CreateSubscription not implemented for PostgreSQL")
}

// GetUserSubscription 获取用户订阅
func (db *PostgresDatabase) GetUserSubscription(userID string) (*models.UserSubscription, error) {
	// TODO: 实现PostgreSQL订阅查询
	return nil, fmt.Errorf("GetUserSubscription not implemented for PostgreSQL")
}

// UpdateSubscription 更新订阅
func (db *PostgresDatabase) UpdateSubscription(subscription *models.UserSubscription) error {
	// TODO: 实现PostgreSQL订阅更新
	return fmt.Errorf("UpdateSubscription not implemented for PostgreSQL")
}

// CancelSubscription 取消订阅
func (db *PostgresDatabase) CancelSubscription(userID string) error {
	// TODO: 实现PostgreSQL订阅取消
	return fmt.Errorf("CancelSubscription not implemented for PostgreSQL")
}

// GetUserAICredits 获取AI积分
func (db *PostgresDatabase) GetUserAICredits(userID string) (*models.AICredits, error) {
	// TODO: 实现PostgreSQL AI积分查询
	return nil, fmt.Errorf("GetUserAICredits not implemented for PostgreSQL")
}

// UpdateAICredits 更新AI积分
func (db *PostgresDatabase) UpdateAICredits(credits *models.AICredits) error {
	// TODO: 实现PostgreSQL AI积分更新
	return fmt.Errorf("UpdateAICredits not implemented for PostgreSQL")
}

// ConsumeAICredits 消费AI积分
func (db *PostgresDatabase) ConsumeAICredits(userID string, amount int) error {
	// TODO: 实现PostgreSQL AI积分消费
	return fmt.Errorf("ConsumeAICredits not implemented for PostgreSQL")
}

// HealthCheck 健康检查
func (db *PostgresDatabase) HealthCheck() error {
	return db.db.Ping()
}

// Close 关闭连接
func (db *PostgresDatabase) Close() error {
	return db.db.Close()
}
