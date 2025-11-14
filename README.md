<div align="center">

# Go-Web 综合性 Web 应用项目

</div>

## 📝 项目简介

Go-Web 是目前一个以 Go 语言为核心构建的综合性 Web 应用，集成文章互动中心、文件管理、游戏大厅、天气与汇率查询、AI 翻译等多个业务模块。项目采用 **Gin + Gorm + Mysql + Redis + HTML模板渲染** 的轻量方案，同时提供完整的 RESTful API。

### 🎯 项目特色

- 📝 **前后端分离**：前后端分离，前端使用原生 JavaScript、html和css，后端使用 Gin + Gorm + Mysql + Redis 框架
- 🧾 **文章论坛系统**：多用户的文章写作、点赞、评论、转发、收藏夹及收藏文件与广告位一应俱全
- 📁 **文件管理**：基于配额的安全文件上传、下载、筛选与下载统计
- 🎮 **多游戏大厅**：猜数字、地图寻路(三种迷宫生成算法)、2048 三款小游戏 + 实时排行榜
- 🌦️ **天气与定位**：高德 IP 定位 + 腾讯天气数据 + AQI 展示
- 💱 **汇率监控**：实时汇率、CNY Top10 榜单与手动刷新
- 🌐 **AI 翻译**：对接 OpenAI 风格接口，支持多语言翻译与历史管理
- 🔐 **安全与监控**：JWT + 角色权限 + Zap 日志 + 文件系统监控
- 📚 **自动化的API文档**：Swag 驱动的 Swagger 文档，随代码同步更新
- 🛡️ **后台管理**： 支持用户管理、数据统计、系统设置以及自定义终端查询系统数据资源
- 🔧 **持续集成**：支持 Docker 镜像构建与推送

## 🛠️ 技术栈

### 后端
- Go 1.24+
- Gin、GORM、MySQL、Redis
- Zap、fsnotify、Viper、JWT、bcrypt
- SSE协议与WebSocket
- Swagger（Swagger 文档）

### 前端
- HTML5 模板 + 原生 JavaScript
- CSS Variables、响应式布局
- Fetch API、LocalStorage

### 外部服务
- 自定义api的 IP 定位,这里使用高德开放平台
- 腾讯天气开放接口
- Frankfurter 汇率 API
- 自定义模型的翻译功能（OpenAI 风格翻译接口，这里本人使用Moonshot Kimi K2可自行替换）

## 🧰 系统要求

- `Go >= 1.24`
- 可用的 `MySQL` 实例（默认端口 `13306`，可在 `config/config.yaml` 修改）
- 可用的 `Redis` 实例（默认 `localhost:6379`）
- 生成接口文档需安装 `swag`：`go install github.com/swaggo/swag/cmd/swag@latest`

## 🌟 核心模块速览

- **用户与权限**：注册 / 登录 / 登出 / 注销、JWT 鉴权、角色权限管理（user/admin/superadmin） 
  - 支持用户注册(滑动窗口)和登录(redis锁)的限流以及高并发的登录功能
- **文章中心**：
  - Markdown 风格富文本（模板渲染）
  - 文章增删改查、分页、排序、关键词搜索
  - Redis 首页缓存、点赞 / 评论 / 收藏计数缓存
  - 点赞开关、评论树、收藏夹分组管理（转发计数逻辑已实现，按需挂载路由）
- **收藏夹系统**：
  - 多收藏夹管理、文章去重逻辑
  - 收藏次数冗余字段维护、并发加锁校准
  - 收藏夹内容一键拉取，按加入时间倒序
- **评论互动**：
  - 支持多级回复的树形结构
  - 3 秒频控防刷
  - 评论计数与 Redis 文章存在性缓存
- **游戏大厅**：
  - 猜数字：难度递进 + 返回最佳成绩
  - 地图寻路：A* 算法 + 用时排名
  - 2048：前端逻辑，后端存储分数
  - Redis ZSET 排行榜 + MySQL 历史持久化 + 个人排行
