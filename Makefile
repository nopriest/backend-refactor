# Tab Sync Backend Refactor - Makefile

.PHONY: help setup-db dev build clean test

# 默认目标
help:
	@echo "Tab Sync Backend Refactor - Available Commands:"
	@echo ""
	@echo "  setup-db    - Initialize PostgreSQL database with tables and test data"
	@echo "  dev         - Start development server with vercel dev"
	@echo "  build       - Build the project"
	@echo "  clean       - Clean build artifacts"
	@echo "  test        - Run tests"
	@echo "  deps        - Install dependencies"
	@echo ""

# 安装依赖
deps:
	@echo "📦 Installing Go dependencies..."
	go mod tidy
	go mod download

# 初始化数据库
setup-db:
	@echo "🗄️  Setting up PostgreSQL database..."
	go run scripts/setup_db.go

# 启动开发服务器
dev:
	@echo "🚀 Starting development server..."
	@echo "📊 Environment: development"
	@echo "🌐 Server will be available at: http://localhost:3000"
	@echo "📡 API endpoints: http://localhost:3000/api"
	@echo ""
	vercel dev --listen 3000

# 构建项目
build:
	@echo "🔨 Building project..."
	go build -o bin/setup-db scripts/setup_db.go
	@echo "✅ Build completed"

# 清理构建产物
clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -rf bin/
	rm -rf .vercel/
	rm -rf data/
	@echo "✅ Clean completed"

# 运行测试
test:
	@echo "🧪 Running tests..."
	go test ./...

# 检查代码格式
fmt:
	@echo "🎨 Formatting code..."
	go fmt ./...

# 代码检查
lint:
	@echo "🔍 Running linter..."
	golangci-lint run

# 完整设置（首次使用）
setup: deps setup-db
	@echo "🎉 Setup completed! You can now run 'make dev' to start development."

# 快速重置（重新初始化数据库）
reset: clean setup-db
	@echo "🔄 Reset completed!"
