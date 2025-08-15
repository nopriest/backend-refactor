# Tab Sync Backend - Refactored

基于Vercel平台Go无服务器函数最佳实践的重构版本，采用"单体路由模式"架构。

## 🚀 重构亮点

### 架构优势
- **单体路由模式**: 所有API端点通过单一入口`api/index.go`统一管理，解决了Vercel文件路由的代码分散问题
- **Go Chi路由器**: 使用高性能的Chi路由器处理请求分发，支持中间件链和路由组
- **统一中间件**: 集中管理CORS、认证、日志、错误处理等横切关注点
- **数据库抽象**: 支持本地文件、PostgreSQL、Supabase的无缝切换
- **Vercel优化**: 针对Vercel平台的Go函数进行了专门优化

### 性能提升
- **冷启动优化**: Go的AOT编译特性实现极速启动（~40ms vs Node.js ~133ms）
- **内存效率**: 相比Node.js节省2-3倍内存使用（16-20MB vs 64-90MB）
- **并发性能**: Goroutines提供真正的并行处理能力
- **可预测性**: 稳定的性能表现，无GC暂停影响

## 📁 项目结构

```
backend-refactor/
├── api/
│   └── index.go          # Vercel函数入口，集成Chi路由器
├── pkg/
│   ├── config/           # 配置管理
│   │   └── config.go     # 环境变量加载和验证
│   ├── database/         # 数据库抽象层
│   │   ├── interface.go  # 数据库接口定义
│   │   ├── local.go      # 本地文件数据库实现
│   │   ├── postgres.go   # PostgreSQL数据库实现
│   │   └── supabase.go   # Supabase数据库实现
│   ├── handlers/         # API处理器
│   │   ├── auth.go       # 认证相关处理器
│   │   └── snapshots.go  # 快照管理处理器
│   ├── middleware/       # 中间件
│   │   ├── auth.go       # JWT认证中间件
│   │   ├── cors.go       # CORS中间件
│   │   ├── logging.go    # 日志中间件
│   │   ├── recovery.go   # 错误恢复中间件
│   │   └── validation.go # 请求验证中间件
│   ├── models/           # 数据模型
│   │   ├── user.go       # 用户模型
│   │   ├── tab.go        # 标签页模型
│   │   └── subscription.go # 订阅模型
│   └── utils/            # 工具函数
│       ├── response.go   # HTTP响应工具
│       └── jwt.go        # JWT工具
├── scripts/              # 脚本文件
│   ├── init_db.sql       # 数据库初始化脚本
│   └── setup_db.go       # 数据库设置工具
├── .env.example          # 环境变量示例
├── .env.local            # 本地开发环境变量
├── vercel.json           # Vercel配置，路由重写
├── go.mod                # Go模块定义
└── README.md
```

## 🛠️ 本地开发

### 1. 环境准备

```bash
# 克隆项目
git clone <your-repo>
cd backend-refactor

# 安装Go依赖
go mod tidy
```

### 2. 数据库配置

#### 选项1：使用本地文件数据库（推荐用于快速开发）
```bash
# 复制环境变量文件
cp .env.example .env.local

# 编辑 .env.local，确保：
USE_LOCAL_DB=true
```

#### 选项2：使用本地PostgreSQL
```bash
# 启动PostgreSQL（Docker方式）
docker run --name tabsync-postgres \
  -e POSTGRES_PASSWORD=123456 \
  -e POSTGRES_DB=postgres \
  -p 5432:5432 -d postgres:15

# 编辑 .env.local：
# USE_LOCAL_DB=false
POSTGRES_DSN=postgres://postgres:123456@localhost:5432/postgres?sslmode=disable

# 初始化数据库（可选）
go run scripts/setup_db.go
```

### 3. 启动开发服务器

```bash
# 使用Vercel Dev（推荐）
vercel dev --listen 3000

# 服务器将在以下地址启动：
# 🌐 健康检查: http://localhost:3000/
# 📡 API基础路径: http://localhost:3000/api
```

### 4. 测试API

```bash
# 健康检查
curl http://localhost:3000/

# 测试订阅状态检查（需要有效用户ID）
curl -X POST http://localhost:3000/api/auth \
  -H "Content-Type: application/json" \
  -d '{"provider": "check_subscription", "user_id": "test-user-123"}'
```

