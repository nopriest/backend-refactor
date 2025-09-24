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
	// Sanitize DSN to avoid stray CR/LF from env values
	dsn = strings.TrimSpace(dsn)
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
    if user.Provider == "" {
        user.Provider = "email"
    }
    // Debug: 打印当前数据库/Schema 和 public.users 列，确认运行时连接与结构
    {
        var dbName, currSchema, searchPath string
        _ = db.db.QueryRow("SELECT current_database(), current_schema(), array_to_string(current_schemas(true), ',')").Scan(&dbName, &currSchema, &searchPath)
        fmt.Printf("DEBUG[PG] current_database=%s, current_schema=%s, search_path=%s\n", dbName, currSchema, searchPath)
        if rows, err := db.db.Query("SELECT column_name, data_type FROM information_schema.columns WHERE table_schema='public' AND table_name='users' ORDER BY ordinal_position"); err == nil {
            defer rows.Close()
            cols := []string{}
            for rows.Next() {
                var cn, dt string
                if err := rows.Scan(&cn, &dt); err == nil {
                    cols = append(cols, fmt.Sprintf("%s(%s)", cn, dt))
                }
            }
            fmt.Printf("DEBUG[PG] public.users columns: %s\n", strings.Join(cols, ", "))
        }
    }
    query := `
        INSERT INTO public.users (email, password_hash, name, avatar, provider, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
        RETURNING id, created_at, updated_at
    `
    var createdAt, updatedAt time.Time
    err := db.db.QueryRow(query, user.Email, user.Password, user.Name, user.Avatar, user.Provider).
        Scan(&user.ID, &createdAt, &updatedAt)
    if err != nil {
        return fmt.Errorf("failed to create user: %w", err)
    }
    user.CreatedAt = createdAt
    user.UpdatedAt = updatedAt
    return nil
}

// GetUserByEmail 根据邮箱获取用户
func (db *PostgresDatabase) GetUserByEmail(email string) (*models.User, error) {
    query := `
        SELECT id, email, COALESCE(name,''), COALESCE(avatar,''), COALESCE(provider,'email'),
               COALESCE(password_hash,''), created_at, updated_at
        FROM public.users
        WHERE email = $1
    `
    var u models.User
    var createdAt, updatedAt time.Time
    err := db.db.QueryRow(query, email).Scan(
        &u.ID, &u.Email, &u.Name, &u.Avatar, &u.Provider, &u.Password, &createdAt, &updatedAt,
    )
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, fmt.Errorf("user not found")
        }
        return nil, fmt.Errorf("failed to get user by email: %w", err)
    }
    u.CreatedAt = createdAt
    u.UpdatedAt = updatedAt
    return &u, nil
}

