# Go Web 应用项目

## 项目简介

这是一个基于Go语言开发的综合性Web应用，集成了多种实用功能模块，包括用户认证、汇率查询、天气信息、游戏系统、文章管理等。项目采用前后端分离架构，后端基于RESTful API设计，前端使用HTML模板渲染。

## 技术栈
- **Go的标准库**： encoding/json、io、context、containers、strconv、container/heap等go标准库的使用
- **后端框架**: Gin (高性能Go Web框架)
- **ORM**: GORM (Go语言ORM库)
- **数据库**: MySQL (主数据库)
- **缓存**: Redis (缓存和会话存储)
- **日志**: Zap (高性能日志库)
- **API文档**: Swagger (自动生成API文档)
- **认证**: JWT (JSON Web Token)
- **中间件**: 自定义日志、恢复、认证、权限控制中间件

## 功能特性

### 用户系统
- 用户注册、登录、登出
- JWT认证机制
- 角色权限管理（普通用户、管理员、超级管理员）

### 汇率查询
- 实时汇率数据获取
- 人民币汇率Top10排行
- 汇率数据可视化展示

### 天气信息
- 城市天气数据获取
- Top10城市天气排行
- 用户地理信息的定位
- 地理位置相关天气信息

### 游戏系统
- 猜数字游戏
- 地图时间挑战游戏
- 游戏排行榜
- 个人成绩追踪

### 文章管理
- 文章列表展示
- 文章详情页面

### 其他功能
- 图片代理服务
- Shell命令执行界面
- 管理员仪表板

## 项目结构

```
project/
├── config/          # 配置文件和配置处理
├── controllers/     # 控制器层
├── docs/           # Swagger API文档
├── global/         # 全局变量
├── middlewares/    # 中间件
├── models/         # 数据模型
├── router/         # 路由配置
├── static/         # 静态资源
├── templates/      # HTML模板
└── utils/          # 工具函数
```

## 安装与运行

### 环境要求

- Go 1.16+
- MySQL 5.7+
- Redis 6.0+

### 安装步骤

1. 克隆项目
```bash
git clone <项目仓库地址>
cd project
```

2. 安装依赖
```bash
go mod download
```

3. 配置数据库

修改 `config/config.yaml` 文件中的数据库和Redis连接信息:

```yaml
database:
  dsn: your_username:your_password@tcp(127.0.0.1:3306)/your_database?charset=utf8mb4&parseTime=True&loc=Local
  
redis:
  addr: localhost:6379
  DB: 0
  Password: "your_redis_password"
```

4. 初始化数据库

确保MySQL中已创建对应数据库，表结构会在应用启动时自动创建。

5. 生成Swagger文档
```bash
swag init
```

6. 启动应用
```bash
go run main.go
```

应用默认运行在 `http://localhost:3000`

## API文档

启动应用后，可以通过以下地址访问Swagger API文档:

```
http://localhost:3000/swagger/index.html
```

## 默认账号

- **超级管理员**:
  - 用户名: `superadmin`
  - 密码: `admin123456`

## 开发说明

### 日志系统

项目使用Zap日志库，实现了开发与生产环境的不同日志级别。日志流程如下:

```
请求到达
↓
GinLogger中间件开始 (记录start时间)
↓
c.Next() → 执行其他中间件
↓
c.Next() → 执行处理函数
↓
处理函数返回 (设置响应状态码)
↓
回到GinLogger (计算耗时，记录完整日志)
↓
返回响应
```

### 中间件

- **日志中间件**: 记录请求处理时间和详细信息
- **恢复中间件**: 捕获panic并记录错误堆栈
- **认证中间件**: JWT token验证
- **权限中间件**: 基于角色的访问控制

## 许可证

[MIT License](LICENSE)