- **天气 & 定位**：自动定位城市天气、热门城市榜单、天气图标代理、AQI
- **汇率中心**：基础汇率 CRUD、CNY Top10、Redis Snapshot、手动刷新
- **AI 翻译服务**：
  - 自动或手动选择源语言，返回语言检测结果
  - 请求限额、历史记录、单条 / 批量删除
- **文件管理**：
  - 单文件 / 总配额限制（MB）
  - 安全路径拼接、临时文件写入、扩展名 + MIME 白名单
  - 支持分页、关键字、扩展名、MIME、时间、大小条件过滤
  - 下载次数统计、ETag/Range 支持、防止目录穿越
- **系统监控**：
  - Zap 日志（开发 / 生产模式）
  - fsnotify 目录监控器
  - 页面化仪表盘、Shell 页面入口（需管理员权限）
- **图片代理**：白名单代理，解决跨域图片加载，将前端无法展示的图片资源通过后端发送数据从而展示出来
- **Dashboard管理面板**：
  - 权限管理
  - 支持用户数据的增删改查
  - 统计、用户列表、文件列表、游戏数据
  - 绘制新增用户、新增文件以及新增文章的折线图
- **超级管理员终端**：
   - 终端UI界面交互展示
   - 自定义的指令功能
   - 支持超级管理员对系统资源的查询、管理
   - 支持指令的执行终端以及多用户指令
-  **补充模块**：
   - 使用自定义的登录和错误处理中间件
   - 附有监控的goroutine协程并且加入统计文件和代码行数的的脚本
## 项目展示
### 项目入口界面
- 项目封面
- ![alt text](/static/pictures/cover.png)
- 数据统计脚本
- ![alt text](/static/pictures/stastics.png)
- 登录注册页面
- ![alt text](/static/pictures/login_register.png)

### 项目菜单选择界面
- 项目菜单选择界面
![alt text](/static/pictures/main_menu.png)
- 多功能程序中心菜单-包含6大功能
![alt text](/static/pictures/6app.png)
- 翻译界面
![alt text](/static/pictures/translator.png)
- 
### 论坛中心界面
- 论坛中心界面
![alt text](/static/pictures/article.png)
- 个人文章管理界面，包括收藏夹
![alt text](/static/pictures/collection.png)
- 文章详情界面(这里是用户的个人界面可以编辑与删除),用户本身可以评论、转发、收藏功能。
![alt text](/static/pictures/myarticle.png)
- 三种找迷宫地图的可视化展示-类BFS探索(简单难度)-Prim算法(中等难度)-经典打通墙面的DFS算法(困难难度)
  ![alt text](/static/pictures/map.png)
### 管理员的页面展示
- 管理控制台
  
- ![alt text](/static/pictures/dashboard.png)
- /help指令已查询当前终端支持哪些命令
![alt text](/static/pictures/terminal1.png)
- 文件的查询以及cpu使用率等查询指令
![alt text](/static/pictures/terminal2.png)
- 操作系统查询、echo、cat以及file等指令的支持
![alt text](/static/pictures/terminal3.png)


## 📂 项目结构

