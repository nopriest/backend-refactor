package database

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"tab-sync-backend-refactor/pkg/models"
)

// SupabaseDatabase Supabase数据库实现
type SupabaseDatabase struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewSupabaseDatabase 创建Supabase数据库实例
func NewSupabaseDatabase(url, key string) DatabaseInterface {
	// 确保URL格式正确
	if !strings.HasPrefix(url, "http") {
		url = "https://" + url
	}

	return &SupabaseDatabase{
		baseURL: url,
		apiKey:  key,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// makeRequest 发送HTTP请求到Supabase
func (db *SupabaseDatabase) makeRequest(method, endpoint string, body interface{}) ([]byte, error) {
	var reqBody io.Reader

	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := db.baseURL + "/rest/v1" + endpoint
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("apikey", db.apiKey)
	req.Header.Set("Authorization", "Bearer "+db.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	resp, err := db.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// makeRequestWithHeaders 发送HTTP请求到Supabase（支持自定义头）
func (db *SupabaseDatabase) makeRequestWithHeaders(method, endpoint string, body interface{}, customHeaders map[string]string) ([]byte, error) {
	var reqBody io.Reader

	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	url := db.baseURL + "/rest/v1" + endpoint
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置默认请求头
	req.Header.Set("apikey", db.apiKey)
	req.Header.Set("Authorization", "Bearer "+db.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Prefer", "return=representation")

	// 设置自定义请求头
	for key, value := range customHeaders {
		req.Header.Set(key, value)
	}

	resp, err := db.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// ================= Organizations & Spaces & Invitations =================

// Organizations
func (db *SupabaseDatabase) CreateOrganization(org *models.Organization) error {
    payload := map[string]interface{}{
        "name":        org.Name,
        "owner_id":    org.OwnerID,
        "description": org.Description,
        "avatar":      org.Avatar,
    }
    data, err := db.makeRequest("POST", "/organizations", payload)
    if err != nil { return err }
    var rows []map[string]interface{}
    if err := json.Unmarshal(data, &rows); err == nil && len(rows) > 0 {
        if id, ok := rows[0]["id"].(string); ok { org.ID = id }
    }
    // owner membership
    _, err = db.makeRequest("POST", "/organization_memberships", map[string]interface{}{
        "organization_id": org.ID,
        "user_id":         org.OwnerID,
        "role":            "owner",
    })
    return err
}

func (db *SupabaseDatabase) ListUserOrganizations(userID string) ([]models.Organization, error) {
    // filter by owner or membership; do two queries and merge
    ownedData, err := db.makeRequest("GET", "/organizations?owner_id=eq."+userID+"&select=*", nil)
    if err != nil { return nil, err }
    var owned []models.Organization
    _ = json.Unmarshal(ownedData, &owned)

    memData, err := db.makeRequest("GET", "/organization_memberships?user_id=eq."+userID+"&select=organization_id", nil)
    if err != nil { return owned, nil }
    var mems []map[string]string
    _ = json.Unmarshal(memData, &mems)
    orgIDs := map[string]bool{}
    for _, o := range owned { orgIDs[o.ID] = true }
    for _, m := range mems { if id, ok := m["organization_id"]; ok { orgIDs[id] = true } }
    // fetch orgs by ids
    var result []models.Organization
    for id := range orgIDs {
        data, err := db.makeRequest("GET", "/organizations?id=eq."+id+"&select=*", nil)
        if err == nil {
            var tmp []models.Organization
            if json.Unmarshal(data, &tmp) == nil && len(tmp) > 0 {
                result = append(result, tmp[0])
            }
        }
    }
    return result, nil
}

func (db *SupabaseDatabase) GetOrganization(orgID string) (*models.Organization, error) {
    data, err := db.makeRequest("GET", "/organizations?id=eq."+orgID+"&select=*", nil)
    if err != nil { return nil, err }
    var rows []models.Organization
    if err := json.Unmarshal(data, &rows); err != nil || len(rows) == 0 { return nil, fmt.Errorf("organization not found") }
    return &rows[0], nil
}

func (db *SupabaseDatabase) AddOrganizationMember(m *models.OrganizationMembership) error {
    payload := map[string]interface{}{
        "organization_id": m.OrganizationID,
        "user_id":         m.UserID,
        "role":            string(m.Role),
    }
    _, err := db.makeRequest("POST", "/organization_memberships", payload)
    return err
}

func (db *SupabaseDatabase) ListOrganizationMembers(orgID string) ([]models.OrganizationMembership, error) {
    data, err := db.makeRequest("GET", "/organization_memberships?organization_id=eq."+orgID+"&select=*", nil)
    if err != nil { return nil, err }
    var rows []models.OrganizationMembership
    if err := json.Unmarshal(data, &rows); err != nil { return nil, err }
    return rows, nil
}

// Spaces
func (db *SupabaseDatabase) CreateSpace(space *models.Space) error {
    payload := map[string]interface{}{
        "organization_id": space.OrganizationID,
        "name":            space.Name,
        "description":     space.Description,
        "is_default":      space.IsDefault,
    }
    data, err := db.makeRequest("POST", "/spaces", payload)
    if err != nil { return err }
    var rows []map[string]interface{}
    if err := json.Unmarshal(data, &rows); err == nil && len(rows) > 0 {
        if id, ok := rows[0]["id"].(string); ok { space.ID = id }
    }
    return nil
}

func (db *SupabaseDatabase) ListSpacesByOrganization(orgID string) ([]models.Space, error) {
    data, err := db.makeRequest("GET", "/spaces?organization_id=eq."+orgID+"&select=*", nil)
    if err != nil { return nil, err }
    var rows []models.Space
    if err := json.Unmarshal(data, &rows); err != nil { return nil, err }
    return rows, nil
}

func (db *SupabaseDatabase) UpdateSpace(space *models.Space) error {
    _, err := db.makeRequest("PATCH", "/spaces?id=eq."+space.ID, map[string]interface{}{
        "name":        space.Name,
        "description": space.Description,
        "is_default":  space.IsDefault,
    })
    return err
}

func (db *SupabaseDatabase) SetSpacePermission(spaceID, userID string, canEdit bool) error {
    // upsert-like: first try patch, if none affected then insert
    _, err := db.makeRequestWithHeaders("PATCH", "/space_permissions?space_id=eq."+spaceID+"&user_id=eq."+userID, map[string]interface{}{"can_edit": canEdit}, map[string]string{"Prefer": "return=representation"})
    if err != nil {
        _, err = db.makeRequest("POST", "/space_permissions", map[string]interface{}{
            "space_id": spaceID,
            "user_id":  userID,
            "can_edit": canEdit,
        })
    }
    return err
}

func (db *SupabaseDatabase) GetSpacePermissions(spaceID string) ([]models.SpacePermission, error) {
    data, err := db.makeRequest("GET", "/space_permissions?space_id=eq."+spaceID+"&select=*", nil)
    if err != nil { return nil, err }
    var rows []models.SpacePermission
    if err := json.Unmarshal(data, &rows); err != nil { return nil, err }
    return rows, nil
}

// Invitations
func (db *SupabaseDatabase) CreateInvitation(inv *models.OrganizationInvitation) error {
    payload := map[string]interface{}{
        "organization_id": inv.OrganizationID,
        "email":           inv.Email,
        "inviter_id":      inv.InviterID,
        "token":           inv.Token,
        "status":          string(inv.Status),
        "expires_at":      inv.ExpiresAt.Format(time.RFC3339),
    }
    data, err := db.makeRequest("POST", "/organization_invitations", payload)
    if err != nil { return err }
    var rows []map[string]interface{}
    if err := json.Unmarshal(data, &rows); err == nil && len(rows) > 0 {
        if id, ok := rows[0]["id"].(string); ok { inv.ID = id }
    }
    return nil
}

func (db *SupabaseDatabase) GetInvitationByToken(token string) (*models.OrganizationInvitation, error) {
    data, err := db.makeRequest("GET", "/organization_invitations?token=eq."+token+"&select=*", nil)
    if err != nil { return nil, err }
    var rows []models.OrganizationInvitation
    if err := json.Unmarshal(data, &rows); err != nil || len(rows) == 0 { return nil, fmt.Errorf("invitation not found") }
    return &rows[0], nil
}

func (db *SupabaseDatabase) ListInvitationsByEmail(email string) ([]models.OrganizationInvitation, error) {
    data, err := db.makeRequest("GET", "/organization_invitations?email=eq."+email+"&select=*", nil)
    if err != nil { return nil, err }
    var rows []models.OrganizationInvitation
    if err := json.Unmarshal(data, &rows); err != nil { return nil, err }
    return rows, nil
}

func (db *SupabaseDatabase) UpdateInvitation(inv *models.OrganizationInvitation) error {
    _, err := db.makeRequest("PATCH", "/organization_invitations?id=eq."+inv.ID, map[string]interface{}{
        "status":     string(inv.Status),
        "accepted_by": inv.AcceptedBy,
        "expires_at":  inv.ExpiresAt.Format(time.RFC3339),
    })
    return err
}
// CreateUser 创建用户
func (db *SupabaseDatabase) CreateUser(user *models.User) error {
	// 使用所有可用字段 - 不包含id字段，让PostgreSQL自动生成UUID
	userData := map[string]interface{}{
		"email":         user.Email,
		"password_hash": "", // OAuth用户没有密码，设为空字符串
		"name":          user.Name,
		"avatar":        user.Avatar,
		"provider":      user.Provider,
		"created_at":    user.CreatedAt.Format(time.RFC3339),
		"updated_at":    user.UpdatedAt.Format(time.RFC3339),
	}

	// 发送POST请求到users表
	data, err := db.makeRequest("POST", "/users", userData)
	if err != nil {
		return err
	}

	// 解析返回的数据以获取生成的ID
	if len(data) > 0 {
		var users []map[string]interface{}
		if err := json.Unmarshal(data, &users); err == nil && len(users) > 0 {
			if id, ok := users[0]["id"].(string); ok {
				user.ID = id // 设置生成的UUID
			}
		}
	}

	fmt.Printf("👤 Created OAuth user %s via Supabase REST (provider: %s)\n", user.Email, user.Provider)
	return nil
}

// GetUserByEmail 根据邮箱获取用户
func (db *SupabaseDatabase) GetUserByEmail(email string) (*models.User, error) {
	// 构建查询URL - 参考旧项目实现
	url := fmt.Sprintf("/users?email=eq.%s&select=*", email)

	// 发送GET请求
	data, err := db.makeRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// 解析响应 - 使用临时结构体处理字段映射
	var rawUsers []map[string]interface{}
	if err := json.Unmarshal(data, &rawUsers); err != nil {
		return nil, err
	}

	if len(rawUsers) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	// 转换为User结构体
	rawUser := rawUsers[0]
	user := &models.User{
		ID:       rawUser["id"].(string),
		Email:    rawUser["email"].(string),
		Password: rawUser["password_hash"].(string),
	}

	// 处理可选字段
	if name, ok := rawUser["name"].(string); ok {
		user.Name = name
	}
	if avatar, ok := rawUser["avatar"].(string); ok {
		user.Avatar = avatar
	}
	if provider, ok := rawUser["provider"].(string); ok {
		user.Provider = provider
	}

	// 处理时间字段
	if createdAt, ok := rawUser["created_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, createdAt); err == nil {
			user.CreatedAt = t
		}
	}
	if updatedAt, ok := rawUser["updated_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, updatedAt); err == nil {
			user.UpdatedAt = t
		}
	}

	return user, nil
}

// GetUserByID 根据ID获取用户
func (db *SupabaseDatabase) GetUserByID(id string) (*models.User, error) {
	endpoint := fmt.Sprintf("/users?id=eq.%s&select=*", id)

	data, err := db.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var users []models.User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	user := &users[0]
	return user, nil
}

// UpdateUser 更新用户
func (db *SupabaseDatabase) UpdateUser(user *models.User) error {
	// 更新所有可用字段
	userData := map[string]interface{}{
		"name":       user.Name,
		"avatar":     user.Avatar,
		"provider":   user.Provider,
		"tier":       user.Tier,
		"updated_at": user.UpdatedAt.Format(time.RFC3339),
	}

	endpoint := fmt.Sprintf("/users?id=eq.%s", user.ID)
	_, err := db.makeRequest("PATCH", endpoint, userData)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	fmt.Printf("👤 Updated user %s via Supabase REST (provider: %s, tier: %s)\n", user.Email, user.Provider, user.Tier)
	return nil
}

// DeleteUser 删除用户
func (db *SupabaseDatabase) DeleteUser(id string) error {
	// TODO: 实现Supabase用户删除
	return fmt.Errorf("DeleteUser not implemented for Supabase")
}

// GetUserWithSubscription 获取用户及订阅信息
func (db *SupabaseDatabase) GetUserWithSubscription(userID string) (*models.UserWithSubscription, error) {
	// 使用Supabase REST API查询用户信息
	endpoint := fmt.Sprintf("/users?id=eq.%s&select=*", userID)

	respBody, err := db.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// 解析响应
	var users []models.UserWithSubscription
	if err := json.Unmarshal(respBody, &users); err != nil {
		return nil, fmt.Errorf("failed to parse user response: %w", err)
	}

	if len(users) == 0 {
		return nil, fmt.Errorf("user not found")
	}

	user := &users[0]

	// 如果没有设置tier，默认为free
	if user.Tier == "" {
		user.Tier = models.TierFree
	}

	fmt.Printf("📋 GetUserWithSubscription (Supabase REST): user=%s, tier=%s\n", user.Email, user.Tier)
	return user, nil
}

// SaveSnapshot 保存快照
func (db *SupabaseDatabase) SaveSnapshot(userID, name string, tabGroups []models.TabGroup) error {
	// 计算统计信息
	groupCount := len(tabGroups)
	tabCount := 0
	for _, group := range tabGroups {
		tabCount += len(group.Tabs)
	}

	// 构建快照数据
	snapshot := map[string]interface{}{
		"user_id":     userID,
		"name":        name,
		"tab_groups":  tabGroups,
		"group_count": groupCount,
		"tab_count":   tabCount,
		"updated_at":  time.Now().Format(time.RFC3339),
	}

	// 检查快照是否已存在（参考旧项目实现）
	existingSnapshot, err := db.getSnapshotByName(userID, name)
	if err == nil && existingSnapshot != nil {
		// 更新现有快照
		fmt.Printf("📝 Updating existing snapshot: %s\n", name)
		endpoint := fmt.Sprintf("/snapshots?user_id=eq.%s&name=eq.%s", userID, name)
		_, err = db.makeRequest("PATCH", endpoint, snapshot)
		if err != nil {
			fmt.Printf("❌ Failed to update snapshot: %v\n", err)
			return fmt.Errorf("failed to update snapshot: %w", err)
		} else {
			fmt.Printf("✅ Successfully updated snapshot: %s\n", name)
		}
		return nil
	}

	// 创建新快照
	fmt.Printf("🆕 Creating new snapshot: %s\n", name)
	endpoint := "/snapshots"
	_, err = db.makeRequest("POST", endpoint, snapshot)
	if err != nil {
		fmt.Printf("❌ Failed to create snapshot: %v\n", err)
		return fmt.Errorf("failed to create snapshot: %w", err)
	} else {
		fmt.Printf("✅ Successfully created snapshot: %s\n", name)
	}

	fmt.Printf("💾 Saved snapshot '%s' for user %s (%d groups, %d tabs) via Supabase REST\n", name, userID, groupCount, tabCount)
	return nil
}

// ListSnapshots 列出快照
func (db *SupabaseDatabase) ListSnapshots(userID string) ([]SnapshotInfo, error) {
	// 使用Supabase REST API查询快照列表
	endpoint := fmt.Sprintf("/snapshots?user_id=eq.%s&select=name,created_at,updated_at,group_count,tab_count&order=updated_at.desc", userID)

	respBody, err := db.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query snapshots: %w", err)
	}

	// 解析响应
	var snapshots []struct {
		Name       string    `json:"name"`
		CreatedAt  time.Time `json:"created_at"`
		UpdatedAt  time.Time `json:"updated_at"`
		GroupCount int       `json:"group_count"`
		TabCount   int       `json:"tab_count"`
	}

	if err := json.Unmarshal(respBody, &snapshots); err != nil {
		return nil, fmt.Errorf("failed to parse snapshots response: %w", err)
	}

	// 转换为SnapshotInfo格式
	var result []SnapshotInfo
	for _, snapshot := range snapshots {
		result = append(result, SnapshotInfo{
			Name:       snapshot.Name,
			CreatedAt:  snapshot.CreatedAt.Format(time.RFC3339),
			UpdatedAt:  snapshot.UpdatedAt.Format(time.RFC3339),
			GroupCount: snapshot.GroupCount,
			TabCount:   snapshot.TabCount,
		})
	}

	return result, nil
}

// LoadSnapshot 加载快照
func (db *SupabaseDatabase) LoadSnapshot(userID, name string) (*LoadSnapshotResponse, error) {
	fmt.Printf("🔍 LoadSnapshot: Querying for userID=%s, name=%s\n", userID, name)

	// 使用Supabase REST API查询指定快照
	endpoint := fmt.Sprintf("/snapshots?user_id=eq.%s&name=eq.%s&select=name,tab_groups,created_at,updated_at", userID, name)
	fmt.Printf("🔍 LoadSnapshot: Query endpoint: %s\n", endpoint)

	respBody, err := db.makeRequest("GET", endpoint, nil)
	if err != nil {
		fmt.Printf("❌ LoadSnapshot: Request failed: %v\n", err)
		return nil, fmt.Errorf("failed to query snapshot: %w", err)
	}

	fmt.Printf("🔍 LoadSnapshot: Raw response: %s\n", string(respBody))

	// 解析响应 - Supabase返回的是数组格式
	var snapshots []struct {
		Name      string            `json:"name"`
		TabGroups []models.TabGroup `json:"tab_groups"`
		CreatedAt time.Time         `json:"created_at"`
		UpdatedAt time.Time         `json:"updated_at"`
	}

	if err := json.Unmarshal(respBody, &snapshots); err != nil {
		fmt.Printf("❌ LoadSnapshot: JSON parsing failed: %v\n", err)
		return nil, fmt.Errorf("failed to parse snapshot response: %w", err)
	}

	fmt.Printf("🔍 LoadSnapshot: Found %d snapshots\n", len(snapshots))

	// 检查是否找到快照
	if len(snapshots) == 0 {
		fmt.Printf("❌ LoadSnapshot: No snapshots found for userID=%s, name=%s\n", userID, name)
		return nil, fmt.Errorf("snapshot not found")
	}

	// 返回第一个匹配的快照
	snapshot := snapshots[0]
	fmt.Printf("✅ LoadSnapshot: Successfully loaded snapshot '%s' with %d tab groups\n", snapshot.Name, len(snapshot.TabGroups))

	return &LoadSnapshotResponse{
		Name:      snapshot.Name,
		TabGroups: snapshot.TabGroups,
		CreatedAt: snapshot.CreatedAt.Format(time.RFC3339),
		UpdatedAt: snapshot.UpdatedAt.Format(time.RFC3339),
	}, nil
}

// DeleteSnapshot 删除快照
func (db *SupabaseDatabase) DeleteSnapshot(userID, name string) error {
	// 使用Supabase REST API删除指定快照
	endpoint := fmt.Sprintf("/snapshots?user_id=eq.%s&name=eq.%s", userID, name)

	_, err := db.makeRequest("DELETE", endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	fmt.Printf("🗑️ Deleted snapshot '%s' for user %s\n", name, userID)
	return nil
}

// CreateSubscription 创建订阅
func (db *SupabaseDatabase) CreateSubscription(subscription *models.UserSubscription) error {
	// TODO: 实现Supabase订阅创建
	return fmt.Errorf("CreateSubscription not implemented for Supabase")
}

// GetUserSubscription 获取用户订阅
func (db *SupabaseDatabase) GetUserSubscription(userID string) (*models.UserSubscription, error) {
	// TODO: 实现Supabase订阅查询
	return nil, fmt.Errorf("GetUserSubscription not implemented for Supabase")
}

// UpdateSubscription 更新订阅
func (db *SupabaseDatabase) UpdateSubscription(subscription *models.UserSubscription) error {
	// TODO: 实现Supabase订阅更新
	return fmt.Errorf("UpdateSubscription not implemented for Supabase")
}

// CancelSubscription 取消订阅
func (db *SupabaseDatabase) CancelSubscription(userID string) error {
	// TODO: 实现Supabase订阅取消
	return fmt.Errorf("CancelSubscription not implemented for Supabase")
}

// GetUserAICredits 获取AI积分
func (db *SupabaseDatabase) GetUserAICredits(userID string) (*models.AICredits, error) {
	// TODO: 实现Supabase AI积分查询
	return nil, fmt.Errorf("GetUserAICredits not implemented for Supabase")
}

// UpdateAICredits 更新AI积分
func (db *SupabaseDatabase) UpdateAICredits(credits *models.AICredits) error {
	// TODO: 实现Supabase AI积分更新
	return fmt.Errorf("UpdateAICredits not implemented for Supabase")
}

// ConsumeAICredits 消费AI积分
func (db *SupabaseDatabase) ConsumeAICredits(userID string, amount int) error {
	// TODO: 实现Supabase AI积分消费
	return fmt.Errorf("ConsumeAICredits not implemented for Supabase")
}

// getSnapshotByName 根据名称获取快照（内部方法）
func (db *SupabaseDatabase) getSnapshotByName(userID, name string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/snapshots?user_id=eq.%s&name=eq.%s&select=*", userID, name)

	data, err := db.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	var snapshots []map[string]interface{}
	if err := json.Unmarshal(data, &snapshots); err != nil {
		return nil, err
	}

	if len(snapshots) == 0 {
		return nil, fmt.Errorf("snapshot not found")
	}

	return snapshots[0], nil
}

// HealthCheck 健康检查
func (db *SupabaseDatabase) HealthCheck() error {
	// 发送简单的查询来检查连接
	_, err := db.makeRequest("GET", "/", nil)
	return err
}

// Close 关闭连接
func (db *SupabaseDatabase) Close() error {
	// HTTP客户端无需显式关闭
	return nil
}
