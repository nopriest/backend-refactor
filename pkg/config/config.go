package config

import (
    "bufio"
    "fmt"
    "os"
    "strconv"
    "strings"
    "sync"
)

// Config 应用配置结构
type Config struct {
	// 环境配置
	Environment string
	Port        string

	// 数据库配置
	UseLocalDB  bool
	PostgresDSN string
	SupabaseURL string
	SupabaseKey string

	// JWT配置
	JWTSecret string

	// Paddle配置
	PaddleAPIKey        string
	PaddleEnvironment   string
	PaddleWebhookSecret string
	PaddleProPriceID    string
	PaddlePowerPriceID  string

	// OAuth配置
	GoogleClientID     string
	GoogleClientSecret string
	GitHubClientID     string
	GitHubClientSecret string
	OAuthRedirectURI   string
	BaseURL            string // 基础URL，用于构建回调URL

	// CORS配置
	AllowedOrigins []string

	// 调试配置
	Debug bool
}

// LoadConfig 加载配置（支持本地和Vercel环境）
func LoadConfig() *Config {
	// 根据环境加载对应的 .env 文件
	env := os.Getenv("ENVIRONMENT")
	if env == "" {
		env = "development" // 默认开发环境
	}

	// 按优先级加载环境文件
	switch env {
	case "production":
		loadEnvFile(".env.production")
	case "development":
		loadEnvFile(".env.local")
	default:
		loadEnvFile(".env.local")
	}

	config := &Config{
		// 默认值
		Environment: getEnvWithDefault("ENVIRONMENT", "development"),
		Port:        getEnvWithDefault("PORT", "3000"),
		UseLocalDB:  getEnvBool("USE_LOCAL_DB", true),
		JWTSecret:   getEnvWithDefault("JWT_SECRET", "your-secret-key-change-in-production"),
		Debug:       getEnvBool("DEBUG", false),
	}

	// 数据库配置
    // Trim whitespace to avoid trailing spaces/newlines from env sources
    config.PostgresDSN = strings.TrimSpace(os.Getenv("POSTGRES_DSN"))
    config.SupabaseURL = strings.TrimSpace(os.Getenv("SUPABASE_URL"))
    config.SupabaseKey = strings.TrimSpace(os.Getenv("SUPABASE_SERVICE_KEY"))

	// Paddle配置
	config.PaddleAPIKey = os.Getenv("PADDLE_API_KEY")
	config.PaddleEnvironment = getEnvWithDefault("PADDLE_ENVIRONMENT", "sandbox")
	config.PaddleWebhookSecret = os.Getenv("PADDLE_WEBHOOK_SECRET")
	config.PaddleProPriceID = os.Getenv("PADDLE_PRO_PRICE_ID")
	config.PaddlePowerPriceID = os.Getenv("PADDLE_POWER_PRICE_ID")

	// OAuth配置
    config.GoogleClientID = strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID"))
    config.GoogleClientSecret = strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_SECRET"))
    config.GitHubClientID = strings.TrimSpace(os.Getenv("GITHUB_CLIENT_ID"))
    config.GitHubClientSecret = strings.TrimSpace(os.Getenv("GITHUB_CLIENT_SECRET"))
    config.OAuthRedirectURI = strings.TrimSpace(os.Getenv("OAUTH_REDIRECT_URI"))
    config.BaseURL = strings.TrimSpace(os.Getenv("BASE_URL"))

	// CORS配置
	allowedOrigins := getEnvWithDefault("ALLOWED_ORIGINS", "*")
	if allowedOrigins == "*" {
		config.AllowedOrigins = []string{"*"}
	} else {
		config.AllowedOrigins = strings.Split(allowedOrigins, ",")
	}

	// 环境特定配置
	if config.Environment == "production" {
		// 生产环境强制使用外部数据库（PostgreSQL或Supabase）
		if config.PostgresDSN != "" || (config.SupabaseURL != "" && config.SupabaseKey != "") {
			config.UseLocalDB = false
		} else {
			// 生产环境没有配置外部数据库，这是一个警告
			fmt.Println("⚠️  WARNING: Production environment using local file database. Please configure POSTGRES_DSN or SUPABASE_URL+SUPABASE_SERVICE_KEY")
		}
		// 生产环境关闭调试
		config.Debug = false
	}

	return config
}

// Cached config (initialized once per cold start)
var (
    cachedConfig *Config
    configOnce   sync.Once
)

// GetCached returns the process-wide cached Config.
// On serverless (Vercel), it initializes once per cold start and
// reuses it across warm invocations, avoiding per-request parsing.
func GetCached() *Config {
    configOnce.Do(func() {
        cachedConfig = LoadConfig()
    })
    return cachedConfig
}

// Validate 验证配置
func (c *Config) Validate() error {
	// 验证端口
	if c.Port == "" {
		return fmt.Errorf("PORT is required")
	}

	// 验证JWT密钥
	if c.JWTSecret == "" || c.JWTSecret == "your-secret-key-change-in-production" || c.JWTSecret == "your-local-development-secret-key" {
		if c.Environment == "production" {
			return fmt.Errorf("JWT_SECRET must be set in production")
		}
		if c.Environment == "development" {
			fmt.Println("⚠️  Using default JWT secret (not recommended for production)")
		}
	}

	// 验证数据库配置
	if c.UseLocalDB {
		// 使用本地文件数据库，无需额外验证
	} else if c.PostgresDSN != "" {
		// 使用本地PostgreSQL，无需额外验证
	} else if c.SupabaseURL != "" && c.SupabaseKey != "" {
		// 使用Supabase，无需额外验证
	} else {
		return fmt.Errorf("数据库配置不完整：请配置 POSTGRES_DSN 或 SUPABASE_URL+SUPABASE_SERVICE_KEY")
	}

	return nil
}

// IsProduction 检查是否为生产环境
func (c *Config) IsProduction() bool {
	return c.Environment == "production"
}

// IsDevelopment 检查是否为开发环境
func (c *Config) IsDevelopment() bool {
	return c.Environment == "development"
}

// 辅助函数

// getEnvWithDefault 获取环境变量，如果不存在则使用默认值
func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvBool 获取布尔类型的环境变量
func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if parsed, err := strconv.ParseBool(value); err == nil {
			return parsed
		}
	}
	return defaultValue
}

// loadEnvFile 加载 .env 文件到环境变量
func loadEnvFile(filename string) {
	// 检查文件是否存在
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return // 文件不存在，静默返回
	}

	file, err := os.Open(filename)
	if err != nil {
		return // 无法打开文件，静默返回
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// 跳过空行和注释行
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// 解析 KEY=VALUE 格式
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		// 移除值两端的引号（如果有）
		if len(value) >= 2 {
			if (strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"")) ||
				(strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'")) {
				value = value[1 : len(value)-1]
			}
		}

		// 只有当环境变量不存在时才设置
		if os.Getenv(key) == "" {
			os.Setenv(key, value)
		}
	}
}
