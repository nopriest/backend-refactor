package database

import (
	"fmt"
	"sync"
	"time"
)

// DatabasePool 数据库连接池
type DatabasePool struct {
	instance DatabaseInterface
	config   DatabaseConfig
	mu       sync.RWMutex
	lastUsed time.Time
}

var (
	globalPool *DatabasePool
	poolMutex  sync.Mutex
)

// GetDatabase 获取数据库连接（单例模式 + 连接池）
func GetDatabase(config DatabaseConfig) DatabaseInterface {
	poolMutex.Lock()
	defer poolMutex.Unlock()

	// 检查是否需要创建新的连接池
	if globalPool == nil || shouldRecreateConnection(globalPool, config) {
		fmt.Printf("🔄 Creating new database connection pool\n")
		
		// 关闭旧连接（如果存在）
		if globalPool != nil && globalPool.instance != nil {
			globalPool.instance.Close()
		}

		// 创建新连接
		instance := NewDatabase(config)
		globalPool = &DatabasePool{
			instance: instance,
			config:   config,
			lastUsed: time.Now(),
		}
	} else {
		// 更新最后使用时间
		globalPool.mu.Lock()
		globalPool.lastUsed = time.Now()
		globalPool.mu.Unlock()
		
		fmt.Printf("♻️  Reusing existing database connection\n")
	}

	return globalPool.instance
}

// shouldRecreateConnection 判断是否需要重新创建连接
func shouldRecreateConnection(pool *DatabasePool, newConfig DatabaseConfig) bool {
	if pool == nil || pool.instance == nil {
		return true
	}

	// 检查配置是否发生变化
	if !configEquals(pool.config, newConfig) {
		fmt.Printf("🔄 Database configuration changed, recreating connection\n")
		return true
	}

	// 检查连接是否过期（30分钟）
	pool.mu.RLock()
	expired := time.Since(pool.lastUsed) > 30*time.Minute
	pool.mu.RUnlock()

	if expired {
		fmt.Printf("⏰ Database connection expired, recreating\n")
		return true
	}

	// 检查连接健康状态
	if err := pool.instance.HealthCheck(); err != nil {
		fmt.Printf("❌ Database health check failed, recreating: %v\n", err)
		return true
	}

	return false
}

// configEquals 比较两个数据库配置是否相等
func configEquals(a, b DatabaseConfig) bool {
	return a.UseLocalDB == b.UseLocalDB &&
		a.PostgresDSN == b.PostgresDSN &&
		a.SupabaseURL == b.SupabaseURL &&
		a.SupabaseKey == b.SupabaseKey
}

// CleanupIdleConnections 清理空闲连接（可以在后台定期调用）
func CleanupIdleConnections() {
	poolMutex.Lock()
	defer poolMutex.Unlock()

	if globalPool == nil {
		return
	}

	globalPool.mu.RLock()
	idle := time.Since(globalPool.lastUsed) > 10*time.Minute
	globalPool.mu.RUnlock()

	if idle {
		fmt.Printf("🧹 Cleaning up idle database connection\n")
		if globalPool.instance != nil {
			globalPool.instance.Close()
		}
		globalPool = nil
	}
}

// GetConnectionStats 获取连接池统计信息
func GetConnectionStats() map[string]interface{} {
	poolMutex.Lock()
	defer poolMutex.Unlock()

	if globalPool == nil {
		return map[string]interface{}{
			"status":    "no_connection",
			"last_used": nil,
		}
	}

	globalPool.mu.RLock()
	lastUsed := globalPool.lastUsed
	globalPool.mu.RUnlock()

	return map[string]interface{}{
		"status":    "connected",
		"last_used": lastUsed.Format(time.RFC3339),
		"age":       time.Since(lastUsed).String(),
		"config": map[string]interface{}{
			"use_local_db": globalPool.config.UseLocalDB,
			"has_postgres": globalPool.config.PostgresDSN != "",
			"has_supabase": globalPool.config.SupabaseURL != "",
		},
	}
}
