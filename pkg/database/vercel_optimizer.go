package database

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// VercelOptimizer Vercel环境优化器
type VercelOptimizer struct {
	connections map[string]DatabaseInterface
	lastUsed    map[string]time.Time
	mu          sync.RWMutex
}

var (
	vercelOptimizer *VercelOptimizer
	optimizerOnce   sync.Once
)

// GetVercelOptimizer 获取Vercel优化器单例
func GetVercelOptimizer() *VercelOptimizer {
	optimizerOnce.Do(func() {
		vercelOptimizer = &VercelOptimizer{
			connections: make(map[string]DatabaseInterface),
			lastUsed:    make(map[string]time.Time),
		}

		// 在Vercel环境中启动后台清理
		if IsVercelEnvironment() {
			go vercelOptimizer.backgroundCleanup()
		}
	})
	return vercelOptimizer
}

// GetOptimizedConnection 获取优化的数据库连接
func (vo *VercelOptimizer) GetOptimizedConnection(config DatabaseConfig) DatabaseInterface {
	// 生成配置的唯一键
	configKey := vo.generateConfigKey(config)

	vo.mu.Lock()
	defer vo.mu.Unlock()

	// 检查是否有现有连接
	if conn, exists := vo.connections[configKey]; exists {
		// 检查连接健康状态
		if err := conn.HealthCheck(); err == nil {
			vo.lastUsed[configKey] = time.Now()
			fmt.Printf("♻️  Reusing optimized database connection (key: %s)\n", configKey[:8])
			return conn
		} else {
			fmt.Printf("❌ Connection unhealthy, removing: %v\n", err)
			conn.Close()
			delete(vo.connections, configKey)
			delete(vo.lastUsed, configKey)
		}
	}

	// 创建新连接
	fmt.Printf("🔄 Creating new optimized database connection (key: %s)\n", configKey[:8])
	conn := NewDatabase(config)

	vo.connections[configKey] = conn
	vo.lastUsed[configKey] = time.Now()

	return conn
}

// generateConfigKey 生成配置的唯一键
func (vo *VercelOptimizer) generateConfigKey(config DatabaseConfig) string {
    return fmt.Sprintf("%s_%s_%s_%t",
        hashString(config.PostgresDSN),
        hashString(config.SupabaseURL),
        hashString(config.SupabaseKey),
        config.Debug,
    )
}

// hashString 简单的字符串哈希（用于生成短键）
func hashString(s string) string {
	if s == "" {
		return "empty"
	}
	if len(s) > 8 {
		return s[:4] + s[len(s)-4:]
	}
	return s
}

// backgroundCleanup 后台清理过期连接
func (vo *VercelOptimizer) backgroundCleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			vo.cleanupExpiredConnections()
		}
	}
}

// cleanupExpiredConnections 清理过期连接
func (vo *VercelOptimizer) cleanupExpiredConnections() {
	vo.mu.Lock()
	defer vo.mu.Unlock()

	now := time.Now()
	expiredKeys := []string{}

	for key, lastUsed := range vo.lastUsed {
		// 在Vercel中，连接空闲超过10分钟就清理
		if now.Sub(lastUsed) > 10*time.Minute {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		if conn, exists := vo.connections[key]; exists {
			fmt.Printf("🧹 Cleaning up expired connection (key: %s)\n", key[:8])
			conn.Close()
			delete(vo.connections, key)
			delete(vo.lastUsed, key)
		}
	}

	if len(expiredKeys) > 0 {
		fmt.Printf("🧹 Cleaned up %d expired connections\n", len(expiredKeys))
	}
}

// GetStats 获取优化器统计信息
func (vo *VercelOptimizer) GetStats() map[string]interface{} {
	vo.mu.RLock()
	defer vo.mu.RUnlock()

	stats := map[string]interface{}{
		"total_connections": len(vo.connections),
		"connections":       make([]map[string]interface{}, 0),
	}

	for key, lastUsed := range vo.lastUsed {
		connInfo := map[string]interface{}{
			"key":       key[:8] + "...",
			"last_used": lastUsed.Format(time.RFC3339),
			"age":       time.Since(lastUsed).String(),
		}
		stats["connections"] = append(stats["connections"].([]map[string]interface{}), connInfo)
	}

	return stats
}

// ForceCleanup 强制清理所有连接
func (vo *VercelOptimizer) ForceCleanup() {
	vo.mu.Lock()
	defer vo.mu.Unlock()

	fmt.Printf("🧹 Force cleaning up all connections\n")

	for key, conn := range vo.connections {
		conn.Close()
		delete(vo.connections, key)
		delete(vo.lastUsed, key)
	}
}

// GetOptimizedDatabase 全局函数，获取优化的数据库连接
func GetOptimizedDatabase(config DatabaseConfig) DatabaseInterface {
	if IsVercelEnvironment() {
		// 在Vercel环境中使用优化器
		optimizer := GetVercelOptimizer()
		return optimizer.GetOptimizedConnection(config)
	} else {
		// 在非Vercel环境中使用简单的连接池
		return GetDatabase(config)
	}
}

// IsVercelEnvironment 检查是否在Vercel环境中（导出版本）
func IsVercelEnvironment() bool {
	vercelEnv := os.Getenv("VERCEL_ENV")
	vercelURL := os.Getenv("VERCEL_URL")
	awsLambda := os.Getenv("AWS_LAMBDA_FUNCTION_NAME")

	return vercelEnv != "" || vercelURL != "" || awsLambda != ""
}
