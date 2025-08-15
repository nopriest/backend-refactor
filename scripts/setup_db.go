package main

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	// 从环境变量或命令行参数获取数据库连接字符串
	dsn := os.Getenv("POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://postgres:123456@localhost:5432/postgres?sslmode=disable"
	}

	if len(os.Args) > 1 {
		dsn = os.Args[1]
	}

	fmt.Printf("🔗 Connecting to database: %s\n", maskPassword(dsn))

	// 连接数据库
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 测试连接
	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Failed to ping database: %v", err)
	}

	fmt.Println("✅ Database connection successful")

	// 读取SQL脚本
	sqlContent, err := ioutil.ReadFile("scripts/init_db.sql")
	if err != nil {
		log.Fatalf("❌ Failed to read init_db.sql: %v", err)
	}

	fmt.Println("📄 Executing database initialization script...")

	// 执行SQL脚本
	_, err = db.Exec(string(sqlContent))
	if err != nil {
		log.Fatalf("❌ Failed to execute SQL script: %v", err)
	}

	fmt.Println("✅ Database initialization completed successfully!")

	// 验证表是否创建成功
	tables := []string{"users", "snapshots", "subscription_plans", "user_subscriptions", "ai_credits"}
	fmt.Println("🔍 Verifying tables...")

	for _, table := range tables {
		var count int
		err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
		if err != nil {
			log.Printf("⚠️  Warning: Failed to query table %s: %v", table, err)
		} else {
			fmt.Printf("✅ Table %s: %d records\n", table, count)
		}
	}

	// 测试用户查询
	fmt.Println("🧪 Testing user query...")
	var email, tier string
	err = db.QueryRow("SELECT email, tier FROM users WHERE id = 'test-user-123'").Scan(&email, &tier)
	if err != nil {
		log.Printf("⚠️  Warning: Failed to query test user: %v", err)
	} else {
		fmt.Printf("✅ Test user found: %s (tier: %s)\n", email, tier)
	}

	fmt.Println("🎉 Database setup completed! You can now run 'vercel dev' to start the application.")
}

// maskPassword 隐藏连接字符串中的密码
func maskPassword(dsn string) string {
	// 简单的密码隐藏逻辑
	if len(dsn) > 50 {
		return dsn[:20] + "***" + dsn[len(dsn)-20:]
	}
	return dsn[:10] + "***"
}