```
project/
├── assets/                         # 设计稿与素材资源
├── config/                         # 配置及初始化
│   ├── config.go                   # Viper 读取 + 统一初始化
│   ├── config.yaml                 # 默认配置（数据库、Redis、API、上传配额等）
│   ├── db.go                       # MySQL 初始化与迁移
│   ├── redis.go                    # Redis 客户端
│   └── info.md                     # 配置说明
├── controllers/                    # 业务控制器
│   ├── article_controller.go       # 文章 CRUD + 缓存 + 查询
│   ├── colletion_repost_controller.go # 收藏夹、转发、频控
│   ├── comments_likes_controller.go   # 点赞 / 评论树
│   ├── exchange_rate_controller.go    # 汇率 & 用户信息
│   ├── files_controller.go         # 文件上传 / 下载 / 列表
│   ├── game_*.go                   # 游戏逻辑与排行榜
│   ├── openAIapi.go                # 翻译服务接入层
│   ├── translator.go               # 翻译业务逻辑
│   ├── weather_location.go         # 定位 + 天气
│   └── ...                         # 其他控制器
├── docs/                           # Swag 自动生成的接口文档
├── files/                          # 用户上传文件存储根目录（按用户/日期划分）
├── global/                         # 全局句柄（DB、Redis、配置）
├── log/                            # 日志与文件监控
│   ├── logger.go
│   └── monitor.go
├── md_pictures/                    # README、文档插图
├── middlewares/                    # 中间件（日志、恢复、JWT、权限）
├── models/                         # GORM 数据模型
│   ├── article.go                  # 文章、评论、收藏关联
│   ├── exchange_rate.go
│   ├── file.go
│   ├── game.go
│   ├── translation_history.go
│   └── user.go
├── router/
│   ├── router.go                   # 主路由 & 页面
│   └── swagger.go                  # Swagger 映射
├── static/                         # 静态资源（CSS、图片、分享素材）
├── templates/                      # HTML 模板
│   ├── index.html                  # 首页
│   ├── login.html / register.html  # 认证页面
│   ├── articles_pages.html         # 文章列表
│   ├── article_detail.html         # 文章详情
│   ├── article_my_list.html        # 我的文章
│   ├── article_create.html / edit.html
│   ├── collections.html            # 收藏夹视图
│   ├── game_selection.html         # 游戏大厅
│   ├── game_guess_number.html
│   ├── game_map_time.html
│   ├── map_display.html
│   ├── game_2048.html
│   ├── game_leaderboards.html
│   ├── translator.html / translator_history.html
│   ├── weather.html
│   ├── exchange_rates.html / rmb_top10.html
│   ├── upload.html / file_lists.html
│   ├── calculator.html
│   ├── dashboard.html
│   └── shell.html
├── utils/                         # 工具函数（JWT、密码、路径等）
├── Reference_learning_documents/  # 学习笔记与参考资料
├── info.md                        # 项目信息
├── main.go                        # 应用入口
├── go.mod / go.sum
└── README.md
```

> **注**:📌 `files/` 目录用于持久化上传文件，请确保运行环境具备写权限；若目录不存在，应用会自动创建。

## ⚙️ 配置说明

所有配置集中在 `config/config.yaml`，可根据实际环境覆盖：

```yaml
app:
  name: Go-Web            # 应用名称
  port: :3000             # 应用监听端口（支持 :3000 / 3000 两种写法）

database:
  dsn: root:123456@tcp(127.0.0.1:13306)/test?charset=utf8mb4&parseTime=True&loc=Local # 数据库dsn，依据自己的数据库配置
  MaxIdleConns: 11
  MaxOpenConns: 114
  ConnMaxLifetimeHours: 1

redis:
  addr: localhost:6379
  DB: 0
  Password: "" # redis密码自己可配置

superadmin:
  username: superadmin
  password: admin123456

local_api:
  baseURL: "https://restapi.amap.com/v3/ip" # 定位API可以自定义更改
  apiKey: "" # 密钥
  LocationDailyLimit: 100

translation_api:
  provider: "Kimi K2"
  apiKey: "" # 密钥
  baseURL: "https://api.moonshot.cn/v1"  # 模型API可以自定义更改
  model: "moonshot-v1-8k"

upload:
  totalSize: 500          # 单用户总容量上限（MB）
  fileSize: 50            # 单文件大小上限（MB）
  storagepath: "files"    # 相对存储目录
```

> 这里建议用户将敏感信息（数据库、API Key）通过环境变量或 CI/CD Secret 注入。

## 🚀 快速开始

1. 克隆并进入项目
   ```bash
   git clone <repository-url>
   cd project
   ```
2. 安装依赖
   ```bash
   go mod download
   ```
3. 准备配置  
   - 按需修改 `config/config.yaml`  
   - 确保 MySQL、Redis 服务已就绪  
   - 同步创建数据库（表会在启动时自动迁移）
4. （可选）重新生成 Swagger 文档
   ```bash
   go install github.com/swaggo/swag/cmd/swag@latest
   swag init
   ```
5. 启动应用
   ```bash
   go run main.go
   ```
