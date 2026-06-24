# Smarticky Notes

Smarticky Notes 是一个自托管便签和轻量知识库，适合把零散想法、会议记录、代码片段、流程图、白板草图和待办整理在一个地方。它的界面参考锤子便签的纸张感，但功能更偏向长期使用：能写、能找、能备份，也能和外部笔记工具、AI 工具连接。

## 它能解决什么问题

很多笔记软件要么太重，要么只适合纯文本。Smarticky Notes 更适合这些场景：

- 临时想法很多，需要快速写下来，并且之后还能按标题、内容、颜色、标签或文件夹找回。
- 会议纪要、方案草稿、学习笔记里经常混合 Markdown、代码块、数学公式和图表。
- 想把流程图、时序图、draw.io 图、白板草图和文字笔记放在一起，而不是分散在几个工具里。
- 不想把私人笔记托管到第三方服务，希望数据留在自己的服务器、NAS 或 LazyCat 环境里。
- 已经在用 Evernote、Notion、Joplin 或思源笔记，希望能逐步迁移或把关键笔记同步到 Smarticky。
- 想让 AI 客户端通过 MCP 读取、搜索或创建自己的笔记，但又需要按用户和权限隔离。

## 主要功能

### 写作和整理

- 创建、编辑、收藏、删除和恢复便签，误删内容可以先进入废纸篓。
- 支持文件夹、标签、颜色和排序，适合把工作、学习、灵感、项目资料分开管理。
- 支持全文搜索，能在大量便签里快速定位标题和正文。
- 自动保存编辑内容，减少忘记保存导致的丢失。
- 支持明亮和深色主题，中英文界面会根据浏览器语言自动选择，也可以手动切换。

### Markdown 编辑体验

- 使用 Markdown 编辑器记录结构化内容，支持标题、列表、表格、链接、图片、代码块和数学公式。
- 代码块支持常见编程语言语法高亮，也支持 Mermaid、drawio 和 Excalidraw 白板引用。
- 支持代码组，适合在一条笔记里同时展示 npm、pnpm、yarn 等不同命令。
- 可以把单条便签导出为 Markdown 文件，方便归档或迁移。

### 图表和白板

- Mermaid 图表可以直接写在代码块里，包括 flowchart、sequenceDiagram、classDiagram、erDiagram、gantt、mindmap 等类型。
- draw.io XML 可以直接放进 `drawio` 代码块，预览、分享图和浏览器生成图片时会渲染为图形。
- 内置 Excalidraw 白板，适合画草图、架构图、流程草稿和临时视觉说明。
- 便签可以生成 PNG 长图，适合分享会议纪要、清单或图文笔记。

### 隐私保护

- 单条便签可以设置访问密码，也可以启用加密模式。
- 加密模式使用浏览器端 AES-GCM 加密，密钥由密码通过 PBKDF2 派生，服务端只保存密文。
- 被保护的笔记会在列表、搜索、MCP 和图片生成等入口做正文隐藏，避免无意泄露。

### 导入、同步和备份

- 支持 Evernote ENEX 导入，可以预览笔记本、笔记数量、标签和资源数量后再确认导入。
- 支持连接思源笔记、Notion 和 Joplin，可以从远端目标导入笔记，也可以把当前便签推送回连接的服务。
- 支持 WebDAV 和 S3 兼容存储备份，备份配置在界面里管理。
- 支持手动备份、恢复和自动备份计划，恢复前会自动保留当前数据库副本。

### 多用户和 AI 接入

- 支持用户账户、头像、分享签名和管理员管理，适合家庭、小团队或个人多设备使用。
- 每个用户可以创建自己的 MCP Token，AI 客户端通过 `/mcp` 访问时只看到当前用户有权限访问的笔记。
- MCP 支持查询、搜索、读取、创建笔记，也支持生成笔记长图。受保护笔记不会通过 MCP 返回正文。
- LazyCat LPK 内置 MCP provider 配置，便于在 LazyCat/LightOS 环境里委托访问。

## 图表代码块示例

Mermaid 支持标准 `mermaid` 围栏，也可以直接使用具体类型。选择具体类型时，可以只写主体内容，应用会补齐 Mermaid 类型声明。

````markdown
```flowchart
A->B
B->C
```

```sequenceDiagram
Alice->>Bob: Hello
Bob-->>Alice: Hi
```
````

draw.io 文件通常就是 XML，可以把内容粘到 `drawio` 代码块里。

````markdown
```drawio
<mxfile>
  <diagram name="Page-1">
    <mxGraphModel>
      <root>
        <mxCell id="0"/>
        <mxCell id="1" parent="0"/>
        <mxCell id="2" value="Hello" style="rounded=1;whiteSpace=wrap;html=1;" vertex="1" parent="1">
          <mxGeometry x="80" y="80" width="120" height="60" as="geometry"/>
        </mxCell>
      </root>
    </mxGraphModel>
  </diagram>
</mxfile>
```
````

## 技术栈

Smarticky Notes 后端使用 Go、Echo、Ent 和 SQLite，适合单机部署和容器化运行；前端使用 Svelte 构建应用界面，编辑器基于 Milkdown 和 CodeMirror，并集成 Mermaid、drawio 渲染、Excalidraw 白板、KaTeX 公式和浏览器端加密能力。项目提供 Docker 镜像构建、GitHub Release 二进制产物和 LazyCat MCP provider 配置。

## 快速开始

### Docker Compose

```bash
git clone <your-repo-url>
cd smarticky
docker-compose up -d
```

启动后访问：

```text
http://localhost:8080
```

### 本地运行

```bash
go mod download
go generate ./ent
go build -o smarticky ./cmd/server
./smarticky
```

## 常用配置

```bash
# 服务端口
PORT=8080

# 数据目录
SMARTICKY_DATA_DIR=./data

# 首次空库启动时创建管理员，已有用户后不会再次创建或覆盖
SMARTICKY_ADMIN_USERNAME=admin
SMARTICKY_ADMIN_PASSWORD=change-me-now
SMARTICKY_ADMIN_EMAIL=admin@example.com
SMARTICKY_ADMIN_NICKNAME=Owner

# LazyCat 环境下才开启，信任 X-HC-User-ID / X-HC-SOURCE 转发身份
SMARTICKY_TRUST_LAZYCAT_HEADERS=false

# 可选，后端生成分享图片时使用的字体
SMARTICKY_SHARE_FONT=/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc
```

WebDAV 和 S3 备份配置在应用设置里管理，不再通过环境变量配置。

## 常用快捷键

| 快捷键 | 作用 |
| --- | --- |
| `Ctrl/Cmd + N` | 新建便签 |
| `Ctrl/Cmd + F` | 聚焦搜索 |
| `Ctrl/Cmd + S` | 切换收藏 |
| `Ctrl/Cmd + D` | 切换主题 |
| `Esc` | 关闭弹窗 |

## 开发

```bash
# 后端测试
go test ./...

# 前端检查、测试和构建
cd web/app
npm run check
npm test
npm run build
```

## 贡献

欢迎提交 Issue 和 Pull Request。功能建议、导入兼容性问题、图表渲染问题和部署反馈都可以直接提。

## 许可

MIT License
