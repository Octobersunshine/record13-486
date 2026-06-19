# Read-Only Database API

Go HTTP 服务，提供只读数据库连接会话，仅支持 SELECT 查询语句执行。

## 功能特性

- **只读数据库连接**: 使用 SQLite 的 `mode=ro` 和 `_readonly=true` 参数确保数据库以只读模式打开
- **会话管理**: 每个连接创建独立的会话，支持会话过期自动清理
- **SQL 安全检查**: 严格的 SQL 解析器，仅允许 SELECT 查询，禁止所有写入操作
- **多层安全防护**:
  - 数据库级别的只读模式
  - PRAGMA `query_only = true` 设置
  - 应用层 SQL 关键词过滤
  - 只读事务隔离
- **HTTP API**: 简洁的 RESTful API 接口

## 项目结构

```
.
├── main.go                    # 主程序入口
├── init_db.go                 # 数据库初始化脚本
├── go.mod
├── internal/
│   ├── database/
│   │   └── session.go         # 数据库会话管理
│   ├── sqlparser/
│   │   └── sqlparser.go       # SQL 安全解析器
│   └── handler/
│       └── handler.go         # HTTP 处理器
└── test.db                    # SQLite 数据库文件（运行后生成）
```

## 快速开始

### 1. 安装依赖

```bash
go mod tidy
```

### 2. 初始化测试数据库

```bash
go run init_db.go
```

### 3. 启动服务

```bash
go run main.go
```

或指定参数：

```bash
go run main.go -db test.db -addr :8080 -timeout 30m
```

## API 接口

### 1. 健康检查

```bash
GET /health
```

响应示例：

```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T12:00:00Z",
  "service": "readonly-db-api"
}
```

### 2. 创建只读会话

```bash
POST /session/create
```

响应示例：

```json
{
  "session_id": "550e8400-e29b-41d4-a716-446655440000",
  "created_at": "2024-01-01T12:00:00Z",
  "expires_at": "2024-01-01T12:30:00Z",
  "message": "Read-only session created successfully. Use session_id in X-Session-Id header for queries."
}
```

### 3. 执行查询

```bash
POST /query
Headers:
  X-Session-Id: <session_id>
  Content-Type: application/json

Body:
{
  "sql": "SELECT * FROM users WHERE id = ?"
}
```

响应示例：

```json
{
  "columns": ["id", "username", "email", "age", "created_at", "is_active"],
  "rows": [
    {
      "id": 1,
      "username": "alice",
      "email": "alice@example.com",
      "age": 25,
      "created_at": "2023-06-15T10:30:00Z",
      "is_active": true
    }
  ],
  "count": 1,
  "execution_time_ms": 1.23
}
```

### 4. 关闭会话

```bash
POST /session/close
Headers:
  X-Session-Id: <session_id>
```

## 使用示例

### 使用 curl

```bash
# 1. 创建会话
SESSION_ID=$(curl -s -X POST http://localhost:8080/session/create | jq -r '.session_id')

# 2. 执行查询
curl -X POST http://localhost:8080/query \
  -H "X-Session-Id: $SESSION_ID" \
  -H "Content-Type: application/json" \
  -d '{"sql": "SELECT id, username, email FROM users LIMIT 5"}'

# 3. 多表关联查询
curl -X POST http://localhost:8080/query \
  -H "X-Session-Id: $SESSION_ID" \
  -H "Content-Type: application/json" \
  -d '{
    "sql": "SELECT u.username, COUNT(o.id) as order_count, SUM(o.total_price) as total_spent FROM users u LEFT JOIN orders o ON u.id = o.user_id GROUP BY u.id ORDER BY total_spent DESC"
  }'

# 4. 关闭会话
curl -X POST http://localhost:8080/session/close \
  -H "X-Session-Id: $SESSION_ID"
```

### 示例查询