6. 访问入口  
   - 页面入口：`http://localhost:3000`  
   - Swagger：`http://localhost:3000/swagger/index.html`

## 📜 页面路径速查

- 首页：`/`
- 认证：`/auth/login`、`/auth/register`、`/auth/logout`
- 受保护页面（需登录）：`/page/*`（文章、收藏夹、游戏、天气、翻译、文件、计算器）
- 管理后台：`/admin/dashboard`、`/admin/users`
- 超管终端：`/admin/superadmin/terminal`

## 📡 API 速查

### 认证 / 用户
- `POST /api/auth/register` 用户注册
- `POST /api/auth/login` 用户登录
- `POST /api/auth/logout` 用户登出
- `GET /api/me` 当前登录用户信息
- `GET /api/ad` 作者博客宣传位

### 文章与互动
- `GET /api/articles` 文章列表，支持分页/排序/搜索
- `POST /api/create_articles` 创建文章
- `PUT /api/update_articles/:id` 更新文章
- `DELETE /api/articles/:id` 删除文章（级联清理评论/点赞/收藏缓存）
- `GET /api/articles/me` 我的文章管理列表
- `POST /api/articles/:article_id/like` 点赞 / 取消点赞
- `GET /api/articles/:id/comments` 获取文章评论树
- `POST /api/comments` 发表评论或回复

### 收藏夹
- `POST /api/collections` 新建收藏夹（带频控）
- `GET /api/collections/all` 我的收藏夹列表
- `GET /api/collections/all_items` 收藏夹 + 文章明细
- `POST /api/collections/item` 将文章加入收藏夹
- `DELETE /api/collections/item` 从收藏夹移除文章
- `DELETE /api/collections/:collectionId` 删除收藏夹（自动维护计数）

### 文件中心
- `POST /api/files/upload` 上传文件（multipart/form-data）
- `GET /api/files/:id` 下载 / 预览文件（支持 `download=1`）
- `DELETE /api/files/:id` 删除文件
- `GET /api/files/lists` 文件列表，支持多条件筛选

### 游戏中心
- `POST /api/game/guess` 猜数字提交
- `POST /api/game/reset` 猜数字重置
- `POST /api/game/map/start` 地图游戏开始
- `POST /api/game/map/complete` 地图游戏完成
- `POST /api/game/map/reset` 地图游戏重置
- `GET /api/game/map/display` 地图可视化数据
- `POST /api/game/2048/save` 保存 2048 分数
- `GET /api/game/leaderboards` 全部排行榜
- `GET /api/game/leaderboard/me` 我的各游戏成绩

### 天气 & 定位
- `GET /api/weather/info` 当前定位天气
- `GET /api/weather/top10` 热门城市 TOP10

### 汇率
- `GET /api/exchangeRates` 汇率列表
- `POST /api/exchangeRates` 新增汇率记录
- `PUT /api/exchangeRates/:id` 更新汇率
- `DELETE /api/exchangeRates/:id` 删除汇率
- `GET /api/rmb-top10` CNY Top10 快照
- `POST /api/rmb-top10/refresh` 手动刷新榜单

### 翻译
- `POST /api/translate` 翻译文本
- `GET /api/translate/languages` 支持语言列表
- `GET /api/translate/history` 翻译历史
- `DELETE /api/translate/history/:id` 删除单条历史
- `DELETE /api/translate/history` 清空历史

### 工具 & 其他
- `POST /api/calculator/calculate` 在线计算器
- `GET /api/proxy/image` 图片代理服务

> 所有 `/api/**` 接口默认受 JWT 保护，需在请求头携带 `Authorization: Bearer <token>`。

### 管理员接口
- `GET /api/dashboard/total` 仪表盘总览数据
- `GET /api/dashboard/add` 今日新增统计
- `POST /api/dashboard/curve` 近期间隔曲线数据
- `GET /api/dashboard/time/sse` 后台时间信息（SSE）
- `GET /api/dashboard/users` 用户列表
- `POST /api/dashboard/user` 新增用户
- `PUT /api/dashboard/user/:id` 更新用户
- `DELETE /api/dashboard/user/:id` 删除用户

