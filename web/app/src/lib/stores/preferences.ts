import { get, writable } from "svelte/store";

export type Language = "zh" | "en";
export type Theme = "light" | "dark";

interface PreferencesState {
  language: Language;
  theme: Theme;
}

const messages = {
  zh: {
    add: "添加",
    addAttachment: "添加附件",
    addTag: "添加标签",
    addTagFailed: "添加标签失败",
    allNotes: "全部笔记",
    attachments: "附件",
    backup: "备份",
    backupComing: "备份工具将在设置面板中接入",
    bold: "加粗",
    back: "返回",
    cancel: "取消",
    closeSettings: "关闭设置面板",
    closeShareImage: "关闭生成图片",
    contentEmpty: "没有正文",
    continueImport: "继续导入",
    copyImage: "复制图片",
    copyImageFailed: "复制图片失败，请下载后分享",
    copyImageUnsupported: "当前浏览器不支持复制图片",
    copiedImage: "分享图片已复制",
    darkTheme: "深色",
    delete: "删除",
    details: "详情",
    downloadPng: "下载 PNG",
    editor: "编辑器",
    emptyNoteList: "写下你的第一篇笔记",
    emptyNoteListSubtitle: "记录想法，沉淀思考",
    enterFocus: "专注",
    exitFocus: "退出",
    failed: "失败",
    fontManagement: "字体管理",
    fontManagementComing: "字体管理将在设置面板中接入",
    format: "版式",
    generatedImage: "分享图片已生成",
    generateImage: "生成图片",
    generateImageFailed: "生成图片失败",
    imageInsertAlt: "图片说明",
    imageLabel: "图",
    import: "导入",
    importCompleted: "导入完成",
    importCompletedPartial: "导入完成，部分条目失败",
    imported: "已导入",
    importing: "处理中",
    importFailed: "导入失败，请稍后重试",
    importStart: "开始导入",
    language: "语言",
    lightTheme: "浅色",
    loadNotesFailed: "无法加载笔记，请稍后重试",
    loadingNotes: "正在加载笔记",
    logout: "退出登录",
    logoutConfirm: "确认退出当前账号？",
    markdownToolbar: "Markdown 工具栏",
    newNote: "新建笔记",
    noAttachments: "暂无附件",
    noTags: "暂无标签",
    noteInfo: "笔记信息",
    noteList: "笔记列表",
    notes: "笔记",
    noteTitle: "笔记标题",
    insertImage: "插入图片",
    italic: "斜体",
    orderedList: "有序列表",
    personalProfile: "个人资料",
    personalProfileComing: "个人资料将在设置面板中接入",
    preparing: "正在准备笔记工作台",
    restore: "恢复",
    restoreFailed: "恢复失败",
    restoreNote: "恢复笔记",
    restoreNoteMessage: "确认将这篇笔记恢复到普通列表？",
    restoredNote: "已恢复笔记",
    saveError: "保存失败",
    saved: "已保存",
    saving: "正在保存",
    searchNotes: "搜索笔记",
    selectFile: "选择 .enex 文件",
    selectFileFirst: "请先选择 .enex 文件",
    selectImportFile: "选择 Evernote ENEX 文件",
    selectOrCreate: "选择一篇笔记，或新建一篇",
    settings: "设置",
    sessionExpired: "登录已过期",
    shareClassic: "白底经典",
    shareDialogLabel: "生成分享图片",
    shareNight: "夜读",
    sharePaper: "暖纸长文",
    shareSmartisanSubtitle: "Smartisan 风格排版",
    showDetails: "信息",
    skipped: "已跳过",
    squareImage: "方图",
    starred: "收藏",
    star: "收藏",
    starAdded: "已收藏",
    starRemoved: "已取消收藏",
    tags: "标签",
    task: "待办",
    theme: "主题",
    today: "今天",
    trash: "废纸篓",
    trashFailed: "移动失败",
    trashNote: "移入废纸篓",
    trashNoteMessage: "确认将这篇笔记移入废纸篓？",
    trashedNote: "已移入废纸篓",
    untitled: "未命名",
    uploadAttachmentFailed: "上传附件失败",
    unorderedList: "无序列表",
    userManagement: "用户管理",
    userManagementComing: "用户管理将在设置面板中接入",
    updateStarFailed: "更新收藏状态失败",
    warnings: "提示",
    wideImage: "长图",
    wordUnit: "字",
    yesterday: "昨天",
  },
  en: {
    add: "Add",
    addAttachment: "Add attachment",
    addTag: "Add tag",
    addTagFailed: "Failed to add tag",
    allNotes: "All notes",
    attachments: "Attachments",
    backup: "Backup",
    backupComing: "Backup will be connected in Settings",
    bold: "Bold",
    back: "Back",
    cancel: "Cancel",
    closeSettings: "Close settings panel",
    closeShareImage: "Close image generator",
    contentEmpty: "No body",
    continueImport: "Import another",
    copyImage: "Copy image",
    copyImageFailed: "Copy failed. Download the image instead",
    copyImageUnsupported: "This browser cannot copy images",
    copiedImage: "Image copied",
    darkTheme: "Dark",
    delete: "Delete",
    details: "Details",
    downloadPng: "Download PNG",
    editor: "Editor",
    emptyNoteList: "Write your first note",
    emptyNoteListSubtitle: "Capture thoughts and keep them clear",
    enterFocus: "Focus",
    exitFocus: "Exit",
    failed: "Failed",
    fontManagement: "Fonts",
    fontManagementComing: "Fonts will be connected in Settings",
    format: "Format",
    generatedImage: "Share image generated",
    generateImage: "Generate image",
    generateImageFailed: "Image generation failed",
    imageInsertAlt: "Image description",
    imageLabel: "Img",
    import: "Import",
    importCompleted: "Import completed",
    importCompletedPartial: "Import completed with some failures",
    imported: "Imported",
    importing: "Processing",
    importFailed: "Import failed. Try again later",
    importStart: "Start import",
    language: "Language",
    lightTheme: "Light",
    loadNotesFailed: "Could not load notes. Try again later",
    loadingNotes: "Loading notes",
    logout: "Log out",
    logoutConfirm: "Log out of the current account?",
    markdownToolbar: "Markdown toolbar",
    newNote: "New note",
    noAttachments: "No attachments",
    noTags: "No tags",
    noteInfo: "Note info",
    noteList: "Note list",
    notes: "Notes",
    noteTitle: "Note title",
    insertImage: "Insert image",
    italic: "Italic",
    orderedList: "Ordered list",
    personalProfile: "Profile",
    personalProfileComing: "Profile will be connected in Settings",
    preparing: "Preparing your notes workspace",
    restore: "Restore",
    restoreFailed: "Restore failed",
    restoreNote: "Restore note",
    restoreNoteMessage: "Restore this note to the main list?",
    restoredNote: "Note restored",
    saveError: "Save failed",
    saved: "Saved",
    saving: "Saving",
    searchNotes: "Search notes",
    selectFile: "Choose .enex file",
    selectFileFirst: "Choose an .enex file first",
    selectImportFile: "Choose Evernote ENEX file",
    selectOrCreate: "Select a note or create one",
    settings: "Settings",
    sessionExpired: "Session expired",
    shareClassic: "Classic white",
    shareDialogLabel: "Generate share image",
    shareNight: "Night reading",
    sharePaper: "Warm paper",
    shareSmartisanSubtitle: "Smartisan-style typography",
    showDetails: "Info",
    skipped: "Skipped",
    squareImage: "Square",
    starred: "Starred",
    star: "Star",
    starAdded: "Starred",
    starRemoved: "Unstarred",
    tags: "Tags",
    task: "Task",
    theme: "Theme",
    today: "Today",
    trash: "Trash",
    trashFailed: "Move failed",
    trashNote: "Move to trash",
    trashNoteMessage: "Move this note to trash?",
    trashedNote: "Moved to trash",
    untitled: "Untitled",
    uploadAttachmentFailed: "Attachment upload failed",
    unorderedList: "Bulleted list",
    userManagement: "Users",
    userManagementComing: "User management will be connected in Settings",
    updateStarFailed: "Failed to update star",
    warnings: "Warnings",
    wideImage: "Long image",
    wordUnit: "chars",
    yesterday: "Yesterday",
  },
} satisfies Record<Language, Record<string, string>>;