```sql
-- 简单查询
SELECT * FROM users;

-- 聚合查询
SELECT category, COUNT(*) as count, AVG(price) as avg_price 
FROM products 
GROUP BY category;

-- 多表关联
SELECT u.username, p.name, o.quantity, o.total_price 
FROM orders o
JOIN users u ON o.user_id = u.id
JOIN products p ON o.product_id = p.id
WHERE o.status = 'delivered'
LIMIT 10;

-- CTE 查询
WITH user_stats AS (
    SELECT user_id, COUNT(*) as order_count, SUM(total_price) as total
    FROM orders
    GROUP BY user_id
)
SELECT u.username, us.order_count, us.total
FROM users u
JOIN user_stats us ON u.id = us.user_id
WHERE us.total > 1000;
```

## 安全限制

### 禁止的 SQL 操作

所有以下操作都会被拒绝：

- **写入操作**: `INSERT`, `UPDATE`, `DELETE`, `REPLACE`, `MERGE`
- **DDL 操作**: `CREATE`, `DROP`, `ALTER`, `TRUNCATE`, `RENAME`
- **权限操作**: `GRANT`, `REVOKE`
- **执行操作**: `EXECUTE`, `EXEC`, `CALL`
- **事务控制**: `BEGIN`, `COMMIT`, `ROLLBACK`
- **锁定子句**: `FOR UPDATE`, `FOR SHARE` 等
- **危险函数**: `pg_sleep`, `sleep`, `load_file`, `xp_cmdshell` 等
- **SELECT INTO**: 不允许 `SELECT ... INTO` 语法

### 禁止操作示例

```bash
# 会被拒绝 - INSERT 操作
curl -X POST http://localhost:8080/query \
  -H "X-Session-Id: $SESSION_ID" \
  -H "Content-Type: application/json" \
  -d '{"sql": "INSERT INTO users (username) VALUES (\"hacker\")"}'

# 会被拒绝 - DROP 操作
curl -X POST http://localhost:8080/query \
  -H "X-Session-Id: $SESSION_ID" \
  -H "Content-Type: application/json" \
  -d '{"sql": "DROP TABLE users"}'

# 会被拒绝 - FOR UPDATE
curl -X POST http://localhost:8080/query \
  -H "X-Session-Id: $SESSION_ID" \
  -H "Content-Type: application/json" \
  -d '{"sql": "SELECT * FROM users FOR UPDATE"}'
```

## 安全特性

1. **多层只读防护**
   - SQLite 连接字符串 `mode=ro&_readonly=true`
   - `PRAGMA query_only = true` 确保数据库级只读
   - `sql.TxOptions{ReadOnly: true}` 事务级只读
   - 应用层 SQL 关键词过滤

2. **会话隔离**
   - 每个会话独立数据库连接
   - 最大连接数限制为 1
   - 会话超时自动清理

3. **查询超时**
   - 默认 30 秒查询超时
   - 防止慢查询阻塞服务

4. **输入验证**
   - 严格的 SQL 语法检查
   - 危险关键词和函数过滤
   - SQL 注入防护

## 命令行参数

| 参数 | 默认值 | 说明 |
|------|--------|------|
| `-db` | `test.db` | SQLite 数据库文件路径 |
| `-addr` | `:8080` | HTTP 服务监听地址 |
| `-timeout` | `30m` | 会话超时时间 |

## 响应状态码

| 状态码 | 说明 |
|--------|------|
| 200 | 请求成功 |
| 201 | 会话创建成功 |
| 400 | 请求参数错误 |
| 401 | 会话无效或已过期 |
| 403 | 查询被禁止（非 SELECT 语句） |
| 404 | 会话不存在 |
| 405 | 方法不允许 |
| 500 | 服务器内部错误 |

## 注意事项

1. 当前使用 SQLite 作为数据库，如需支持 MySQL/PostgreSQL 等其他数据库，需要修改 `internal/database/session.go` 中的驱动和连接参数
2. 确保数据库文件对服务进程有读取权限
3. 生产环境建议使用更严格的网络隔离和认证机制