### 终端（超级管理员）
- `GET /api/superadmin/terminal` WebSocket 终端
- `GET /api/superadmin/terminal/info` 终端能力信息
- `GET /api/ws/terminal` WebSocket 终端（登录用户可用，白名单命令）
- `GET /api/ws/terminal/info` 终端能力信息（登录用户）

## 🧠 数据与缓存设计

- **Redis**  
  - 游戏排行榜：`game:*` ZSET
  - 文章首页缓存：`articles:list:homepage:default`
  - 点赞缓存：`articles:{id}:likes`、`articles:{id}:user:{uid}:like`
  - 收藏计数、评论频控、预留转发缓存：`collection:*`、`comment:rate:*`、`repost:*`
  - 汇率快照：`rmb_top10:cny`
- **缓存策略**  
  - 常规 TTL：2h/12h；文章存在性缓存 24h
  - 缓存穿透保护：命中空值写 `0`
  - 频控：评论 / 收藏夹 / 转发均设置 3 秒窗口
  - 分布式锁：排行榜刷新使用短 TTL 锁

## 🔒 安全机制

- JWT + 角色权限控制
- bcrypt 存储密码
- Redis 频控防刷
- 上传文件类型、MIME、体积三重校验
- SafeJoin 防止文件路径穿越
- Swagger Token 注入（`Authorize -> Bearer`）
- GORM 参数化查询防止 SQL 注入
- 图片代理白名单，避免 SSRF

## 📈 性能与监控

- Gin Release 模式（生产环境）
- MySQL / Redis 连接池与超时配置
- 首页、排行榜、天气、汇率等热点数据缓存
- Zap 结构化日志 + `tail -f log/app.log`
- `log.NewMonitor()` 监听文件系统变化
- 页面化 Dashboard 与 Shell（仅管理员可访问）
- 自定义的Terminal 终端
  - 支持各种命令执行、可输入/help进行查看
  - 实时显示命令输出，支持滚动查看
  - 支持多用户同时操作，每个用户有独立的会话环境

## 默认账号

- 超级管理员：账号:`superadmin `密码:`admin123456`  
  登录后可访问 `/admin/dashboard`、`/page/shell` 等受限功能。

## 开发日志
- 这里使用自定义的日志中间件Login，初始化文件为logger.go
- 开发模式初始化日志：`log.Init(false)`
- 生产模式初始化日志：`log.Init(true)`
- 常用调试命令：
  ```bash
  go test ./...
  swag fmt       # 格式化注释（生成前可选）
  tail -f log/app.log
  ```

## ❓ 常见问题

- Swagger 页面无法访问：确保已执行 `swag init`，并访问 `http://localhost:3000/swagger/index.html`。
- `files/` 无写入权限：确保运行用户对项目目录有写权限；应用会自动创建目录。
- 数据库连接失败：检查 `config/config.yaml` 中 `database.dsn` 与数据库端口（示例 `127.0.0.1:13306`）。
- Redis 未启动：确认 `redis.addr` 指向可用实例，例如 `localhost:6379`。
- 接口返回 401：需先登录并在请求头设置 `Authorization: Bearer <token>`。
- 终端命令不执行：仅允许白名单内命令（如 `uptime`、`ls`、`free`、`df` 等）。

## 📄 许可证与贡献

- 授权协议：MIT License
- 欢迎 Issue / PR：
  1. Fork 仓库
  2. `git checkout -b feature/awesome`
  3. 编写代码并补充文档 / 注释
  4. `git commit -m "feat: add awesome feature"`
  5. `git push origin feature/awesome`
  6. 提交 PR

## 👨‍💻 作者 & 联系方式

- 作者：soul-XuYang
- 类型：学习各个框架 / 以及构建相关综合项目
- 年份：2025
- [邮箱](610415432@qq.com)
- [GitHub](https://github.com/Soul-XuYang/go-web)

---

<div align="center">

**如果这个项目对您有帮助，欢迎 ⭐ Star 支持！**

Made with ❤️ by Soul-XuYang

</div>
