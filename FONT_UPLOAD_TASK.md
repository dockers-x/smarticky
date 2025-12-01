# 字体上传功能开发任务

## 功能概述
允许用户上传自定义字体文件，所有用户都可以在编辑器中使用这些字体。字体列表显示上传者信息和预览。

## 技术栈
- 后端：Go + Echo + Ent ORM
- 数据库：SQLite
- 前端：Vanilla JavaScript
- 编辑器：Milkdown

---

## 任务列表

### 1. 数据库设计 ⏳
**文件**: `ent/schema/font.go`

**字段设计**:
- `id`: UUID (主键)
- `name`: string (字体名称)
- `display_name`: string (显示名称)
- `file_path`: string (文件存储路径)
- `file_size`: int64 (文件大小)
- `format`: string (字体格式: ttf, otf, woff, woff2)
- `preview_text`: string (预览文本，默认: "字体预览 Font Preview")
- `uploaded_by`: edge -> User (上传者关系)
- `created_at`: time.Time (上传时间)

**状态**: ⏳ 进行中

---

### 2. 生成 Ent 代码 ⏳
**命令**: `go generate ./ent`

**操作**:
- 生成 Font 实体的 CRUD 代码
- 自动创建数据库迁移

**状态**: ⏳ 待开始

---

### 3. 后端 API 实现 ⏳
**文件**: `internal/handler/font.go`

**API 端点**:
```
POST   /api/fonts              - 上传字体（需登录）
GET    /api/fonts              - 获取字体列表
GET    /api/fonts/:id/download - 下载字体文件
DELETE /api/fonts/:id          - 删除字体（仅上传者和管理员）
```

**功能**:
- 字体文件验证（格式、大小限制）
- 文件存储到 `data/uploads/fonts/`
- 返回字体信息（包含上传者用户名）

**状态**: ⏳ 待开始

---

### 4. 路由配置 ⏳
**文件**: `cmd/server/main.go`

**路由添加**:
```go
protected.POST("/fonts", h.UploadFont)
protected.GET("/fonts", h.GetFonts)
protected.GET("/fonts/:id/download", h.DownloadFont)
protected.DELETE("/fonts/:id", h.DeleteFont)
```

**状态**: ⏳ 待开始

---

### 5. 前端 UI 设计 ⏳
**文件**: `web/templates/index.html`

**UI 元素**:
- 侧边栏添加"字体管理"按钮
- 字体管理模态对话框
  - 字体列表（表格形式）
    - 字体名称
    - 预览（实时渲染）
    - 上传者
    - 上传时间
    - 操作按钮（删除）
  - 上传按钮和文件选择器
  - 实时预览上传的字体

**状态**: ⏳ 待开始

---

### 6. 前端逻辑实现 ⏳
**文件**: `web/static/js/app.js`

**功能模块**:
1. **字体上传**
   - 文件选择和验证
   - 上传进度显示
   - 实时预览（使用 FontFace API）

2. **字体列表**
   - 加载和渲染字体列表
   - 显示上传者信息
   - 每个字体显示预览（动态加载字体）

3. **字体删除**
   - 权限检查（仅上传者和管理员）
   - 确认对话框

4. **字体选择器**
   - 在编辑器工具栏添加字体下拉菜单
   - 列出系统字体 + 自定义字体
   - 显示上传者信息

**状态**: ⏳ 待开始

---

### 7. 编辑器集成 ⏳
**文件**: `web/static/js/milkdown-editor.js` 或 `app.js`

**功能**:
1. 动态加载用户上传的字体
2. 应用字体到编辑器
3. 字体设置持久化（localStorage）
4. 编辑器重载时恢复字体设置

**实现方式**:
```javascript
// 使用 CSS FontFace API 动态加载
const fontFace = new FontFace('CustomFont', 'url(/api/fonts/xxx/download)');
await fontFace.load();
document.fonts.add(fontFace);

// 应用到编辑器
editor.style.fontFamily = 'CustomFont';
```

**状态**: ⏳ 待开始

---

### 8. CSS 样式 ⏳
**文件**: `web/static/css/custom.css` 或新建 `fonts.css`

**样式**:
- 字体管理模态对话框样式
- 字体预览卡片样式
- 字体选择器下拉菜单样式
- 上传进度条样式

**状态**: ⏳ 待开始

---

### 9. 国际化 (i18n) ⏳
**文件**: `web/static/js/i18n.json`

**新增翻译**:
```json
{
  "zh": {
    "fonts_management": "字体管理",
    "upload_font": "上传字体",
    "font_name": "字体名称",
    "uploaded_by": "上传者",
    "upload_time": "上传时间",
    "font_preview": "字体预览",
    "delete_font": "删除字体",
    "select_font": "选择字体"
  },
  "en": {
    "fonts_management": "Font Management",
    "upload_font": "Upload Font",
    "font_name": "Font Name",
    "uploaded_by": "Uploaded By",
    "upload_time": "Upload Time",
    "font_preview": "Font Preview",
    "delete_font": "Delete Font",
    "select_font": "Select Font"
  }
}
```

**状态**: ⏳ 待开始

---

### 10. 测试 ⏳

**测试项**:
- [ ] 上传不同格式的字体（.ttf, .otf, .woff, .woff2）
- [ ] 字体预览正确显示
- [ ] 字体应用到编辑器
- [ ] 上传者信息正确显示
- [ ] 删除权限控制（仅上传者和管理员）
- [ ] 多用户场景测试
- [ ] 大文件上传（文件大小限制）
- [ ] 非法文件格式拦截
- [ ] 字体持久化（刷新页面后仍然可用）

**状态**: ⏳ 待开始

---

## 技术细节

### 字体格式支持
- **.ttf** (TrueType Font) - 最常见
- **.otf** (OpenType Font) - 支持更多特性
- **.woff** (Web Open Font Format) - Web 优化
- **.woff2** (Web Open Font Format 2.0) - 更好的压缩

### 文件大小限制
建议限制: **10MB** (防止滥用)

### 存储结构
```
data/uploads/fonts/
├── 550e8400-e29b-41d4-a716-446655440000.ttf
├── 6ba7b810-9dad-11d1-80b4-00c04fd430c8.woff2
└── ...
```

### 前端字体加载流程
1. 页面加载时，获取字体列表 API
2. 使用 FontFace API 动态加载字体
3. 添加到 `document.fonts`
4. 更新编辑器字体选择器

### 权限控制
- 所有登录用户可以上传字体
- 所有用户可以使用任何字体
- 仅上传者和管理员可以删除字体

---

## 进度追踪

- [ ] 任务 1: 数据库设计
- [ ] 任务 2: 生成 Ent 代码
- [ ] 任务 3: 后端 API 实现
- [ ] 任务 4: 路由配置
- [ ] 任务 5: 前端 UI 设计
- [ ] 任务 6: 前端逻辑实现
- [ ] 任务 7: 编辑器集成
- [ ] 任务 8: CSS 样式
- [ ] 任务 9: 国际化
- [ ] 任务 10: 测试

---

## 预计完成时间
总计: **约 4-6 小时**

## 备注
- 字体文件需要进行格式验证，防止上传恶意文件
- 考虑添加字体预览文本自定义功能
- 未来可以添加字体分类、标签功能
- 考虑字体使用统计（哪些字体最受欢迎）
