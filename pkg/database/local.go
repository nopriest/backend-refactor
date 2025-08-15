package database

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"tab-sync-backend-refactor/pkg/models"

	"github.com/google/uuid"
)

// LocalDatabase 本地文件数据库实现
type LocalDatabase struct {
	dataDir string
}

// NewLocalDatabase 创建本地数据库实例
func NewLocalDatabase() DatabaseInterface {
	// 在Vercel等只读文件系统中，使用临时目录
	dataDir := "./data"

	// 尝试创建数据目录，如果失败则使用内存存储
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		fmt.Printf("Warning: Failed to create data directory: %v\n", err)
		// 在只读文件系统中，使用临时目录或内存存储
		dataDir = "/tmp/tabsync-data"
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			fmt.Printf("Warning: Failed to create temp data directory: %v\n", err)
			// 如果连临时目录都无法创建，使用当前目录（但不会实际写入文件）
			dataDir = "."
		}
	}

	return &LocalDatabase{
		dataDir: dataDir,
	}
}

// CreateUser 创建用户
func (db *LocalDatabase) CreateUser(user *models.User) error {
	if user.ID == "" {
		user.ID = uuid.New().String()
	}

	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	return db.saveUser(user)
}

// GetUserByEmail 根据邮箱获取用户
func (db *LocalDatabase) GetUserByEmail(email string) (*models.User, error) {
	users, err := db.loadAllUsers()
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		if user.Email == email {
			return &user, nil
		}
	}

	return nil, fmt.Errorf("user not found")
}

// GetUserByID 根据ID获取用户
func (db *LocalDatabase) GetUserByID(id string) (*models.User, error) {
	users, err := db.loadAllUsers()
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		if user.ID == id {
			return &user, nil
		}
	}

	return nil, fmt.Errorf("user not found")
}

// UpdateUser 更新用户
func (db *LocalDatabase) UpdateUser(user *models.User) error {
	user.UpdatedAt = time.Now()
	return db.saveUser(user)
}