// GetUserByID 根据ID获取用户
func (db *PostgresDatabase) GetUserByID(id string) (*models.User, error) {
    query := `
        SELECT id, email, created_at, updated_at
        FROM public.users
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
    if user.ID == "" {
        return fmt.Errorf("user ID is required for update")
    }
    query := `
        UPDATE public.users
        SET name = $1,
            avatar = $2,
            provider = COALESCE($3, provider),
            updated_at = NOW()
        WHERE id = $4
    `
    _, err := db.db.Exec(query, user.Name, user.Avatar, user.Provider, user.ID)
    if err != nil {
        return fmt.Errorf("failed to update user: %w", err)
    }
    return nil
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
        FROM public.users u
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

// ================= Organizations & Spaces & Invitations =================

// Organizations
func (db *PostgresDatabase) CreateOrganization(org *models.Organization) error {
    query := `
        INSERT INTO organizations (name, owner_id, description, avatar, created_at, updated_at)
        VALUES ($1, $2, $3, $4, NOW(), NOW())
        RETURNING id, created_at, updated_at
    `
    err := db.db.QueryRow(query, org.Name, org.OwnerID, org.Description, org.Avatar).
        Scan(&org.ID, &org.CreatedAt, &org.UpdatedAt)
    if err != nil {
        return fmt.Errorf("failed to create organization: %w", err)
    }
    // owner membership
    _, err = db.db.Exec(`
        INSERT INTO organization_memberships (organization_id, user_id, role, created_at)
        VALUES ($1, $2, 'owner', NOW())
        ON CONFLICT (organization_id, user_id) DO NOTHING
    `, org.ID, org.OwnerID)
    if err != nil {
        return fmt.Errorf("failed to add owner membership: %w", err)
    }
    return nil
}

func (db *PostgresDatabase) ListUserOrganizations(userID string) ([]models.Organization, error) {
    query := `
        SELECT DISTINCT o.id, o.name, o.owner_id, o.description, o.avatar, o.created_at, o.updated_at
        FROM organizations o
        LEFT JOIN organization_memberships m ON m.organization_id = o.id
        WHERE o.owner_id = $1 OR m.user_id = $1
        ORDER BY o.created_at DESC
    `
    rows, err := db.db.Query(query, userID)
    if err != nil {
        return nil, fmt.Errorf("failed to list organizations: %w", err)
    }
    defer rows.Close()
    var result []models.Organization
    for rows.Next() {
        var o models.Organization
        if err := rows.Scan(&o.ID, &o.Name, &o.OwnerID, &o.Description, &o.Avatar, &o.CreatedAt, &o.UpdatedAt); err != nil {
            return nil, err
        }
        result = append(result, o)
    }
    return result, nil
}

func (db *PostgresDatabase) GetOrganization(orgID string) (*models.Organization, error) {
    query := `SELECT id, name, owner_id, description, avatar, created_at, updated_at FROM organizations WHERE id = $1`
    var o models.Organization
    err := db.db.QueryRow(query, orgID).Scan(&o.ID, &o.Name, &o.OwnerID, &o.Description, &o.Avatar, &o.CreatedAt, &o.UpdatedAt)
    if err != nil {
        if err == sql.ErrNoRows {
            return nil, fmt.Errorf("organization not found")
        }
        return nil, fmt.Errorf("failed to get organization: %w", err)
    }
    return &o, nil
}

func (db *PostgresDatabase) AddOrganizationMember(m *models.OrganizationMembership) error {
    query := `
        INSERT INTO organization_memberships (organization_id, user_id, role, created_at)
        VALUES ($1, $2, $3, NOW())
        ON CONFLICT (organization_id, user_id) DO UPDATE SET role = EXCLUDED.role
        RETURNING id
    `
    return db.db.QueryRow(query, m.OrganizationID, m.UserID, string(m.Role)).Scan(&m.ID)
}

func (db *PostgresDatabase) ListOrganizationMembers(orgID string) ([]models.OrganizationMembership, error) {
    query := `
        SELECT id, organization_id, user_id, role, created_at
        FROM organization_memberships
        WHERE organization_id = $1
        ORDER BY created_at ASC
    `
    rows, err := db.db.Query(query, orgID)
    if err != nil {
        return nil, fmt.Errorf("failed to list members: %w", err)
    }
    defer rows.Close()
    var result []models.OrganizationMembership
    for rows.Next() {
        var m models.OrganizationMembership
        var role string
        if err := rows.Scan(&m.ID, &m.OrganizationID, &m.UserID, &role, &m.CreatedAt); err != nil {
            return nil, err
        }
        m.Role = models.OrgMemberRole(role)
        result = append(result, m)
    }
    return result, nil
}

// Spaces
func (db *PostgresDatabase) CreateSpace(space *models.Space) error {
    query := `
        INSERT INTO spaces (organization_id, name, description, is_default, created_at, updated_at)
        VALUES ($1, $2, $3, $4, NOW(), NOW())
        RETURNING id, created_at, updated_at
    `
    return db.db.QueryRow(query, space.OrganizationID, space.Name, space.Description, space.IsDefault).
        Scan(&space.ID, &space.CreatedAt, &space.UpdatedAt)
}

func (db *PostgresDatabase) ListSpacesByOrganization(orgID string) ([]models.Space, error) {
    rows, err := db.db.Query(`SELECT id, organization_id, name, description, is_default, created_at, updated_at FROM spaces WHERE organization_id = $1 ORDER BY created_at ASC`, orgID)
    if err != nil {
        return nil, fmt.Errorf("failed to list spaces: %w", err)
    }
    defer rows.Close()
    var result []models.Space
    for rows.Next() {
        var s models.Space
        if err := rows.Scan(&s.ID, &s.OrganizationID, &s.Name, &s.Description, &s.IsDefault, &s.CreatedAt, &s.UpdatedAt); err != nil {
            return nil, err
        }
        result = append(result, s)
    }
    return result, nil
}

func (db *PostgresDatabase) UpdateSpace(space *models.Space) error {
    _, err := db.db.Exec(`UPDATE spaces SET name=$1, description=$2, is_default=$3, updated_at=NOW() WHERE id=$4`, space.Name, space.Description, space.IsDefault, space.ID)
    return err
}

func (db *PostgresDatabase) SetSpacePermission(spaceID, userID string, canEdit bool) error {
    _, err := db.db.Exec(`
        INSERT INTO space_permissions (space_id, user_id, can_edit, created_at, updated_at)
        VALUES ($1, $2, $3, NOW(), NOW())
        ON CONFLICT (space_id, user_id) DO UPDATE SET can_edit = EXCLUDED.can_edit, updated_at = NOW()
    `, spaceID, userID, canEdit)
    return err
}

func (db *PostgresDatabase) GetSpacePermissions(spaceID string) ([]models.SpacePermission, error) {
    rows, err := db.db.Query(`SELECT id, space_id, user_id, can_edit, created_at, updated_at FROM space_permissions WHERE space_id=$1`, spaceID)
    if err != nil {
        return nil, fmt.Errorf("failed to get space permissions: %w", err)
    }
    defer rows.Close()
    var result []models.SpacePermission
    for rows.Next() {
        var p models.SpacePermission
        if err := rows.Scan(&p.ID, &p.SpaceID, &p.UserID, &p.CanEdit, &p.CreatedAt, &p.UpdatedAt); err != nil {
            return nil, err
        }
        result = append(result, p)
    }
    return result, nil
}

// Invitations
func (db *PostgresDatabase) CreateInvitation(inv *models.OrganizationInvitation) error {
    query := `
        INSERT INTO organization_invitations (organization_id, email, inviter_id, token, status, expires_at, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, NOW(), NOW())
        RETURNING id, created_at, updated_at
    `
    return db.db.QueryRow(query, inv.OrganizationID, inv.Email, inv.InviterID, inv.Token, string(inv.Status), inv.ExpiresAt).
        Scan(&inv.ID, &inv.CreatedAt, &inv.UpdatedAt)
}

func (db *PostgresDatabase) GetInvitationByToken(token string) (*models.OrganizationInvitation, error) {
    var inv models.OrganizationInvitation
    var status string
    err := db.db.QueryRow(`
        SELECT id, organization_id, email, inviter_id, token, status, expires_at, accepted_by, created_at, updated_at
        FROM organization_invitations WHERE token = $1
    `, token).Scan(&inv.ID, &inv.OrganizationID, &inv.Email, &inv.InviterID, &inv.Token, &status, &inv.ExpiresAt, &inv.AcceptedBy, &inv.CreatedAt, &inv.UpdatedAt)
    if err != nil {
        if err == sql.ErrNoRows { return nil, fmt.Errorf("invitation not found") }
        return nil, fmt.Errorf("failed to get invitation: %w", err)
    }
    inv.Status = models.InvitationStatus(status)
    return &inv, nil
}

func (db *PostgresDatabase) ListInvitationsByEmail(email string) ([]models.OrganizationInvitation, error) {
    rows, err := db.db.Query(`
        SELECT id, organization_id, email, inviter_id, token, status, expires_at, accepted_by, created_at, updated_at
        FROM organization_invitations WHERE email = $1 ORDER BY created_at DESC
    `, email)
    if err != nil {
        return nil, fmt.Errorf("failed to list invitations: %w", err)
    }
    defer rows.Close()
    var list []models.OrganizationInvitation
    for rows.Next() {
        var inv models.OrganizationInvitation
        var status string
        if err := rows.Scan(&inv.ID, &inv.OrganizationID, &inv.Email, &inv.InviterID, &inv.Token, &status, &inv.ExpiresAt, &inv.AcceptedBy, &inv.CreatedAt, &inv.UpdatedAt); err != nil {
            return nil, err
        }
        inv.Status = models.InvitationStatus(status)
        list = append(list, inv)
    }
    return list, nil
}

func (db *PostgresDatabase) UpdateInvitation(inv *models.OrganizationInvitation) error {
    _, err := db.db.Exec(`
        UPDATE organization_invitations SET status=$1, accepted_by=$2, expires_at=$3, updated_at=NOW() WHERE id=$4
    `, string(inv.Status), inv.AcceptedBy, inv.ExpiresAt, inv.ID)
    return err
}
