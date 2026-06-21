# Smarticky Notes

一个基于 Go 的现代便签应用，界面参考锤子便签设计。

A modern notes application built with Go, inspired by Smartisan Notes.

## 功能特性 / Features

- ✅ **便签管理** / Note Management
  - 创建、编辑、删除便签
  - 支持 Markdown 格式
  - 收藏功能
  - 废纸篓（软删除）
  - 搜索功能
  - **便签颜色标签**（黄、绿、蓝、粉、紫）
  - **便签排序**（按时间、标题、颜色）
  - **导出功能**（导出为 Markdown）

- 🌍 **多语言支持** / Multi-language Support
  - 中文 (Chinese)
  - English
  - 自动语言检测

- 🎨 **主题** / Themes
  - 明亮主题
  - 暗色主题
  - 本地存储主题偏好

- ⌨️ **快捷键** / Keyboard Shortcuts
  - `Ctrl/Cmd + N` - 新建便签
  - `Ctrl/Cmd + F` - 聚焦搜索
  - `Ctrl/Cmd + S` - 切换收藏
  - `Ctrl/Cmd + D` - 切换主题
  - `Esc` - 关闭模态框

- 🎨 **图标系统** / Icon System
  - 使用 Feather Icons 开源图标库
  - 本地部署，支持离线使用
  - 轻量级、美观现代
  - 深色模式适配

- ☁️ **远程备份** / Remote Backup
  - **数据库文件备份** - 直接备份 SQLite 数据库文件
  - **配置存储在数据库中** - 备份配置可通过 UI 管理
  - WebDAV 支持（备份和恢复）
  - S3 兼容存储（备份和恢复）
  - 自动备份计划（每日/每周）
  - **图形化配置界面**

- 🔌 **MCP 接入** / MCP Access
  - Streamable HTTP endpoint: `/mcp`
  - 用户级 MCP Token，绑定当前笔记用户
  - 支持查询、搜索、读取、创建笔记
  - 支持将笔记生成 PNG 长图
  - LazyCat 微服内支持应用间委托访问

- 🐳 **Docker 部署** / Docker Deployment
  - 支持 Docker 容器化部署
  - Docker Compose 一键启动

## 技术栈 / Tech Stack

- **后端 Backend**: Go 1.23+
  - Echo (Web Framework)
  - Ent (ORM)
  - SQLite (Database)

- **前端 Frontend**: Vanilla JavaScript
  - HTML5
  - CSS3
  - 无框架依赖

## 快速开始 / Quick Start

### 使用 Docker Compose (推荐)

```bash
# 克隆仓库
git clone <your-repo-url>
cd smarticky

# 启动服务
docker-compose up -d

# 访问应用
# http://localhost:8080
```

### 本地运行

```bash
# 安装依赖
go mod download

# 生成 Ent 代码
go generate ./ent

# 编译
go build -o smarticky ./cmd/server

# 运行
./smarticky
```

## 环境变量 / Environment Variables

```bash
# 服务端口 / Server Port
PORT=8080

# 数据目录 / Data directory
SMARTICKY_DATA_DIR=./data

# 首次空库启动时创建管理员 / Create first admin only when the user table is empty
SMARTICKY_ADMIN_USERNAME=admin
SMARTICKY_ADMIN_PASSWORD=change-me-now
SMARTICKY_ADMIN_EMAIL=admin@example.com
SMARTICKY_ADMIN_NICKNAME=Owner

# LazyCat 环境下才开启：信任 X-HC-User-ID / X-HC-SOURCE 转发身份
SMARTICKY_TRUST_LAZYCAT_HEADERS=false

# 可选：后端生图字体路径
SMARTICKY_SHARE_FONT=/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc
```

`SMARTICKY_ADMIN_*` 只在数据库没有任何用户时生效；已有用户后不会再次创建或覆盖管理员账号。

**注意**: WebDAV 和 S3 备份配置现在存储在数据库中，可通过 UI 或 API 管理，不再使用环境变量。

**Note**: WebDAV and S3 backup configurations are now stored in the database and can be managed through the UI or API, no longer using environment variables.

## Docker 构建 / Docker Build

```bash
# 构建镜像
docker build -t smarticky:latest .

# 运行容器
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -e PORT=8080 \
  smarticky:latest
```

## 项目结构 / Project Structure

```
smarticky/
├── cmd/
│   └── server/          # 主程序入口
├── ent/
│   └── schema/          # 数据库模型定义
├── internal/
│   ├── handler/         # HTTP 处理程序
│   └── service/         # 业务逻辑
├── web/
│   ├── static/          # 静态资源
│   │   ├── css/
│   │   └── js/
│   └── templates/       # HTML 模板
├── Dockerfile
├── docker-compose.yml
└── README.md
```

## API 文档 / API Documentation

### Notes API

- `GET /api/notes` - 获取便签列表
  - Query: `starred=true|false`, `trash=true|false`, `q=search`
- `POST /api/notes` - 创建便签
- `GET /api/notes/:id` - 获取单个便签
- `PUT /api/notes/:id` - 更新便签
- `DELETE /api/notes/:id` - 永久删除便签

### Backup Configuration API

- `GET /api/backup/config` - 获取备份配置 / Get backup configuration
- `PUT /api/backup/config` - 更新备份配置 / Update backup configuration

### Backup & Restore API

- `POST /api/backup/webdav` - 备份数据库到 WebDAV / Backup database to WebDAV
- `POST /api/backup/s3` - 备份数据库到 S3 / Backup database to S3
- `POST /api/restore/webdav` - 从 WebDAV 恢复数据库 / Restore database from WebDAV
- `POST /api/restore/s3` - 从 S3 恢复数据库 / Restore database from S3

**备份说明 / Backup Notes**:
- 备份的是完整的 SQLite 数据库文件 (.db)
- 恢复前会自动备份当前数据库
- Backups are full SQLite database files (.db)
- Current database is automatically backed up before restore

### MCP

- `POST /mcp` - MCP Streamable HTTP endpoint
- `GET /api/mcp/tokens` - 列出当前用户的 MCP token
- `POST /api/mcp/tokens` - 创建当前用户的 MCP token，明文只返回一次
- `DELETE /api/mcp/tokens/:id` - 撤销当前用户的 MCP token
- `GET /api/mcp/images/:id` - 下载当前用户生成的 MCP 笔记图片

MCP tools:

- `smarticky_list_notes`
- `smarticky_search_notes`
- `smarticky_get_note`
- `smarticky_create_note`
- `smarticky_generate_note_image`

锁定笔记不会通过 MCP 返回正文，也不能通过 MCP 生成图片。

LazyCat LPK exports `resources/mcp-providers/smarticky/mcp.yml` with `endpoint: /mcp`.
For LazyCat delegated access, set the matching LazyCat user ID in the user's profile settings first.

## 开发 / Development

```bash
# 监听文件变化并自动重启
go install github.com/cosmtrek/air@latest
air

# 运行测试
go test ./...

# 代码格式化
go fmt ./...
```

## 贡献 / Contributing

欢迎提交 Issue 和 Pull Request!

Contributions are welcome! Please feel free to submit a Pull Request.

## 许可 / License

MIT License

## 致谢 / Credits

界面设计参考了锤子便签的优秀设计理念。

UI design inspired by Smartisan Notes.