// DeleteUser 删除用户
func (db *LocalDatabase) DeleteUser(id string) error {
	users, err := db.loadAllUsers()
	if err != nil {
		return err
	}

	var updatedUsers []models.User
	found := false

	for _, user := range users {
		if user.ID != id {
			updatedUsers = append(updatedUsers, user)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("user not found")
	}

	return db.saveAllUsers(updatedUsers)
}

// GetUserWithSubscription 获取用户及订阅信息
func (db *LocalDatabase) GetUserWithSubscription(userID string) (*models.UserWithSubscription, error) {
	user, err := db.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	// 本地数据库默认返回免费用户
	userWithSub := &models.UserWithSubscription{
		User: *user,
		Tier: models.TierFree,
	}

	return userWithSub, nil
}

// SaveSnapshot 保存快照
func (db *LocalDatabase) SaveSnapshot(userID, name string, tabGroups []models.TabGroup) error {
	snapshot := &models.Snapshot{
		ID:        uuid.New().String(),
		UserID:    userID,
		Name:      name,
		TabGroups: tabGroups,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 计算统计信息
	snapshot.GroupCount = len(tabGroups)
	for _, group := range tabGroups {
		snapshot.TabCount += len(group.Tabs)
	}

	return db.saveSnapshot(snapshot)
}

// ListSnapshots 列出用户的所有快照
func (db *LocalDatabase) ListSnapshots(userID string) ([]SnapshotInfo, error) {
	snapshots, err := db.loadUserSnapshots(userID)
	if err != nil {
		return nil, err
	}

	var infos []SnapshotInfo
	for _, snapshot := range snapshots {
		infos = append(infos, SnapshotInfo{
			Name:       snapshot.Name,
			CreatedAt:  snapshot.CreatedAt.Format(time.RFC3339),
			UpdatedAt:  snapshot.UpdatedAt.Format(time.RFC3339),
			TabCount:   snapshot.TabCount,
			GroupCount: snapshot.GroupCount,
		})
	}

	return infos, nil
}

// LoadSnapshot 加载快照
func (db *LocalDatabase) LoadSnapshot(userID, name string) (*LoadSnapshotResponse, error) {
	snapshots, err := db.loadUserSnapshots(userID)
	if err != nil {
		return nil, err
	}

	for _, snapshot := range snapshots {
		if snapshot.Name == name {
			return &LoadSnapshotResponse{
				Name:      snapshot.Name,
				TabGroups: snapshot.TabGroups,
				CreatedAt: snapshot.CreatedAt.Format(time.RFC3339),
				UpdatedAt: snapshot.UpdatedAt.Format(time.RFC3339),
			}, nil
		}
	}

	return nil, fmt.Errorf("snapshot not found")
}

// DeleteSnapshot 删除快照
func (db *LocalDatabase) DeleteSnapshot(userID, name string) error {
	snapshots, err := db.loadUserSnapshots(userID)
	if err != nil {
		return err
	}

	var updatedSnapshots []models.Snapshot
	found := false

	for _, snapshot := range snapshots {
		if snapshot.Name != name {
			updatedSnapshots = append(updatedSnapshots, snapshot)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("snapshot not found")
	}

	return db.saveUserSnapshots(userID, updatedSnapshots)
}

// CreateSubscription 创建订阅（本地实现为空）
func (db *LocalDatabase) CreateSubscription(subscription *models.UserSubscription) error {
	return fmt.Errorf("subscriptions not supported in local database")
}

// GetUserSubscription 获取用户订阅（本地实现为空）
func (db *LocalDatabase) GetUserSubscription(userID string) (*models.UserSubscription, error) {
	return nil, fmt.Errorf("subscriptions not supported in local database")
}

// UpdateSubscription 更新订阅（本地实现为空）
func (db *LocalDatabase) UpdateSubscription(subscription *models.UserSubscription) error {
	return fmt.Errorf("subscriptions not supported in local database")
}

// CancelSubscription 取消订阅（本地实现为空）
func (db *LocalDatabase) CancelSubscription(userID string) error {
	return fmt.Errorf("subscriptions not supported in local database")
}

// GetUserAICredits 获取AI积分（本地实现返回默认值）
func (db *LocalDatabase) GetUserAICredits(userID string) (*models.AICredits, error) {
	return &models.AICredits{
		ID:               uuid.New().String(),
		UserID:           userID,
		CreditsTotal:     100,
		CreditsUsed:      0,
		CreditsRemaining: 100,
		PeriodStart:      time.Now().Truncate(24 * time.Hour),
		PeriodEnd:        time.Now().AddDate(0, 1, 0).Truncate(24 * time.Hour),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}, nil
}

// UpdateAICredits 更新AI积分（本地实现为空）
func (db *LocalDatabase) UpdateAICredits(credits *models.AICredits) error {
	return fmt.Errorf("AI credits not supported in local database")
}

// ConsumeAICredits 消费AI积分（本地实现为空）
func (db *LocalDatabase) ConsumeAICredits(userID string, amount int) error {
	return fmt.Errorf("AI credits not supported in local database")
}

// HealthCheck 健康检查
func (db *LocalDatabase) HealthCheck() error {
	// 检查数据目录是否可访问
	if _, err := os.Stat(db.dataDir); os.IsNotExist(err) {
		return fmt.Errorf("data directory does not exist: %s", db.dataDir)
	}
	return nil
}

// Close 关闭连接（本地数据库无需关闭）
func (db *LocalDatabase) Close() error {
	return nil
}

// 私有辅助方法

func (db *LocalDatabase) getUsersFilePath() string {
	return filepath.Join(db.dataDir, "users.json")
}

func (db *LocalDatabase) getSnapshotsFilePath(userID string) string {
	return filepath.Join(db.dataDir, fmt.Sprintf("snapshots_%s.json", userID))
}

func (db *LocalDatabase) saveUser(user *models.User) error {
	users, err := db.loadAllUsers()
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// 更新或添加用户
	found := false
	for i, u := range users {
		if u.ID == user.ID {
			users[i] = *user
			found = true
			break
		}
	}

	if !found {
		users = append(users, *user)
	}

	return db.saveAllUsers(users)
}

func (db *LocalDatabase) loadAllUsers() ([]models.User, error) {
	filePath := db.getUsersFilePath()

	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return []models.User{}, nil
	}
	if err != nil {
		return nil, err
	}

	var users []models.User
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, err
	}

	return users, nil
}

func (db *LocalDatabase) saveAllUsers(users []models.User) error {
	filePath := db.getUsersFilePath()

	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

func (db *LocalDatabase) saveSnapshot(snapshot *models.Snapshot) error {
	snapshots, err := db.loadUserSnapshots(snapshot.UserID)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// 更新或添加快照
	found := false
	for i, s := range snapshots {
		if s.Name == snapshot.Name {
			snapshots[i] = *snapshot
			found = true
			break
		}
	}

	if !found {
		snapshots = append(snapshots, *snapshot)
	}

	return db.saveUserSnapshots(snapshot.UserID, snapshots)
}

func (db *LocalDatabase) loadUserSnapshots(userID string) ([]models.Snapshot, error) {
	filePath := db.getSnapshotsFilePath(userID)

	data, err := os.ReadFile(filePath)
	if os.IsNotExist(err) {
		return []models.Snapshot{}, nil
	}
	if err != nil {
		return nil, err
	}

	var snapshots []models.Snapshot
	if err := json.Unmarshal(data, &snapshots); err != nil {
		return nil, err
	}

	return snapshots, nil
}

func (db *LocalDatabase) saveUserSnapshots(userID string, snapshots []models.Snapshot) error {
	filePath := db.getSnapshotsFilePath(userID)

	data, err := json.MarshalIndent(snapshots, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}