## 🚀 部署到Vercel

### 1. 环境变量配置

在Vercel Dashboard中配置以下环境变量：

```bash
# 基础配置
ENVIRONMENT=production
JWT_SECRET=your-production-jwt-secret

# 数据库配置（选择一种）
# PostgreSQL
USE_LOCAL_DB=false
POSTGRES_DSN=postgres://user:pass@host:port/db?sslmode=require

# 或者 Supabase
USE_LOCAL_DB=false
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_SERVICE_KEY=your-service-key

# CORS配置
ALLOWED_ORIGINS=https://your-domain.com,https://your-extension-id.chromiumapp.org

# Paddle配置（如果需要）
PADDLE_API_KEY=your-paddle-api-key
PADDLE_ENVIRONMENT=production
PADDLE_WEBHOOK_SECRET=your-webhook-secret
```

### 2. 部署

```bash
# 推送到Git仓库
git add .
git commit -m "Deploy refactored backend"
git push origin main

# Vercel会自动检测并部署
```

## 📊 API端点

### 公开端点（无需认证）

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/` | 健康检查 |
| POST | `/api/auth/` | 检查用户订阅状态 |
| POST | `/api/auth/register` | 用户注册 |
| POST | `/api/auth/login` | 用户登录 |
| POST | `/api/auth/refresh` | 刷新令牌 |
| POST | `/api/auth/oauth/google` | Google OAuth |
| POST | `/api/auth/oauth/github` | GitHub OAuth |

### 需要认证的端点

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/snapshots/` | 列出用户快照 |
| POST | `/api/snapshots/` | 创建新快照 |
| GET | `/api/snapshots/{name}` | 获取指定快照 |
| PUT | `/api/snapshots/{name}` | 更新快照 |
| DELETE | `/api/snapshots/{name}` | 删除快照 |
| GET | `/api/user/profile` | 获取用户资料 |
| GET | `/api/ai/credits` | 获取AI积分 |

## 🔧 配置说明

### 数据库自动选择逻辑

系统按以下优先级自动选择数据库：

1. **本地文件数据库**: `USE_LOCAL_DB=true`
2. **PostgreSQL**: `POSTGRES_DSN` 已配置
3. **Supabase**: `SUPABASE_URL` 和 `SUPABASE_SERVICE_KEY` 已配置
4. **默认**: 回退到本地文件数据库

### 环境特定行为

- **开发环境** (`ENVIRONMENT=development`):
  - 详细的错误信息和堆栈跟踪
  - 彩色日志输出
  - 允许所有CORS来源
  - 启用调试模式

- **生产环境** (`ENVIRONMENT=production`):
  - 简化的错误信息
  - 结构化JSON日志
  - 严格的CORS策略
  - 安全头部设置

## 🔍 故障排除

### 常见问题

1. **数据库连接失败**
   ```bash
   # 检查PostgreSQL连接
   psql -h localhost -U postgres -d postgres -c "SELECT 1;"

   # 检查环境变量
   echo $POSTGRES_DSN
   ```

2. **Vercel部署失败**
   - 确保`go.mod`和`go.sum`文件已提交
   - 检查环境变量是否正确配置
   - 查看Vercel部署日志

3. **CORS错误**
   - 检查`ALLOWED_ORIGINS`环境变量
   - 确保前端域名已添加到允许列表

### 调试技巧

```bash
# 查看详细日志
vercel dev --debug

# 测试数据库连接
go run scripts/setup_db.go

# 检查配置加载
curl http://localhost:3000/ | jq
```

## 📈 性能监控

重构后的系统提供了以下性能指标：

- **冷启动时间**: 通过健康检查端点的响应时间
- **内存使用**: Vercel函数监控面板
- **数据库性能**: 通过日志中的查询时间
- **错误率**: 通过结构化日志和Vercel Analytics

## 🔄 从原版本迁移

如果您正在从原始版本迁移：

1. **数据兼容性**: 新版本完全兼容现有PostgreSQL数据结构
2. **API兼容性**: 保持了关键API端点的向后兼容性
3. **环境变量**: 大部分环境变量保持不变，新增了一些优化选项

## 🤝 贡献

欢迎提交Issue和Pull Request来改进这个项目！