export type MessageKey = keyof typeof messages.zh;

function storedLanguage(): Language {
  if (typeof localStorage !== "undefined") {
    const saved = localStorage.getItem("language");
    if (saved === "zh" || saved === "en") return saved;
  }
  if (typeof navigator !== "undefined" && navigator.language.startsWith("zh")) {
    return "zh";
  }
  return "en";
}

function storedTheme(): Theme {
  if (typeof localStorage !== "undefined") {
    const saved = localStorage.getItem("theme");
    if (saved === "light" || saved === "dark") return saved;
  }
  if (
    typeof window !== "undefined" &&
    window.matchMedia("(prefers-color-scheme: dark)").matches
  ) {
    return "dark";
  }
  return "light";
}

function applyPreferences(state: PreferencesState): void {
  if (typeof document === "undefined") return;
  document.documentElement.lang = state.language === "zh" ? "zh-CN" : "en";
  document.documentElement.dataset.theme = state.theme;
}

function createPreferencesStore() {
  const initial = {
    language: storedLanguage(),
    theme: storedTheme(),
  };
  const { subscribe, set, update } = writable<PreferencesState>(initial);
  applyPreferences(initial);

  function commit(next: PreferencesState): PreferencesState {
    if (typeof localStorage !== "undefined") {
      localStorage.setItem("language", next.language);
      localStorage.setItem("theme", next.theme);
    }
    applyPreferences(next);
    return next;
  }

  return {
    subscribe,
    hydrate() {
      set(commit({ language: storedLanguage(), theme: storedTheme() }));
    },
    setLanguage(language: Language) {
      update((state) => commit({ ...state, language }));
    },
    toggleLanguage() {
      update((state) =>
        commit({ ...state, language: state.language === "zh" ? "en" : "zh" }),
      );
    },
    setTheme(theme: Theme) {
      update((state) => commit({ ...state, theme }));
    },
    toggleTheme() {
      update((state) =>
        commit({ ...state, theme: state.theme === "light" ? "dark" : "light" }),
      );
    },
  };
}

export const preferencesStore = createPreferencesStore();

export function t(
  key: MessageKey,
  language: Language = get(preferencesStore).language,
): string {
  return messages[language][key] ?? messages.zh[key] ?? key;
}
