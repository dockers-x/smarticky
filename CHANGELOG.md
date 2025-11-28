# 更新日志 / CHANGELOG

## v1.1.0 - 2025-01-28

### ✨ 新功能 / New Features

#### 1. **图标系统改进 / Icon System Enhancement**
- 使用锤子便签原版图标 sprite
- 实现 CSS Sprite 技术优化图标加载
- 图标位置：`/static/img/all_icons_ab3d0991b9.png`
- 支持深色模式下的图标适配

#### 2. **便签颜色功能 / Note Color Feature**
- 为每个便签添加颜色标签功能
- 支持 6 种颜色：默认、黄色、绿色、蓝色、粉色、紫色
- 颜色会应用到便签列表项和编辑器背景
- 添加颜色选择器 UI
- 数据库新增 `color` 字段

#### 3. **便签排序功能 / Note Sorting**
- 按最后修改时间排序（最新/最旧）
- 按标题排序（A-Z / Z-A）
- 按颜色排序
- 排序偏好保存到 localStorage
- 新增排序菜单 UI

#### 4. **导出功能 / Export Feature**
- 支持导出单个便签为 Markdown 文件
- 导出格式：标题 + 内容
- 文件名自动使用便签标题

#### 5. **备份配置界面 / Backup Configuration UI**
- 新增图形化备份配置界面
- WebDAV 和 S3 配置分标签页管理
- 支持在界面中保存配置
- 支持在界面中执行备份和恢复操作
- 替代原来的环境变量配置方式

#### 6. **快捷键支持 / Keyboard Shortcuts**
- `Ctrl/Cmd + N` - 新建便签
- `Ctrl/Cmd + F` - 聚焦搜索框
- `Ctrl/Cmd + S` - 切换当前便签收藏状态
- `Ctrl/Cmd + D` - 切换深色/明亮主题
- `Esc` - 关闭所有打开的模态框

#### 7. **UI/UX 改进 / UI/UX Improvements**
- 添加平滑的动画效果（淡入、滑动）
- 改进模态框交互体验
- 优化搜索框样式，添加搜索图标
- 改进按钮 hover 和 active 状态
- 优化侧边栏布局和图标显示
- 空状态提示优化

### 🔧 技术改进 / Technical Improvements

#### 前端 / Frontend
- **新文件**:
  - `/static/css/icons.css` - 图标样式系统
  - `/static/img/` - 图标资源目录
- **重写**: `/static/js/app.js` - 完整重构，新增所有功能
- **更新**:
  - `/templates/index.html` - 新增模态框和 UI 组件
  - `/static/css/custom.css` - 新增大量样式
  - `/static/js/i18n.json` - 新增翻译键

#### 后端 / Backend
- **更新**: `/ent/schema/note.go` - 新增 `color` 字段
- **重新生成**: ent 代码以支持新字段

### 📦 文件结构变化 / File Structure Changes

```
web/static/
├── css/
│   ├── custom.css        (更新 - 新增大量样式)
│   ├── icons.css         (新建 - 图标系统)
│   ├── note.css          (保持)
│   └── style.css         (保持)
├── js/
│   ├── app.js            (完全重写)
│   ├── i18n.json         (更新 - 新增翻译)
│   └── icons.js          (废弃 - 改用 CSS Sprite)
└── img/                  (新建目录)
    ├── all_icons_ab3d0991b9.png
    ├── edge_004e88bdf2.png
    ├── folder_icon_dc8a8d7563.png
    ├── grid_6e4a41eefc.png
    └── note_blank_icon_ac2a0a264f.png
```

### 🎨 设计改进 / Design Improvements
- 更接近锤子便签的视觉风格
- 使用原版图标增强一致性
- 改进深色模式的视觉效果
- 优化颜色搭配和对比度
- 添加微妙的动画效果提升用户体验

### 🌐 国际化改进 / i18n Improvements
新增翻译键：
- `no_notes` - 无便签提示
- `start_writing` - 编辑器占位符
- `toggle_star` - 收藏按钮提示
- `change_color` - 颜色按钮提示
- `export` - 导出按钮
- `restore_confirm` - 恢复确认
- `restore_success` / `restore_failed` - 恢复状态
- `save_success` / `save_failed` - 保存状态

### 📝 使用说明 / Usage Notes

#### 便签颜色
1. 选择一个便签
2. 点击工具栏中的设置图标
3. 在颜色选择器中选择颜色
4. 便签列表和编辑器背景会自动更新

#### 排序功能
1. 点击侧边栏底部的"更多"图标
2. 选择排序方式
3. 排序偏好会自动保存

#### 备份配置
1. 点击侧边栏底部的"分享"图标
2. 切换到 WebDAV 或 S3 标签
3. 填写配置信息并保存
4. 使用"立即备份"或"恢复"按钮

#### 导出便签
1. 选择要导出的便签
2. 点击工具栏中的"分享"图标
3. 便签将导出为 Markdown 文件

### 🐛 修复 / Bug Fixes
- 修复便签更新时的抖动问题
- 优化搜索框响应速度
- 修复深色模式下的样式问题
- 改进模态框的关闭逻辑

### ⚠️ 破坏性变更 / Breaking Changes
无

### 🔄 迁移指南 / Migration Guide
1. 运行 `go generate ./ent` 重新生成数据库代码
2. 重新编译：`go build -o smarticky ./cmd/server`
3. 启动服务，现有数据库会自动添加 `color` 字段
4. 享受新功能！

---

## v1.0.0 - 初始版本 / Initial Release
基础便签功能、备份系统、多语言支持
