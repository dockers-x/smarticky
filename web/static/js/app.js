const API_BASE = "/api";

// Configure marked.js to support GitHub Flavored Markdown (task lists, tables, etc.)
if (typeof marked !== "undefined") {
  marked.setOptions({
    gfm: true,
    breaks: true,
    pedantic: false,
    smartLists: true,
    smartypants: false,
  });
}

// Auth state
let currentUser = null;
let jwtToken = null;

let state = {
  notes: [],
  currentFilter: "all",
  currentNote: null,
  searchQuery: "",
  sortBy: "updated_desc",
  lang: "zh",
  i18n: {},
  backupConfig: null,
  unlockedNotes: new Set(), // Track unlocked notes in this session
  sidebarExpanded: true, // Track if sidebar is expanded
  attachmentsExpanded: false, // Track if attachments section is expanded (default collapsed)
};

// Auth functions
function getAuthToken() {
  return localStorage.getItem("jwt_token");
}

function getCurrentUser() {
  const userJson = localStorage.getItem("user");
  return userJson ? JSON.parse(userJson) : null;
}

function isAuthenticated() {
  return !!getAuthToken();
}

function logout() {
  localStorage.removeItem("jwt_token");
  localStorage.removeItem("user");
  window.location.href = "/login";
}

// Check authentication and setup status
async function checkAuth() {
  // Check if setup is needed
  try {
    const res = await fetch(`${API_BASE}/setup/check`);
    const data = await res.json();

    if (data.setup_needed) {
      window.location.href = "/setup";
      return false;
    }
  } catch (e) {
    console.error("Failed to check setup status", e);
  }

  // Check if authenticated
  if (!isAuthenticated()) {
    window.location.href = "/login";
    return false;
  }

  // Load current user
  currentUser = getCurrentUser();
  jwtToken = getAuthToken();

  return true;
}

// Fetch with auth
async function fetchWithAuth(url, options = {}) {
  const token = getAuthToken();
  if (!token) {
    logout();
    return null;
  }

  const headers = {
    ...options.headers,
    Authorization: `Bearer ${token}`,
  };

  const response = await fetch(url, { ...options, headers });

  // If unauthorized, logout
  if (response.status === 401) {
    logout();
    return null;
  }

  return response;
}

// Session management for unlocked notes (memory only - cleared on refresh)
function isNoteUnlocked(noteId) {
  // Only check in-memory state - no persistence
  return state.unlockedNotes.has(noteId);
}

function markNoteUnlocked(noteId) {
  // Only store in memory - will be cleared on page refresh
  state.unlockedNotes.add(noteId);
}

function clearNoteUnlock(noteId) {
  state.unlockedNotes.delete(noteId);
}

// Manually lock a note (clear unlock status and show lock screen)
function lockNote(noteId) {
  clearNoteUnlock(noteId);
  renderEditor(); // Refresh to show lock screen
  showNotification(t("note_locked") || "Note locked", "info");
}

// Load i18n
async function loadI18n() {
  try {
    const res = await fetch("/static/js/i18n.json");
    const data = await res.json();
    state.i18n = data;
  } catch (e) {
    console.error("Failed to load i18n", e);
  }
}

// Get translation
function t(key) {
  return state.i18n[state.lang]?.[key] || key;
}

// Update UI text with translations
function updateUIText() {
  // Generic handler for all elements with data-i18n attribute
  document.querySelectorAll("[data-i18n]").forEach((el) => {
    const key = el.getAttribute("data-i18n");
    if (key) {
      el.textContent = t(key);
    }
  });

  // Sidebar menu
  const menuTexts = {
    "menu-all": "all_notes",
    "menu-starred": "starred",
    "menu-trash": "trash",
    "menu-users": "user_management",
    "menu-profile": "profile_settings",
  };

  for (const [id, key] of Object.entries(menuTexts)) {
    const el = document.getElementById(id);
    if (el) {
      const textEl = el.querySelector(".menu-text");
      if (textEl) {
        textEl.textContent = t(key);
      }
    }
  }

  // Search placeholder
  const searchInput = document.getElementById("search-input");
  if (searchInput) {
    searchInput.placeholder = t("search_placeholder");
  }

  // Button titles
  const titleMappings = [
    { selector: ".btn-new-note", key: "new_note" },
    { selector: 'button[onclick="toggleLanguage()"]', key: "language" },
    { selector: 'button[onclick="toggleTheme()"]', key: "toggle_theme" },
    {
      selector: 'button[onclick="showBackupConfig()"]',
      key: "backup_settings",
    },
    { selector: 'button[onclick="showSortMenu()"]', key: "sort" },
    { selector: ".panel-expander", key: "toggle_panel" },
  ];

  titleMappings.forEach(({ selector, key }) => {
    const el = document.querySelector(selector);
    if (el) {
      el.title = t(key);
    }
  });
}

// Init
document.addEventListener("DOMContentLoaded", async () => {
  // Check authentication first
  const authenticated = await checkAuth();
  if (!authenticated) {
    return; // Will redirect to login or setup
  }

  // Detect language - check localStorage first, then browser language
  const savedLang = localStorage.getItem("language");
  if (savedLang) {
    state.lang = savedLang;
  } else {
    const browserLang = navigator.language || navigator.userLanguage;
    state.lang = browserLang.startsWith("zh") ? "zh" : "en";
  }

  await loadI18n();
  updateUIText(); // Update UI text with translations

  // Load and display user info
  if (currentUser) {
    // Display nickname if available, otherwise username
    document.getElementById("user-name").textContent =
      currentUser.nickname || currentUser.username;
    document.getElementById("user-role").textContent = currentUser.role;
    document.getElementById("user-avatar-img").src =
      currentUser.avatar || "/static/img/default-avatar.svg";

    // Show admin menu if admin
    if (currentUser.role === "admin") {
      document.getElementById("menu-users").style.display = "flex";
    }
  }

  await fetchNotes();
  await loadBackupConfig();

  // Load all tags for autocomplete functionality
  loadAllTags();

  // Theme check
  const isDark = localStorage.getItem("theme") === "dark";
  if (isDark) {
    document.body.classList.add("dark-mode");
  }

  // Update theme icon based on current theme
  const themeIcon = document.getElementById("theme-icon");
  if (themeIcon) {
    themeIcon.setAttribute("data-feather", isDark ? "sun" : "moon");
    feather.replace();
  }

  // Sort preference
  const savedSort = localStorage.getItem("sortBy");
  if (savedSort) {
    state.sortBy = savedSort;
  }

  // Keyboard shortcuts
  initKeyboardShortcuts();

  // Close modals on click outside
  document.querySelectorAll(".modal").forEach((modal) => {
    modal.addEventListener("click", (e) => {
      if (e.target === modal) {
        closeModal(modal.id);
      }
    });
  });
});

// Keyboard Shortcuts
function initKeyboardShortcuts() {
  document.addEventListener("keydown", (e) => {
    // Cmd/Ctrl + N: New Note
    if ((e.metaKey || e.ctrlKey) && e.key === "n") {
      e.preventDefault();
      createNote();
    }
    // Cmd/Ctrl + F: Focus Search
    if ((e.metaKey || e.ctrlKey) && e.key === "f") {
      e.preventDefault();
      document.getElementById("search-input").focus();
    }
    // Cmd/Ctrl + S: Save (show feedback)
    if ((e.metaKey || e.ctrlKey) && e.key === "s") {
      e.preventDefault();
      if (state.currentNote) {
        showSaveNotification();
      }
    }
    // Cmd/Ctrl + D: Toggle Dark Mode
    if ((e.metaKey || e.ctrlKey) && e.key === "d") {
      e.preventDefault();
      toggleTheme();
    }
    // Escape: Close modals
    if (e.key === "Escape") {
      document.querySelectorAll(".modal.show").forEach((modal) => {
        closeModal(modal.id);
      });
    }
  });
}

// Fetch Notes
async function fetchNotes() {
  let url = `${API_BASE}/notes?`;
  if (state.currentFilter === "starred") url += "starred=true&";
  if (state.currentFilter === "trash") url += "trash=true&";
  if (state.searchQuery) url += `q=${encodeURIComponent(state.searchQuery)}&`;

  try {
    const res = await fetchWithAuth(url);
    if (!res) return;

    const notes = await res.json();
    state.notes = notes || [];
    applySorting();
    renderList();
  } catch (e) {
    console.error("Failed to fetch notes", e);
  }
}

// Apply Sorting
function applySorting() {
  switch (state.sortBy) {
    case "updated_desc":
      state.notes.sort(
        (a, b) => new Date(b.updated_at) - new Date(a.updated_at),
      );
      break;
    case "updated_asc":
      state.notes.sort(
        (a, b) => new Date(a.updated_at) - new Date(b.updated_at),
      );
      break;
    case "title_asc":
      state.notes.sort((a, b) => (a.title || "").localeCompare(b.title || ""));
      break;
    case "title_desc":
      state.notes.sort((a, b) => (b.title || "").localeCompare(a.title || ""));
      break;
    case "color":
      state.notes.sort((a, b) => (a.color || "").localeCompare(b.color || ""));
      break;
  }
}

// Render List
function renderList() {
  const listEl = document.getElementById("note-list");
  listEl.innerHTML = "";

  if (state.notes.length === 0) {
    listEl.innerHTML = `<div style="padding: 40px 20px; text-align: center; color: #999;">
            ${t("no_notes") || "No notes yet"}
        </div>`;
    return;
  }

  state.notes.forEach((note) => {
    const el = document.createElement("div");
    el.className = `note-item ${state.currentNote && state.currentNote.id === note.id ? "active" : ""}`;
    el.onclick = () => selectNote(note);
    if (note.color) {
      el.setAttribute("data-color", note.color);
    }

    const date = new Date(note.updated_at).toLocaleString();
    const starIcon = note.is_starred
      ? '<i data-feather="star" class="star-filled"></i>'
      : "";

    // Check if note is locked and not unlocked in this session
    const isLocked = note.is_locked && !isNoteUnlocked(note.id);
    const lockIcon = note.is_locked
      ? '<i data-feather="lock" style="width: 14px; height: 14px; margin-left: 6px;"></i>'
      : "";

    // Hide content preview for locked notes
    const contentPreview = isLocked
      ? '<span style="color: #999; font-style: italic;">' + (t("note_locked") || "Note Locked") + '</span>'
      : escapeHtml(note.content || t("no_content")).substring(0, 100);

    el.innerHTML = `
            <div class="note-title">${escapeHtml(note.title || t("untitled"))}${lockIcon}</div>
            <div class="note-preview">${contentPreview}</div>
            <div class="note-meta">
                <span>${date}</span>
                ${starIcon}
            </div>
        `;
    listEl.appendChild(el);
  });

  // Refresh Feather Icons
  feather.replace();

  // Update Sidebar Active State
  document
    .querySelectorAll(".menu-item")
    .forEach((el) => el.classList.remove("active"));
  const activeMenu = document.getElementById(`menu-${state.currentFilter}`);
  if (activeMenu) {
    activeMenu.classList.add("active");
  }
}

// Select Note
function selectNote(note) {
  // Auto-lock previous note when switching to a different note
  if (state.currentNote && state.currentNote.id !== note.id) {
    if (state.currentNote.is_locked) {
      clearNoteUnlock(state.currentNote.id);
    }
  }

  // Check if note is locked and not unlocked in this session
  if (note.is_locked && !isNoteUnlocked(note.id)) {
    state.currentNote = note;
    renderList(); // to update active class
    renderEditor(); // Will show lock screen with unlock button

    // On mobile, open the editor panel
    if (window.innerWidth <= 768) {
      document.body.classList.add('mobile-editor-open');
    }
    return;
  }

  state.currentNote = note;
  renderList(); // to update active class

  // Load all tags for autocomplete functionality
  loadAllTags();

  renderEditor();

  // On mobile, open the editor panel when a note is selected
  if (window.innerWidth <= 768) {
    document.body.classList.add('mobile-editor-open');
  }
}

// Render Editor
function renderEditor() {
  const panel = document.getElementById("editor-panel");
  if (!state.currentNote) {
    panel.innerHTML = `<div class="empty-state">${t("select_note")}</div>`;
    panel.removeAttribute("data-color");
    return;
  }

  const note = state.currentNote;
  const isTrash = state.currentFilter === "trash";
  const isLocked = note.is_locked && !isNoteUnlocked(note.id);

  // Set editor background color
  if (note.color) {
    panel.setAttribute("data-color", note.color);
  } else {
    panel.removeAttribute("data-color");
  }

  const starIcon = note.is_starred ? "star" : "star";
  const starClass = note.is_starred ? "star-filled" : "";
  const lockIcon = note.is_locked ? "lock" : "unlock";
  const isNoteCurrentlyUnlocked = note.is_locked && isNoteUnlocked(note.id);

  // If locked and not unlocked, show lock screen
  if (isLocked) {
    panel.innerHTML = `
            <div class="editor-header">
                <button class="mobile-back-btn" onclick="closeMobileEditor()" title="Back">
                    <i data-feather="arrow-left"></i>
                </button>
                <span>${t("last_updated")}: ${new Date(note.updated_at).toLocaleString()}</span>
                <div class="toolbar">
                    <button class="btn" onclick="showPasswordModal()" title="${t("unlock_note")}">
                        <i data-feather="lock"></i>
                    </button>
                </div>
            </div>
            <div class="locked-note">
                <i data-feather="lock" style="width: 64px; height: 64px; margin-bottom: 20px;"></i>
                <h2>${t("note_locked")}</h2>
                <p>${t("note_locked_desc")}</p>
                <button class="btn-primary" onclick="showPasswordModal()">
                    <i data-feather="unlock"></i> ${t("unlock_note")}
                </button>
            </div>
        `;
    feather.replace();
    return;
  }

  panel.innerHTML = `
        <div class="editor-header">
            <button class="mobile-back-btn" onclick="closeMobileEditor()" title="Back">
                <i data-feather="arrow-left"></i>
            </button>
            <div class="save-status" id="save-status"></div>
            <span>${t("last_updated")}: ${new Date(note.updated_at).toLocaleString()}</span>
            <div class="toolbar">
                ${!isTrash
      ? `
                <button class="btn ${note.is_starred ? "active" : ""}" onclick="toggleStar('${note.id}', ${!note.is_starred})" title="${t("toggle_star") || "Toggle Star"}">
                    <i data-feather="${starIcon}" class="${starClass}"></i>
                </button>
                <button class="btn ${note.is_locked ? "active" : ""}" onclick="${isNoteCurrentlyUnlocked ? `lockNote('${note.id}')` : `showPasswordModal()`}" title="${isNoteCurrentlyUnlocked ? (t("note_locked") || "Lock Note") : t("password_protect")}">
                    <i data-feather="${lockIcon}"></i>
                </button>
                <button class="btn" onclick="showColorPicker()" title="${t("change_color") || "Change Color"}">
                    <i data-feather="droplet"></i>
                </button>
                <button class="btn" onclick="exportNote()" title="${t("export") || "Export"}">
                    <i data-feather="download"></i>
                </button>
                <button class="btn" onclick="shareAsImage()" title="${t("share_as_image") || "Share as Image"}">
                    <i data-feather="image"></i>
                </button>
                <button class="btn" onclick="uploadAttachment('${note.id}')" title="${t("attachments") || "Attachments"}">
                    <i data-feather="paperclip"></i>
                </button>
                <button class="btn" onclick="deleteNote('${note.id}')" title="${t("move_to_trash")}">
                    <i data-feather="trash-2"></i>
                </button>
                `
      : `
                <button class="btn" onclick="restoreNote('${note.id}')">
                    <i data-feather="rotate-ccw"></i> ${t("restore")}
                </button>
                <button class="btn" onclick="deleteNotePermanent('${note.id}')">
                    <i data-feather="x-circle"></i> ${t("delete_forever")}
                </button>
                `
    }
            </div>
        </div>
        <div class="editor-title">
            <input type="text" value="${escapeHtml(note.title)}" oninput="updateNoteDebounced('${note.id}', 'title', this.value)" ${isTrash ? "disabled" : ""} placeholder="${t("untitled")}">
        </div>
        ${renderMarkdownEditor(note, isTrash)}
        ${!isTrash
      ? `
        <div class="attachments-section ${state.attachmentsExpanded ? "" : "collapsed"}">
            <div class="attachments-header" onclick="toggleAttachments()">
                <div style="display: flex; justify-content: space-between; align-items: center;">
                    <h3 style="margin: 0; font-size: 14px; color: #666; display: flex; align-items: center; gap: 8px; cursor: pointer;">
                        <i data-feather="${state.attachmentsExpanded ? "chevron-down" : "chevron-right"}" class="attachments-toggle-icon" style="width: 16px; height: 16px;"></i>
                        ${t("attachments") || "Attachments"}
                    </h3>
                    ${state.attachmentsExpanded
        ? `<button class="btn-secondary" onclick="event.stopPropagation(); uploadAttachment('${note.id}')" style="padding: 6px 12px; font-size: 13px;">
                        <i data-feather="upload" style="width: 14px; height: 14px;"></i> ${t("upload_file") || "Upload"}
                    </button>`
        : ""
      }
                </div>
            </div>
            ${state.attachmentsExpanded
        ? `<div class="attachment-list" id="attachment-list" style="display: flex; flex-direction: column; gap: 10px; margin-top: 10px;">
                <!-- Attachments will be loaded here -->
            </div>`
        : ""
      }
        </div>
        `
      : ""
    }
    `;

  // Refresh Feather Icons
  feather.replace();

  // Milkdown initialization removed for Typora-like experience

  // Load attachments if not in trash
  if (!isTrash && note.id) {
    renderAttachments(note.id);
  }
}

function renderMarkdownEditor(note, isTrash) {
  // For Typora-like experience, only use traditional editor with source/preview modes
  return `
        <div class="editor-content markdown-editor-wrapper">
            <!-- 标签管理区域 -->
            <div class="tag-management" style="
                margin-bottom: 10px;
                padding: 10px;
                background: var(--bg-secondary);
                border-radius: 6px;
                border: 1px solid var(--border-light);
            ">
                <div style="display: flex; align-items: center; gap: 10px; margin-bottom: 8px;">
                    <i data-feather="tag" style="width: 16px; height: 16px; color: var(--text-secondary);"></i>
                    <span style="font-size: 14px; color: var(--text-secondary);">${t("tags") || "Tags"}</span>
                </div>
                <div id="current-tags" class="current-tags" style="display: flex; flex-wrap: wrap; gap: 5px; margin-bottom: 8px;">
                    ${renderCurrentTags(note)}
                </div>
                <div style="display: flex; gap: 5px;">
                    <input type="text" id="tag-input" placeholder="${t("add_tag") || "Add tag..."}"
                           style="flex: 1; padding: 6px 10px; border: 1px solid var(--border-light); border-radius: 4px; font-size: 13px;"
                           onkeypress="handleTagInput(event, '${note.id}')"
                           oninput="handleTagAutocomplete(this.value)"
                           onblur="setTimeout(() => hideTagAutocomplete(), 200)">
                </div>
            </div>

            <!-- 源码/预览模式切换按钮 -->
            <div class="editor-mode-selector" style="
                position: absolute;
                top: 10px;
                right: 10px;
                z-index: 100;
                display: flex;
                gap: 5px;
                background: var(--bg-secondary);
                padding: 4px;
                border-radius: 6px;
                border: 1px solid var(--border-light);
                box-shadow: 0 2px 8px var(--shadow-medium);
            ">
                <button class="editor-mode-btn ${state.markdownViewMode === 'source' ? 'active' : ''}"
                        onclick="switchToSourceMode()"
                        title="${t("source_code") || "Source Code"}"
                        style="${state.markdownViewMode === 'source' ? 'background: var(--primary-color); color: white;' : ''}">
                    <i data-feather="code" style="width: 14px; height: 14px;"></i>
                </button>
                <button class="editor-mode-btn ${state.markdownViewMode === 'preview' ? 'active' : ''}"
                        onclick="switchToPreviewMode()"
                        title="${t("preview") || "Preview"}"
                        style="${state.markdownViewMode === 'preview' ? 'background: var(--primary-color); color: white;' : ''}">
                    <i data-feather="eye" style="width: 14px; height: 14px;"></i>
                </button>
            </div>

            <!-- 源码编辑器 -->
            <textarea id="markdown-editor"
                      style="display: ${state.markdownViewMode === 'source' ? 'block' : 'none'};"
                      oninput="handleMarkdownInput('${note.id}', this)"
                      onkeydown="handleMarkdownKeydown(event)"
                      ${isTrash ? "disabled" : ""}
                      placeholder="${t("start_writing") || "Start writing..."}">${escapeHtml(note.content)}</textarea>
            <div id="markdown-autocomplete" class="markdown-autocomplete" style="display: none;"></div>

            <!-- 预览模式内容 -->
            <div id="markdown-preview-content"
                 class="preview-content"
                 style="display: ${state.markdownViewMode === 'preview' ? 'block' : 'none'};"
                 onclick="handlePreviewClick(event)"
                 title="${t("click_to_edit") || "Click to edit"}">
            </div>
        </div>
    `;
}

// 编辑器切换功能 - Removed Milkdown for Typora-like experience
// Only traditional editor with source/preview modes is available

// 同步编辑器数据 - Simplified for traditional editor only
function syncEditorData() {
  if (!state.currentNote) return;

  // Only handle traditional editor (textarea)
  const textarea = document.getElementById('markdown-editor');
  if (textarea && textarea.value !== state.currentNote.content) {
    state.currentNote.content = textarea.value;
    // 立即保存到服务器
    updateNoteDebounced(state.currentNote.id, 'content', textarea.value);
  }
}

// 更新编辑器类型设置 - Removed as we only have traditional editor now
function updateEditorType() {
  // Function no longer needed, kept for compatibility
  showNotification('Editor preference saved', 'success');
}

// Tag管理函数
function renderCurrentTags(note) {
  if (!note.tags || note.tags.length === 0) {
    return `<span style="color: var(--text-tertiary); font-size: 13px;">${t("no_tags") || "No tags"}</span>`;
  }

  return note.tags.map(tag => `
    <span class="tag-item" style="
      display: inline-flex;
      align-items: center;
      gap: 4px;
      padding: 4px 8px;
      background: ${tag.color || 'var(--primary-color)'};
      color: white;
      border-radius: 12px;
      font-size: 12px;
      font-weight: 500;
    ">
      ${tag.name}
      <button onclick="removeTag('${note.id}', '${tag.id}')"
              style="background: none; border: none; color: white; cursor: pointer; padding: 0; margin: 0;"
              title="${t("remove_tag") || "Remove tag"}">
        <i data-feather="x" style="width: 12px; height: 12px;"></i>
      </button>
    </span>
  `).join('');
}

async function addTag(noteId) {
  const input = document.getElementById('tag-input');
  const tagName = input.value.trim();

  if (!tagName) return;

  // 检查是否已经存在相同的tag
  if (state.currentNote.tags && state.currentNote.tags.some(tag => tag.name.toLowerCase() === tagName.toLowerCase())) {
    showNotification(t("tag_exists") || "Tag already exists", "warning");
    return;
  }

  try {
    // 首先尝试获取现有的tags
    const getTagsRes = await fetchWithAuth(`${API_BASE}/tags`);
    let existingTag = null;

    if (getTagsRes && getTagsRes.ok) {
      const allTags = await getTagsRes.json();
      existingTag = allTags.find(tag => tag.name.toLowerCase() === tagName.toLowerCase());
    }

    let tagToUse;

    if (existingTag) {
      // 使用现有的tag
      tagToUse = existingTag;
    } else {
      // 创建新tag
      const createRes = await fetchWithAuth(`${API_BASE}/tags`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name: tagName, color: '' })
      });

      if (!createRes || !createRes.ok) {
        const error = createRes ? await createRes.json() : { error: "Network error" };
        if (createRes.status === 409) {
          // Tag已存在，获取现有tag
          const getExistingTagRes = await fetchWithAuth(`${API_BASE}/tags`);
          if (getExistingTagRes && getExistingTagRes.ok) {
            const allTags = await getExistingTagRes.json();
            tagToUse = allTags.find(tag => tag.name.toLowerCase() === tagName.toLowerCase());
          }
        } else {
          showNotification(t("create_tag_failed") || "Failed to create tag", "error");
          return;
        }
      } else {
        tagToUse = await createRes.json();
      }
    }

    if (!tagToUse) {
      showNotification(t("tag_not_found") || "Tag not found", "error");
      return;
    }

    // 检查是否已经添加了这个tag到note
    const currentTagIds = state.currentNote.tags ? state.currentNote.tags.map(t => t.id) : [];
    if (currentTagIds.includes(tagToUse.id)) {
      showNotification(t("tag_already_added") || "Tag already added to note", "warning");
      return;
    }

    // 添加tag到note
    const addRes = await fetchWithAuth(`${API_BASE}/notes/${noteId}/tags/${tagToUse.id}`, {
      method: 'POST'
    });

    if (!addRes || !addRes.ok) {
      const error = addRes ? await addRes.json() : { error: "Network error" };
      showNotification(t("add_tag_failed") || "Failed to add tag to note", "error");
      return;
    }

    // 更新当前note的tags
    if (!state.currentNote.tags) state.currentNote.tags = [];
    state.currentNote.tags.push(tagToUse);

    // 清空输入框
    input.value = '';

    // 重新渲染编辑器
    renderEditor();

    showNotification(t("tag_added") || "Tag added successfully", "success");
  } catch (e) {
    console.error("Add tag error:", e);
    showNotification(t("tag_add_error") || "Error adding tag", "error");
  }
}

async function removeTag(noteId, tagId) {
  try {
    const res = await fetchWithAuth(`${API_BASE}/notes/${noteId}/tags/${tagId}`, {
      method: 'DELETE'
    });

    if (!res || !res.ok) {
      const error = res ? await res.json() : { error: "Network error" };
      showNotification(t("remove_tag_failed") || "Failed to remove tag", "error");
      return;
    }

    // 从当前note的tags中移除
    if (state.currentNote.tags) {
      state.currentNote.tags = state.currentNote.tags.filter(tag => tag.id !== tagId);
    }

    // 重新渲染编辑器
    renderEditor();

    showNotification(t("tag_removed") || "Tag removed successfully", "success");
  } catch (e) {
    console.error("Remove tag error:", e);
    showNotification(t("tag_remove_error") || "Error removing tag", "error");
  }
}

function handleTagInput(event, noteId) {
  if (event.key === 'Enter') {
    event.preventDefault();
    addTag(noteId);
  } else if (event.key === ' ' || event.key === 'Spacebar') {
    // 按空格键添加tag
    event.preventDefault();
    const input = document.getElementById('tag-input');
    if (input.value.trim()) {
      addTag(noteId);
    }
  }
}

// Tag自动完成功能
let allTags = [];
let tagAutocompleteVisible = false;

async function loadAllTags() {
  try {
    const res = await fetchWithAuth(`${API_BASE}/tags`);
    if (res && res.ok) {
      allTags = await res.json();
    }
  } catch (e) {
    console.error("Failed to load tags", e);
  }
}

function handleTagAutocomplete(value) {
  if (!value.trim()) {
    hideTagAutocomplete();
    return;
  }

  const filteredTags = allTags.filter(tag =>
    tag.name.toLowerCase().includes(value.toLowerCase()) &&
    (!state.currentNote.tags || !state.currentNote.tags.some(usedTag => usedTag.id === tag.id))
  );

  if (filteredTags.length === 0) {
    hideTagAutocomplete();
    return;
  }

  showTagAutocomplete(filteredTags, value);
}

function showTagAutocomplete(tags, currentValue) {
  hideTagAutocomplete();

  const input = document.getElementById('tag-input');
  const rect = input.getBoundingClientRect();

  const autocompleteHTML = `
    <div id="tag-autocomplete" class="tag-autocomplete" style="
      position: absolute;
      top: ${rect.bottom + window.scrollY}px;
      left: ${rect.left + window.scrollX}px;
      width: ${rect.width}px;
      background: var(--bg-primary);
      border: 1px solid var(--border-light);
      border-radius: 4px;
      box-shadow: 0 2px 8px var(--shadow-medium);
      z-index: 1000;
      max-height: 200px;
      overflow-y: auto;
    ">
      ${tags.map((tag, index) => `
        <div class="tag-autocomplete-item"
             style="
               padding: 8px 12px;
               cursor: pointer;
               font-size: 13px;
               ${index === 0 ? 'background: var(--bg-secondary);' : ''}
             "
             onmouseover="this.style.background='var(--bg-secondary)'"
             onmouseout="this.style.background='transparent'"
             onclick="selectAutocompleteTag('${tag.id}', '${tag.name}')"
        >
          ${tag.name}
        </div>
      `).join('')}
    </div>
  `;

  document.body.insertAdjacentHTML('beforeend', autocompleteHTML);
  tagAutocompleteVisible = true;
}

function hideTagAutocomplete() {
  const autocomplete = document.getElementById('tag-autocomplete');
  if (autocomplete) {
    autocomplete.remove();
  }
  tagAutocompleteVisible = false;
}

function selectAutocompleteTag(tagId, tagName) {
  const input = document.getElementById('tag-input');
  input.value = tagName;
  hideTagAutocomplete();

  // 延迟一点执行添加，确保输入框已更新
  setTimeout(() => {
    if (state.currentNote) {
      addTag(state.currentNote.id);
    }
  }, 100);
}

// Switch to source mode (within traditional editor)
function switchToSourceMode() {
  state.markdownViewMode = 'source';

  // Ensure content is synchronized before switching
  syncPreviewWithSource();

  renderEditor();
}

// Switch to preview mode (within traditional editor)
function switchToPreviewMode() {
  state.markdownViewMode = 'preview';

  // Ensure content is synchronized before switching
  syncPreviewWithSource();

  renderEditor();
}

// Milkdown editor functions removed for Typora-like experience

// Create Note
async function createNote() {
  try {
    const res = await fetchWithAuth(`${API_BASE}/notes`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        title: state.i18n[state.lang]?.untitled || "Untitled",
        content: "",
        color: "",
      }),
    });

    if (!res || !res.ok) {
      const error = res ? await res.json() : { error: "Network error" };
      console.error("Create note failed:", error);
      alert(t("create_note_failed") + ": " + (error.error || "Unknown error"));
      return;
    }

    const note = await res.json();
    state.currentFilter = "all"; // Switch to all to see new note
    state.searchQuery = "";
    await fetchNotes();
    selectNote(note);
  } catch (e) {
    console.error("Create note error:", e);
    alert(t("create_note_failed") + ": " + e.message);
  }
}

// 更新字数统计函数
function updateWordCount() {
  if (!state.currentNote) return;

  const content = state.currentNote.content || '';
  const wordCount = content.trim().length > 0 ? content.trim().split(/\s+/).length : 0;
  const charCount = content.length;

  // 可以在这里添加字数统计显示逻辑
  // 例如更新状态栏或某个UI元素
  console.log(`Words: ${wordCount}, Characters: ${charCount}`);
}

// 更新Markdown预览函数
function updateMarkdownPreview() {
  const previewContent = document.getElementById('markdown-preview-content');
  if (!previewContent || !state.currentNote) return;

  const content = state.currentNote.content || '';
  const rendered = marked.parse(content);
  previewContent.innerHTML = rendered;
}

function updateNoteDebounced(id, field, value) {
  // Update local state immediately for UI responsiveness
  if (state.currentNote && state.currentNote.id === id) {
    state.currentNote[field] = value;
  }

  // Show saving status
  showSaveStatus("saving");

  clearTimeout(debounceTimer);
  debounceTimer = setTimeout(() => {
    updateNote(id, { [field]: value });
  }, 500);
}

function showSaveStatus(status) {
  const statusEl = document.getElementById("save-status");
  if (!statusEl) return;

  clearTimeout(saveStatusTimer);

  if (status === "saving") {
    statusEl.innerHTML =
      '<i data-feather="loader" class="rotating"></i> ' +
      (t("saving") || "Saving...");
    statusEl.className = "save-status saving";
    feather.replace();
  } else if (status === "saved") {
    statusEl.innerHTML =
      '<i data-feather="check"></i> ' + (t("saved") || "Saved");
    statusEl.className = "save-status saved";
    feather.replace();

    // Hide after 2 seconds
    saveStatusTimer = setTimeout(() => {
      statusEl.innerHTML = "";
      statusEl.className = "save-status";
    }, 2000);
  } else if (status === "error") {
    statusEl.innerHTML =
      '<i data-feather="alert-circle"></i> ' +
      (t("save_error") || "Save failed");
    statusEl.className = "save-status error";
    feather.replace();
  }
}

function showSaveNotification() {
  // Manual save notification (Ctrl+S)
  showSaveStatus("saved");
}

async function updateNote(id, data) {
  try {
    const res = await fetchWithAuth(`${API_BASE}/notes/${id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(data),
    });

    if (!res) return; // Auth failed

    showSaveStatus("saved");

    // Refresh note in list
    if (data.color !== undefined) {
      await fetchNotes();
      if (state.currentNote && state.currentNote.id === id) {
        selectNote(state.currentNote);
      }
    }
  } catch (e) {
    console.error("Failed to update note", e);
    showSaveStatus("error");
  }
}

// Toggle Star
async function toggleStar(id, isStarred) {
  await updateNote(id, { is_starred: isStarred });
  if (state.currentNote && state.currentNote.id === id) {
    state.currentNote.is_starred = isStarred;
  }
  renderEditor(); // Update star icon
  fetchNotes(); // Update list
}

// Delete (Move to Trash)
async function deleteNote(id) {
  if (!confirm(t("move_to_trash"))) return;
  await updateNote(id, { is_deleted: true });
  state.currentNote = null;
  fetchNotes();
  renderEditor();
}

// Restore
async function restoreNote(id) {
  await updateNote(id, { is_deleted: false });
  state.currentNote = null;
  fetchNotes();
  renderEditor();
}

// Permanent Delete
async function deleteNotePermanent(id) {
  if (!confirm(t("delete_confirm"))) return;
  try {
    const res = await fetchWithAuth(`${API_BASE}/notes/${id}`, {
      method: "DELETE",
    });
    if (!res) return;

    state.currentNote = null;
    fetchNotes();
    renderEditor();
  } catch (e) {
    alert(t("delete_failed"));
  }
}

// Filter
function filterNotes(filter) {
  // Auto-lock current note when changing filter
  if (state.currentNote && state.currentNote.is_locked) {
    clearNoteUnlock(state.currentNote.id);
  }

  state.currentFilter = filter;
  state.currentNote = null;
  fetchNotes();
  renderEditor();
}

// Search
function searchNotes(query) {
  state.searchQuery = query;
  fetchNotes();
}

// Sort
function sortNotes(sortBy) {
  state.sortBy = sortBy;
  localStorage.setItem("sortBy", sortBy);
  applySorting();
  renderList();
  closeModal("sort-modal");
}

// Theme
function toggleTheme() {
  document.body.classList.toggle("dark-mode");
  const isDark = document.body.classList.contains("dark-mode");

  localStorage.setItem("theme", isDark ? "dark" : "light");

  // Update theme icon
  const themeIcon = document.getElementById("theme-icon");
  if (themeIcon) {
    themeIcon.setAttribute("data-feather", isDark ? "sun" : "moon");
    feather.replace();
  }
}

// Language Toggle
function toggleLanguage() {
  state.lang = state.lang === "zh" ? "en" : "zh";
  localStorage.setItem("language", state.lang);

  // Update all UI text
  updateUIText();

  // Re-render list and editor to update dynamic content
  renderList();
  renderEditor();

  // Refresh Feather icons
  feather.replace();
}

// Modal Controls
function showModal(modalId) {
  const modal = document.getElementById(modalId);
  if (modal) {
    modal.classList.add("show");
  }
}

function closeModal(modalId) {
  const modal = document.getElementById(modalId);
  if (modal) {
    modal.classList.remove("show");
  }
}

function showSortMenu() {
  showModal("sort-modal");
  // Refresh icons in modal
  setTimeout(() => feather.replace(), 10);
}

function showColorPicker() {
  if (!state.currentNote) return;

  // Update selected color
  setTimeout(() => {
    document.querySelectorAll(".color-option").forEach((el) => {
      el.classList.remove("selected");
      if (el.dataset.color === (state.currentNote.color || "")) {
        el.classList.add("selected");
      }
    });
    // Refresh icons in modal
    feather.replace();
  }, 10);

  showModal("color-modal");
}

function setNoteColor(color) {
  if (!state.currentNote) return;
  updateNote(state.currentNote.id, { color: color });
  state.currentNote.color = color;
  renderEditor();
  closeModal("color-modal");
}

// Export Note
function exportNote() {
  if (!state.currentNote) return;

  const note = state.currentNote;
  const content = `# ${note.title || "Untitled"}\n\n${note.content || ""}`;
  const blob = new Blob([content], { type: "text/markdown" });
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = `${note.title || "Untitled"}.md`;
  a.click();
  URL.revokeObjectURL(url);
}

// Backup Configuration
async function showBackupConfig() {
  await loadBackupConfig();

  // Update all translations in the modal
  updateUIText();

  showModal("backup-modal");
  // Refresh icons in modal
  setTimeout(() => feather.replace(), 10);
}

async function loadBackupConfig() {
  try {
    const res = await fetchWithAuth(`${API_BASE}/backup/config`);
    if (!res || !res.ok) return;

    const config = await res.json();
    state.backupConfig = config;

    // Fill WebDAV fields
    if (config.webdav_url) {
      document.getElementById("webdav-url").value = config.webdav_url;
    }
    if (config.webdav_user) {
      document.getElementById("webdav-username").value = config.webdav_user;
    }
    if (config.webdav_password) {
      document.getElementById("webdav-password").value = config.webdav_password;
    }

    // Fill S3 fields
    if (config.s3_endpoint) {
      document.getElementById("s3-endpoint").value = config.s3_endpoint;
    }
    if (config.s3_region) {
      document.getElementById("s3-region").value = config.s3_region;
    }
    if (config.s3_bucket) {
      document.getElementById("s3-bucket").value = config.s3_bucket;
    }
    if (config.s3_access_key) {
      document.getElementById("s3-access-key").value = config.s3_access_key;
    }
    if (config.s3_secret_key) {
      document.getElementById("s3-secret-key").value = config.s3_secret_key;
    }

    // Fill auto backup settings
    const autoEnabled = config.auto_backup_enabled || false;
    document.getElementById("auto-backup-enabled").checked = autoEnabled;
    if (autoEnabled) {
      document.getElementById("auto-backup-options").style.display = "block";
    }

    if (config.backup_schedule) {
      document.getElementById("backup-schedule").value = config.backup_schedule;
    }

    if (config.last_backup_at) {
      document.getElementById("last-backup-info").style.display = "block";
      const date = new Date(config.last_backup_at);
      document.getElementById("last-backup-time").value = date.toLocaleString();
    }

    // Fill cleanup policy
    if (config.backup_retention_days !== undefined) {
      document.getElementById("backup-retention-days").value = config.backup_retention_days;
    }
    if (config.backup_max_count !== undefined) {
      document.getElementById("backup-max-count").value = config.backup_max_count;
    }
  } catch (e) {
    console.error("Failed to load backup config", e);
  }
}

function switchBackupTab(tab) {
  // Update tabs
  document
    .querySelectorAll(".smart-tabs .smart-tab-item")
    .forEach((btn) => btn.classList.remove("active"));

  // Handle both click events and direct calls
  const target = event && event.target ? event.target.closest(".smart-tab-item") : null;
  if (target) {
    target.classList.add("active");
  } else {
    // Fallback if called programmatically or event is missing
    const tabEl = document.querySelector(`.smart-tab-item[data-tab="${tab}"]`);
    if (tabEl) tabEl.classList.add("active");
  }

  // Update content
  document
    .querySelectorAll(".backup-config")
    .forEach((config) => config.classList.remove("active"));
  document.getElementById(`${tab}-config`).classList.add("active");
}

function toggleAutoBackup() {
  const enabled = document.getElementById("auto-backup-enabled").checked;
  document.getElementById("auto-backup-options").style.display = enabled ? "block" : "none";
}

async function saveBackupConfig() {
  const config = {
    webdav_url: document.getElementById("webdav-url").value,
    webdav_user: document.getElementById("webdav-username").value,
    webdav_password: document.getElementById("webdav-password").value,
    s3_endpoint: document.getElementById("s3-endpoint").value,
    s3_region: document.getElementById("s3-region").value,
    s3_bucket: document.getElementById("s3-bucket").value,
    s3_access_key: document.getElementById("s3-access-key").value,
    s3_secret_key: document.getElementById("s3-secret-key").value,
    auto_backup_enabled: document.getElementById("auto-backup-enabled").checked,
    backup_schedule: document.getElementById("backup-schedule").value,
    backup_retention_days: parseInt(document.getElementById("backup-retention-days").value) || 0,
    backup_max_count: parseInt(document.getElementById("backup-max-count").value) || 0,
  };

  try {
    const res = await fetchWithAuth(`${API_BASE}/backup/config`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(config),
    });

    if (!res) return;

    if (res.ok) {
      alert(t("save_success") || "Configuration saved successfully");
      await loadBackupConfig(); // Reload config
    } else {
      const error = await res.json();
      alert(t("save_failed") || "Failed to save configuration: " + error.error);
    }
  } catch (e) {
    alert(t("save_failed") || "Failed to save configuration");
  }
}

// Backup
async function backup(type) {
  if (!confirm(`${t("backup_confirm")} ${type}?`)) return;
  try {
    const res = await fetchWithAuth(`${API_BASE}/backup/${type}`, {
      method: "POST",
    });
    if (!res) return;

    const data = await res.json();
    if (res.ok) {
      alert(`${t("backup_success")}: ${data.file}`);
    } else {
      alert(`${t("backup_failed")}: ${data.error}`);
    }
  } catch (e) {
    alert(t("backup_failed"));
  }
}

// Restore
async function restore(type) {
  if (
    !confirm(
      `${t("restore_confirm") || "Restore from backup? Current data will be backed up first."}`,
    )
  )
    return;
  try {
    const res = await fetchWithAuth(`${API_BASE}/restore/${type}`, {
      method: "POST",
    });
    if (!res) return;

    const data = await res.json();
    if (res.ok) {
      let message = t("restore_success") || "Database restored successfully";

      // Check if restart is required
      if (data.restart_required && data.warning) {
        message += "\n\n⚠️ " + data.warning;
        alert(message);
      } else {
        alert(message);
      }

      // Reload notes
      await fetchNotes();
    } else {
      alert(`${t("restore_failed") || "Restore failed"}: ${data.error}`);
    }
  } catch (e) {
    alert(t("restore_failed") || "Restore failed");
  }
}

// Show backup list modal
async function showBackupList(type) {
  closeModal("backup-modal");

  // Show loading state
  document.getElementById("backup-list-loading").style.display = "block";
  document.getElementById("backup-list-content").style.display = "none";
  document.getElementById("backup-list-empty").style.display = "none";

  showModal("backup-list-modal");

  try {
    const res = await fetchWithAuth(`${API_BASE}/backup/list/${type}`);
    if (!res || !res.ok) {
      alert(t("load_failed") || "Failed to load backup list");
      closeModal("backup-list-modal");
      return;
    }

    const data = await res.json();
    const backups = data.backups || [];

    // Hide loading
    document.getElementById("backup-list-loading").style.display = "none";

    if (backups.length === 0) {
      document.getElementById("backup-list-empty").style.display = "block";
      return;
    }

    // Show content
    document.getElementById("backup-list-content").style.display = "block";

    // Render table
    const tbody = document.getElementById("backup-list-tbody");
    tbody.innerHTML = "";

    backups.forEach((backup) => {
      const row = document.createElement("tr");
      const date = new Date(backup.created_at).toLocaleString();
      const size = formatFileSize(backup.size);

      row.innerHTML = `
        <td>${escapeHtml(backup.filename)}</td>
        <td>${size}</td>
        <td>${date}</td>
        <td>
          <button class="btn-secondary btn-small" onclick="verifyBackup('${type}', '${escapeHtml(backup.filename).replace(/'/g, "\\'")}')">
            <i data-feather="check-circle"></i> ${t("backup_verify") || "Verify"}
          </button>
          <button class="btn-primary btn-small" onclick="restoreFromFile('${type}', '${escapeHtml(backup.filename).replace(/'/g, "\\'")}')">
            <i data-feather="upload"></i> ${t("backup_restore") || "Restore"}
          </button>
        </td>
      `;
      tbody.appendChild(row);
    });

    // Refresh icons
    feather.replace();
  } catch (e) {
    console.error("Load backup list error:", e);
    alert(t("load_failed") || "Failed to load backup list");
    closeModal("backup-list-modal");
  }
}

// Verify backup file
async function verifyBackup(type, filename) {
  if (!confirm(t("backup_verify_confirm") || `Verify backup "${filename}"?`)) {
    return;
  }

  try {
    const res = await fetchWithAuth(`${API_BASE}/backup/verify/${type}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ filename: filename }),
    });

    if (!res || !res.ok) {
      alert(t("backup_verify_failed") || "Backup verification failed");
      return;
    }

    const result = await res.json();

    if (result.valid) {
      let message = t("backup_verify_success") || "Backup is valid!";
      message += "\n\n";
      message += `${t("backup_files") || "Files"}: ${result.file_count}\n`;
      message += `${t("backup_total_size") || "Total Size"}: ${formatFileSize(result.total_size)}\n\n`;
      message += t("backup_details") || "Details:";

      result.file_checks.forEach((check) => {
        const status = check.exists ? "✓" : "✗";
        message += `\n${status} ${check.path}`;
        if (check.error) {
          message += ` - ${check.error}`;
        }
      });

      alert(message);
    } else {
      let message = t("backup_verify_failed") || "Backup verification failed!";
      if (result.error) {
        message += "\n\n" + result.error;
      }
      alert(message);
    }
  } catch (e) {
    console.error("Verify backup error:", e);
    alert(t("backup_verify_failed") || "Backup verification failed");
  }
}

// Restore from specific backup file
async function restoreFromFile(type, filename) {
  if (
    !confirm(
      `${t("restore_confirm_file") || "Restore from backup"} "${filename}"?\n\n${t("restore_confirm") || "Current data will be backed up first."}`,
    )
  ) {
    return;
  }

  try {
    const res = await fetchWithAuth(`${API_BASE}/restore/${type}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ filename: filename }),
    });

    if (!res) return;

    const data = await res.json();
    if (res.ok) {
      let message = t("restore_success") || "Database restored successfully";

      // Check if restart is required
      if (data.restart_required && data.warning) {
        message += "\n\n⚠️ " + data.warning;
        alert(message);
      } else {
        alert(message);
      }

      // Close backup list modal
      closeModal("backup-list-modal");

      // Reload notes
      await fetchNotes();
    } else {
      alert(`${t("restore_failed") || "Restore failed"}: ${data.error}`);
    }
  } catch (e) {
    console.error("Restore error:", e);
    alert(t("restore_failed") || "Restore failed");
  }
}

// Format file size to human readable format
function formatFileSize(bytes) {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + " " + sizes[i];
}

// Utility
function escapeHtml(text) {
  if (!text) return "";
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#039;");
}

// Custom notification system to replace browser alert
function showNotification(message, type = "info", duration = 3000) {
  // Create notification container if it doesn't exist
  let container = document.getElementById("notification-container");
  if (!container) {
    container = document.createElement("div");
    container.id = "notification-container";
    container.style.cssText = `
      position: fixed;
      top: 20px;
      right: 20px;
      z-index: 10001;
      display: flex;
      flex-direction: column;
      gap: 10px;
    `;
    document.body.appendChild(container);
  }

  // Create notification element
  const notification = document.createElement("div");
  notification.className = "custom-notification";

  // Set background color based on type
  let backgroundColor = "#5bc0de"; // info
  let icon = "info";
  if (type === "success") {
    backgroundColor = "#5cb85c";
    icon = "check-circle";
  } else if (type === "error") {
    backgroundColor = "#d9534f";
    icon = "alert-circle";
  } else if (type === "warning") {
    backgroundColor = "#f0ad4e";
    icon = "alert-triangle";
  }

  notification.style.cssText = `
    background: ${backgroundColor};
    color: white;
    padding: 15px 20px;
    border-radius: 8px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    display: flex;
    align-items: center;
    gap: 12px;
    min-width: 300px;
    max-width: 500px;
    animation: slideInRight 0.3s ease;
    cursor: pointer;
  `;

  notification.innerHTML = `
    <i data-feather="${icon}" style="flex-shrink: 0;"></i>
    <span style="flex: 1;">${escapeHtml(message)}</span>
    <i data-feather="x" style="flex-shrink: 0; opacity: 0.7;"></i>
  `;

  container.appendChild(notification);
  feather.replace();

  // Auto remove after duration
  const timeoutId = setTimeout(() => {
    removeNotification(notification);
  }, duration);

  // Click to dismiss
  notification.onclick = () => {
    clearTimeout(timeoutId);
    removeNotification(notification);
  };
}

function removeNotification(notification) {
  notification.style.animation = "slideOutRight 0.3s ease";
  setTimeout(() => {
    if (notification.parentNode) {
      notification.parentNode.removeChild(notification);
    }
  }, 300);
}

// Add CSS animations for notifications
if (!document.getElementById("notification-styles")) {
  const style = document.createElement("style");
  style.id = "notification-styles";
  style.textContent = `
    @keyframes slideInRight {
      from {
        transform: translateX(400px);
        opacity: 0;
      }
      to {
        transform: translateX(0);
        opacity: 1;
      }
    }
    @keyframes slideOutRight {
      from {
        transform: translateX(0);
        opacity: 1;
      }
      to {
        transform: translateX(400px);
        opacity: 0;
      }
    }
  `;
  document.head.appendChild(style);
}

// ====== Password Protection Functions ======

function showPasswordModal() {
  if (!state.currentNote) return;

  const isLocked = state.currentNote.is_locked;

  if (isLocked) {
    // Show unlock form
    document.getElementById("password-modal-title").textContent =
      t("unlock_note") || "Unlock Note";
    document.getElementById("password-set-form").style.display = "none";
    document.getElementById("password-unlock-form").style.display = "block";
    document.getElementById("note-password-unlock").value = "";

    // Update labels for unlock form
    const unlockPasswordLabel = document.getElementById("unlock-password-label");
    if (unlockPasswordLabel) unlockPasswordLabel.textContent = t("password_required") || "Enter Password to Unlock";

    const unlockBtn = document.getElementById("unlock-btn");
    if (unlockBtn) unlockBtn.textContent = t("unlock_note") || "Unlock";

    const unlockInput = document.getElementById("note-password-unlock");
    if (unlockInput) unlockInput.placeholder = t("password_required") || "Enter password";
  } else {
    // Show set password form
    document.getElementById("password-modal-title").textContent =
      t("set_password") || "Set Password";
    document.getElementById("password-set-form").style.display = "block";
    document.getElementById("password-unlock-form").style.display = "none";
    document.getElementById("note-password").value = "";
    document.getElementById("note-password-confirm").value = "";

    // Update labels for set password form
    const passwordLabel = document.getElementById("password-label");
    if (passwordLabel) passwordLabel.textContent = t("password") || "Password";

    const passwordConfirmLabel = document.getElementById("password-confirm-label");
    if (passwordConfirmLabel) passwordConfirmLabel.textContent = t("confirm_password") || "Confirm Password";

    const setPasswordBtn = document.getElementById("set-password-btn");
    if (setPasswordBtn) setPasswordBtn.textContent = t("set_password") || "Set Password";

    const removePasswordBtn = document.getElementById("remove-password-btn");
    if (removePasswordBtn) removePasswordBtn.textContent = t("remove_password_confirm") || "Remove Password";

    const passwordInput = document.getElementById("note-password");
    if (passwordInput) passwordInput.placeholder = t("password_required") || "Enter password";

    const passwordConfirmInput = document.getElementById("note-password-confirm");
    if (passwordConfirmInput) passwordConfirmInput.placeholder = t("confirm_password") || "Confirm password";
  }

  showModal("password-modal");
}

async function setNotePassword() {
  if (!state.currentNote) return;

  const password = document.getElementById("note-password").value;
  const confirm = document.getElementById("note-password-confirm").value;

  if (!password) {
    showNotification(t("password_required") || "Please enter a password", "warning");
    return;
  }

  if (password !== confirm) {
    showNotification(t("password_not_match") || "Passwords do not match", "warning");
    return;
  }

  if (password.length < 4) {
    showNotification(t("password_too_short") || "Password must be at least 4 characters", "warning");
    return;
  }

  try {
    // Send plain password to backend - backend will hash it with argon2
    await updateNote(state.currentNote.id, {
      password: password,
      is_locked: true,
    });

    state.currentNote.is_locked = true;

    // Clear unlock status (user needs to re-enter password)
    clearNoteUnlock(state.currentNote.id);

    showNotification(t("password_set_success") || "Password set successfully", "success");
    closeModal("password-modal");
    renderEditor();
  } catch (e) {
    showNotification(t("password_set_failed") || "Failed to set password", "error");
  }
}

async function removeNotePassword() {
  if (!state.currentNote) return;

  if (!confirm(t("remove_password_confirm") || "Remove password protection?")) {
    return;
  }

  try {
    // Send empty password to remove protection
    await updateNote(state.currentNote.id, {
      password: "",
      is_locked: false,
    });

    state.currentNote.is_locked = false;
    clearNoteUnlock(state.currentNote.id);

    showNotification(t("password_removed") || "Password removed", "success");
    closeModal("password-modal");
    renderEditor();
  } catch (e) {
    showNotification(t("password_remove_failed") || "Failed to remove password", "error");
  }
}

async function unlockNote() {
  if (!state.currentNote) return;

  const password = document.getElementById("note-password-unlock").value;

  if (!password) {
    showNotification(t("password_required") || "Please enter password", "warning");
    return;
  }

  try {
    // Call backend API to verify password
    const res = await fetchWithAuth(`${API_BASE}/notes/${state.currentNote.id}/verify-password`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ password: password }),
    });

    if (!res) return;

    if (res.ok) {
      const data = await res.json();
      // Update note with decrypted content from backend
      if (data.note) {
        state.currentNote = data.note;
      }

      // Unlock successful - mark as unlocked in session
      markNoteUnlocked(state.currentNote.id);
      closeModal("password-modal");
      renderEditor();
      showNotification(t("unlock_success") || "Note unlocked", "success");
    } else {
      showNotification(t("password_incorrect") || "Incorrect password", "error");
      document.getElementById("note-password-unlock").value = "";
    }
  } catch (e) {
    console.error("Unlock error:", e);
    showNotification(t("password_incorrect") || "Failed to unlock", "error");
    document.getElementById("note-password-unlock").value = "";
  }
}

// ====== Markdown Preview Functions ======

function toggleMarkdownMode() {
  // Toggle between source and preview modes
  state.markdownViewMode =
    state.markdownViewMode === "source" ? "preview" : "source";

  renderEditor();

  // Toggle background image for preview mode
  const editorContent = document.querySelector(".editor-content");
  const editorPanel = document.querySelector(".editor-panel");

  if (editorContent && editorPanel) {
    if (state.markdownViewMode === "preview") {
      editorContent.classList.add("preview-mode");
      editorPanel.classList.add("preview-mode");
      updateMarkdownPreview();
    } else {
      editorContent.classList.remove("preview-mode");
      editorPanel.classList.remove("preview-mode");
    }
  }
}

// Sync content from source to preview in real-time
function syncPreviewWithSource() {
  if (state.markdownViewMode === "preview" && state.currentNote) {
    // Ensure both textarea and state have the same content
    const textarea = document.getElementById('markdown-editor');
    if (textarea) {
      // Sync textarea value to state if they differ
      if (textarea.value !== state.currentNote.content) {
        state.currentNote.content = textarea.value;
      }
      updateMarkdownPreview();
    }
  }
}

async function updateMarkdownPreview() {
  if (state.markdownViewMode !== "preview") return;

  const previewEl = document.getElementById("markdown-preview-content");
  if (!previewEl) return;

  const content = state.currentNote?.content || "";
  const note = state.currentNote;

  try {
    // Use marked.js to render markdown on client-side
    if (typeof marked === "undefined") {
      console.error("marked.js is not loaded");
      previewEl.innerHTML =
        '<p style="color: #d9534f;">Markdown library not loaded</p>';
      return;
    }

    const html = marked.parse(content);

    // Add tags display at the top of preview if tags exist
    let tagsHtml = '';
    if (note.tags && note.tags.length > 0) {
      tagsHtml = `
        <div class="preview-tags" style="
          margin-bottom: 15px;
          padding: 10px;
          background: var(--bg-secondary);
          border-radius: 6px;
          border: 1px solid var(--border-light);
        ">
          <div style="display: flex; align-items: center; gap: 8px; margin-bottom: 6px;">
            <i data-feather="tag" style="width: 14px; height: 14px; color: var(--text-secondary);"></i>
            <span style="font-size: 13px; color: var(--text-secondary);">${t("tags") || "Tags"}</span>
          </div>
          <div style="display: flex; flex-wrap: wrap; gap: 5px;">
            ${note.tags.map(tag => `
              <span class="tag" style="
                background: ${tag.color || 'var(--primary-color)'};
                color: white;
                padding: 3px 8px;
                border-radius: 12px;
                font-size: 12px;
                font-weight: 500;
              ">${escapeHtml(tag.name)}</span>
            `).join('')}
          </div>
        </div>
      `;
    }

    previewEl.innerHTML = tagsHtml + html;

    // Refresh Feather icons in the rendered HTML
    feather.replace();
  } catch (e) {
    console.error("Failed to render markdown:", e);
    previewEl.innerHTML =
      '<p style="color: #d9534f;">Failed to render preview</p>';
  }
}

// ====== Sidebar Expander Functions ======

function toggleSidebar() {
  // Check if mobile view
  if (window.innerWidth <= 768) {
    // Mobile: Toggle sidebar drawer
    const sidebar = document.getElementById('sidebar');
    const overlay = document.getElementById('sidebar-overlay');

    if (!sidebar || !overlay) return;

    sidebar.classList.toggle('mobile-visible');
    overlay.classList.toggle('visible');
  } else {
    // Desktop: Toggle sidebar collapse
    state.sidebarExpanded = !state.sidebarExpanded;
    const sidebar = document.querySelector(".sidebar");
    const body = document.body;

    if (state.sidebarExpanded) {
      sidebar.classList.remove("collapsed");
      body.classList.remove("sidebar-collapsed");
    } else {
      sidebar.classList.add("collapsed");
      body.classList.add("sidebar-collapsed");
    }

    // Save preference to localStorage
    localStorage.setItem("sidebarExpanded", state.sidebarExpanded);
  }
}

// Load sidebar state on init
window.addEventListener("DOMContentLoaded", () => {
  const savedState = localStorage.getItem("sidebarExpanded");
  if (savedState !== null) {
    state.sidebarExpanded = savedState === "true";
    const sidebar = document.querySelector(".sidebar");
    const body = document.body;
    if (!state.sidebarExpanded) {
      sidebar.classList.add("collapsed");
      body.classList.add("sidebar-collapsed");
    }
  }
});

// Toggle attachments section visibility
function toggleAttachments() {
  state.attachmentsExpanded = !state.attachmentsExpanded;
  renderEditor();
}

// ====== User Management Functions ======

async function showUserManagement() {
  if (!currentUser || currentUser.role !== "admin") {
    alert("Access denied. Admin only.");
    return;
  }

  // Update modal title with translation
  document.querySelector(
    "#user-management-modal .modal-header h2",
  ).textContent = t("user_management") || "User Management";

  // Update create user section labels
  const createUserSection = document.querySelector(
    "#user-management-modal .form-section:nth-child(1) h3",
  );
  if (createUserSection)
    createUserSection.textContent = t("create_new_user") || "Create New User";

  const formLabels = document.querySelectorAll(
    "#user-management-modal .form-section:nth-child(1) label",
  );
  if (formLabels[0]) formLabels[0].textContent = t("username") + " *";
  if (formLabels[1]) formLabels[1].textContent = t("email");
  if (formLabels[2]) formLabels[2].textContent = t("nickname");
  if (formLabels[3]) formLabels[3].textContent = t("password") + " *";
  if (formLabels[4]) formLabels[4].textContent = t("role") + " *";

  // Update input placeholders
  const usernameInput = document.getElementById("new-user-username");
  if (usernameInput)
    usernameInput.placeholder = t("enter_username") || "Enter username";

  const emailInput = document.getElementById("new-user-email");
  if (emailInput) emailInput.placeholder = t("enter_email") || "Enter email";

  const nicknameInput = document.getElementById("new-user-nickname");
  if (nicknameInput)
    nicknameInput.placeholder =
      t("enter_nickname") || "Enter nickname (optional)";

  const passwordInput = document.getElementById("new-user-password");
  if (passwordInput)
    passwordInput.placeholder =
      t("enter_password_min") || "Enter password (min 6 chars)";

  // Update role dropdown options
  const roleSelect = document.getElementById("new-user-role");
  if (roleSelect) {
    roleSelect.options[0].text = t("user_role") || "User";
    roleSelect.options[1].text = t("admin") || "Admin";
  }

  // Update create button
  const createBtn = document.querySelector(
    "#user-management-modal .form-section:nth-child(1) .btn-primary",
  );
  if (createBtn)
    createBtn.innerHTML = `<i data-feather="user-plus"></i> ${t("create_user") || "Create User"}`;

  // Update user list section
  const userListSection = document.querySelector(
    "#user-management-modal .form-section:nth-child(2) h3",
  );
  if (userListSection)
    userListSection.textContent = t("all_users") || "All Users";

  // Update table headers
  const tableHeaders = document.querySelectorAll(
    "#user-management-modal .user-table th",
  );
  if (tableHeaders[0])
    tableHeaders[0].textContent = t("username") || "Username";
  if (tableHeaders[1])
    tableHeaders[1].textContent = t("nickname") || "Nickname";
  if (tableHeaders[2]) tableHeaders[2].textContent = t("email") || "Email";
  if (tableHeaders[3]) tableHeaders[3].textContent = t("role") || "Role";
  if (tableHeaders[4])
    tableHeaders[4].textContent = t("created_at") || "Created";
  if (tableHeaders[5]) tableHeaders[5].textContent = t("actions") || "Actions";

  // Clear create user form
  document.getElementById("new-user-username").value = "";
  document.getElementById("new-user-email").value = "";
  document.getElementById("new-user-nickname").value = "";
  document.getElementById("new-user-password").value = "";
  document.getElementById("new-user-role").value = "user";

  // Show modal
  showModal("user-management-modal");

  // Load users
  await loadAllUsers();

  // Refresh icons
  setTimeout(() => feather.replace(), 10);
}

async function loadAllUsers() {
  try {
    const res = await fetchWithAuth(`${API_BASE}/users`);
    if (!res || !res.ok) {
      alert(t("load_failed") || "Failed to load users");
      return;
    }

    const users = await res.json();
    renderUserList(users);
  } catch (e) {
    console.error("Load users error:", e);
    alert(t("load_failed") || "Failed to load users");
  }
}

function renderUserList(users) {
  const tbody = document.getElementById("user-list-body");
  tbody.innerHTML = "";

  if (!users || users.length === 0) {
    tbody.innerHTML =
      '<tr><td colspan="6" style="text-align: center; padding: 40px; color: #999;">No users found</td></tr>';
    return;
  }

  users.forEach((user) => {
    const row = document.createElement("tr");
    row.style.borderBottom = "1px solid #eee";

    const createdDate = new Date(user.created_at).toLocaleDateString();
    const roleLabel = user.role === "admin" ? t("admin") : t("user_role");
    const roleBadgeColor = user.role === "admin" ? "#d9534f" : "#5bc0de";

    row.innerHTML = `
            <td style="padding: 12px;">
                <div style="display: flex; align-items: center; gap: 10px;">
                    <div style="width: 32px; height: 32px; border-radius: 50%; overflow: hidden; background: #f0f0f0;">
                        <img src="${user.avatar || "/static/img/default-avatar.svg"}" alt="Avatar" style="width: 100%; height: 100%; object-fit: cover;">
                    </div>
                    <strong>${escapeHtml(user.username)}</strong>
                </div>
            </td>
            <td style="padding: 12px; color: #666;">${escapeHtml(user.nickname || "-")}</td>
            <td style="padding: 12px; color: #666;">${escapeHtml(user.email || "-")}</td>
            <td style="padding: 12px;">
                <span style="display: inline-block; padding: 4px 8px; background: ${roleBadgeColor}; color: white; border-radius: 4px; font-size: 11px; font-weight: 600; text-transform: uppercase;">${roleLabel}</span>
            </td>
            <td style="padding: 12px; color: #999; font-size: 13px;">${createdDate}</td>
            <td style="padding: 12px; text-align: center;">
                ${user.id !== currentUser.id
        ? `
                <button class="btn-icon" onclick="deleteUserConfirm(${user.id}, '${escapeHtml(user.username).replace(/'/g, "\\'")}')" title="${t("delete_user") || "Delete User"}">
                    <i data-feather="trash-2"></i>
                </button>
                `
        : `<span style="color: #ccc; font-size: 12px;">${t("current_user") || "Current User"}</span>`
      }
            </td>
        `;

    tbody.appendChild(row);
  });

  // Refresh Feather Icons in the table
  feather.replace();
}

async function createNewUser() {
  const username = document.getElementById("new-user-username").value.trim();
  const email = document.getElementById("new-user-email").value.trim();
  const nickname = document.getElementById("new-user-nickname").value.trim();
  const password = document.getElementById("new-user-password").value;
  const role = document.getElementById("new-user-role").value;

  if (!username || !password) {
    alert(
      t("username_password_required") || "Username and password are required",
    );
    return;
  }

  if (password.length < 6) {
    alert(t("password_too_short") || "Password must be at least 6 characters");
    return;
  }

  try {
    const res = await fetchWithAuth(`${API_BASE}/users`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        username: username,
        email: email,
        nickname: nickname,
        password: password,
        role: role,
      }),
    });

    if (!res || !res.ok) {
      const error = res ? await res.json() : { error: "Network error" };
      alert(
        t("create_user_failed") ||
        "Failed to create user: " + (error.error || "Unknown error"),
      );
      return;
    }

    // Clear form
    document.getElementById("new-user-username").value = "";
    document.getElementById("new-user-email").value = "";
    document.getElementById("new-user-nickname").value = "";
    document.getElementById("new-user-password").value = "";
    document.getElementById("new-user-role").value = "user";

    alert(t("user_created") || "User created successfully");

    // Reload user list
    await loadAllUsers();
  } catch (e) {
    console.error("Create user error:", e);
    alert(t("create_user_failed") || "Failed to create user");
  }
}

function deleteUserConfirm(userId, username) {
  if (!confirm(`${t("delete_user_confirm") || "Delete user"} "${username}"?`)) {
    return;
  }
  deleteUserById(userId);
}

async function deleteUserById(userId) {
  try {
    const res = await fetchWithAuth(`${API_BASE}/users/${userId}`, {
      method: "DELETE",
    });

    if (!res || !res.ok) {
      const error = res ? await res.json() : { error: "Network error" };
      alert(
        t("delete_user_failed") ||
        "Failed to delete user: " + (error.error || "Unknown error"),
      );
      return;
    }

    alert(t("user_deleted") || "User deleted successfully");

    // Reload user list
    await loadAllUsers();
  } catch (e) {
    console.error("Delete user error:", e);
    alert(t("delete_user_failed") || "Failed to delete user");
  }
}

function showProfileSettings() {
  if (!currentUser) return;

  // Update modal title and labels with translations
  document.querySelector("#profile-modal .modal-header h2").textContent =
    t("profile_settings") || "Profile Settings";

  // Avatar section
  const avatarSection = document.querySelector(
    "#profile-modal .form-section:nth-child(1) h3",
  );
  if (avatarSection) avatarSection.textContent = t("avatar") || "Avatar";

  const uploadBtn = document.querySelector("#profile-modal .btn-primary");
  if (uploadBtn)
    uploadBtn.innerHTML = `<i data-feather="upload"></i> ${t("upload_avatar") || "Upload Avatar"}`;

  const avatarHint = document.querySelector(
    "#profile-modal .form-section:nth-child(1) p",
  );
  if (avatarHint)
    avatarHint.textContent = t("avatar_hint") || "JPG, PNG or GIF (Max 2MB)";

  // Email section
  const emailSection = document.querySelector(
    "#profile-modal .form-section:nth-child(2) h3",
  );
  if (emailSection) emailSection.textContent = t("email") || "Email";

  const emailLabel = document.querySelector(
    "#profile-modal .form-section:nth-child(2) label",
  );
  if (emailLabel)
    emailLabel.textContent = t("email_address") || "Email Address";

  const emailInput = document.getElementById("profile-email");
  if (emailInput)
    emailInput.placeholder = t("enter_email") || "Enter email address";

  const updateEmailBtn = document.querySelector(
    "#profile-modal .form-section:nth-child(2) .btn-primary",
  );
  if (updateEmailBtn)
    updateEmailBtn.innerHTML = `<i data-feather="save"></i> ${t("update_email") || "Update Email"}`;

  // Nickname section
  const nicknameSection = document.querySelector(
    "#profile-modal .form-section:nth-child(3) h3",
  );
  if (nicknameSection)
    nicknameSection.textContent = t("nickname") || "Nickname";

  const nicknameLabel = document.querySelector(
    "#profile-modal .form-section:nth-child(3) label",
  );
  if (nicknameLabel) nicknameLabel.textContent = t("nickname") || "Nickname";

  const nicknameInput = document.getElementById("profile-nickname");
  if (nicknameInput)
    nicknameInput.placeholder =
      t("enter_nickname") || "Enter nickname (optional)";

  const updateNicknameBtn = document.querySelector(
    "#profile-modal .form-section:nth-child(3) .btn-primary",
  );
  if (updateNicknameBtn)
    updateNicknameBtn.innerHTML = `<i data-feather="save"></i> ${t("update_nickname") || "Update Nickname"}`;

  // Password section
  const passwordSection = document.querySelector(
    "#profile-modal .form-section:nth-child(4) h3",
  );
  if (passwordSection)
    passwordSection.textContent = t("change_password") || "Change Password";

  const labels = document.querySelectorAll(
    "#profile-modal .form-section:nth-child(4) label",
  );
  if (labels[0]) labels[0].textContent = t("old_password") || "Old Password";
  if (labels[1]) labels[1].textContent = t("new_password") || "New Password";
  if (labels[2])
    labels[2].textContent = t("confirm_password") || "Confirm New Password";

  const oldPasswordInput = document.getElementById("profile-old-password");
  if (oldPasswordInput)
    oldPasswordInput.placeholder =
      t("enter_old_password") || "Enter old password";

  const newPasswordInput = document.getElementById("profile-new-password");
  if (newPasswordInput)
    newPasswordInput.placeholder =
      t("enter_new_password") || "Enter new password (min 6 characters)";

  const confirmPasswordInput = document.getElementById(
    "profile-confirm-password",
  );
  if (confirmPasswordInput)
    confirmPasswordInput.placeholder =
      t("confirm_new_password") || "Confirm new password";

  const changePasswordBtn = document.querySelector(
    "#profile-modal .form-section:nth-child(4) .btn-primary",
  );
  if (changePasswordBtn)
    changePasswordBtn.innerHTML = `<i data-feather="lock"></i> ${t("change_password") || "Change Password"}`;

  // Load current user data into form
  document.getElementById("profile-email").value = currentUser.email || "";
  document.getElementById("profile-nickname").value =
    currentUser.nickname || "";
  document.getElementById("profile-avatar-preview").src =
    currentUser.avatar || "/static/img/default-avatar.svg";

  // Clear password fields
  document.getElementById("profile-old-password").value = "";
  document.getElementById("profile-new-password").value = "";
  document.getElementById("profile-confirm-password").value = "";

  showModal("profile-modal");

  // Refresh icons
  setTimeout(() => feather.replace(), 10);
}

async function handleAvatarSelect(event) {
  const file = event.target.files[0];
  if (!file) return;

  // Validate file size (max 2MB)
  if (file.size > 2 * 1024 * 1024) {
    alert("File size must be less than 2MB");
    return;
  }

  // Validate file type
  if (!file.type.startsWith("image/")) {
    alert("Please select an image file");
    return;
  }

  // Preview image
  const reader = new FileReader();
  reader.onload = (e) => {
    document.getElementById("profile-avatar-preview").src = e.target.result;
  };
  reader.readAsDataURL(file);

  // Upload avatar
  try {
    const formData = new FormData();
    formData.append("file", file);

    const res = await fetchWithAuth(
      `${API_BASE}/users/${currentUser.id}/avatar`,
      {
        method: "POST",
        body: formData,
      },
    );

    if (!res || !res.ok) {
      alert(t("upload_failed") || "Failed to upload avatar");
      return;
    }

    const data = await res.json();

    // Update current user avatar
    currentUser.avatar = data.avatar;
    localStorage.setItem("user", JSON.stringify(currentUser));

    // Update sidebar avatar
    document.getElementById("user-avatar-img").src = data.avatar;

    alert(t("upload_success") || "Avatar uploaded successfully");
  } catch (e) {
    console.error("Avatar upload error:", e);
    alert(t("upload_failed") || "Failed to upload avatar");
  }
}

async function updateProfileEmail() {
  const email = document.getElementById("profile-email").value.trim();

  if (!email) {
    alert(t("email_required") || "Please enter email address");
    return;
  }

  // Basic email validation
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  if (!emailRegex.test(email)) {
    alert(t("email_invalid") || "Please enter a valid email address");
    return;
  }

  try {
    const res = await fetchWithAuth(`${API_BASE}/users/${currentUser.id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ email: email }),
    });

    if (!res || !res.ok) {
      const error = res ? await res.json() : { error: "Network error" };
      alert(
        t("update_failed") ||
        "Failed to update email: " + (error.error || "Unknown error"),
      );
      return;
    }

    // Update current user email
    currentUser.email = email;
    localStorage.setItem("user", JSON.stringify(currentUser));

    alert(t("update_success") || "Email updated successfully");
  } catch (e) {
    console.error("Email update error:", e);
    alert(t("update_failed") || "Failed to update email");
  }
}

async function updateProfileNickname() {
  const nickname = document.getElementById("profile-nickname").value.trim();

  try {
    const res = await fetchWithAuth(`${API_BASE}/users/${currentUser.id}`, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ nickname: nickname }),
    });

    if (!res || !res.ok) {
      const error = res ? await res.json() : { error: "Network error" };
      alert(
        t("update_failed") ||
        "Failed to update nickname: " + (error.error || "Unknown error"),
      );
      return;
    }

    // Update current user nickname
    currentUser.nickname = nickname;
    localStorage.setItem("user", JSON.stringify(currentUser));

    alert(t("update_success") || "Nickname updated successfully");
  } catch (e) {
    console.error("Nickname update error:", e);
    alert(t("update_failed") || "Failed to update nickname");
  }
}

async function updateProfilePassword() {
  const oldPassword = document.getElementById("profile-old-password").value;
  const newPassword = document.getElementById("profile-new-password").value;
  const confirmPassword = document.getElementById(
    "profile-confirm-password",
  ).value;

  if (!oldPassword || !newPassword || !confirmPassword) {
    alert(t("password_required") || "Please fill in all password fields");
    return;
  }

  if (newPassword.length < 6) {
    alert(
      t("password_too_short") || "New password must be at least 6 characters",
    );
    return;
  }

  if (newPassword !== confirmPassword) {
    alert(t("password_not_match") || "New passwords do not match");
    return;
  }

  try {
    const res = await fetchWithAuth(
      `${API_BASE}/users/${currentUser.id}/password`,
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          old_password: oldPassword,
          new_password: newPassword,
        }),
      },
    );

    if (!res || !res.ok) {
      const error = res ? await res.json() : { error: "Network error" };
      alert(
        t("password_change_failed") ||
        "Failed to change password: " + (error.error || "Unknown error"),
      );
      return;
    }

    // Clear password fields
    document.getElementById("profile-old-password").value = "";
    document.getElementById("profile-new-password").value = "";
    document.getElementById("profile-confirm-password").value = "";

    alert(t("password_changed") || "Password changed successfully");
  } catch (e) {
    console.error("Password change error:", e);
    alert(t("password_change_failed") || "Failed to change password");
  }
}

// ====== Attachment Functions ======

async function loadAttachments(noteId) {
  try {
    const res = await fetchWithAuth(`${API_BASE}/notes/${noteId}/attachments`);
    if (!res) return [];

    const attachments = await res.json();
    return attachments || [];
  } catch (e) {
    console.error("Failed to load attachments:", e);
    return [];
  }
}

async function uploadAttachment(noteId) {
  const input = document.createElement("input");
  input.type = "file";
  input.multiple = true;

  input.onchange = async (e) => {
    const files = e.target.files;
    if (!files || files.length === 0) return;

    for (const file of files) {
      await uploadSingleFile(noteId, file);
    }

    // Refresh attachment list
    await renderAttachments(noteId);
  };

  input.click();
}

async function uploadSingleFile(noteId, file) {
  try {
    const formData = new FormData();
    formData.append("file", file);

    const token = getAuthToken();
    if (!token) {
      logout();
      return;
    }

    const res = await fetch(`${API_BASE}/notes/${noteId}/attachments`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${token}`,
      },
      body: formData,
    });

    if (res.status === 401) {
      logout();
      return;
    }

    if (!res.ok) {
      const error = await res.json();
      alert(`Upload failed: ${error.error || "Unknown error"}`);
      return;
    }

    const attachment = await res.json();
    console.log("Uploaded:", attachment);
    return attachment;
  } catch (e) {
    console.error("Upload error:", e);
    alert(`Upload failed: ${e.message}`);
  }
}

async function downloadAttachment(attachmentId, filename) {
  try {
    const token = getAuthToken();
    if (!token) {
      logout();
      return;
    }

    const url = `${API_BASE}/attachments/${attachmentId}/download`;
    const res = await fetch(url, {
      headers: {
        Authorization: `Bearer ${token}`,
      },
    });

    if (!res.ok) {
      alert("Download failed");
      return;
    }

    const blob = await res.blob();
    const downloadUrl = window.URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = downloadUrl;
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    window.URL.revokeObjectURL(downloadUrl);
    document.body.removeChild(a);
  } catch (e) {
    console.error("Download error:", e);
    alert("Download failed");
  }
}

async function deleteAttachment(attachmentId, noteId) {
  if (!confirm(t("delete_confirm") || "Delete this attachment?")) return;

  try {
    const res = await fetchWithAuth(`${API_BASE}/attachments/${attachmentId}`, {
      method: "DELETE",
    });

    if (!res) return;

    // Refresh attachment list
    await renderAttachments(noteId);
  } catch (e) {
    console.error("Delete attachment error:", e);
    alert("Failed to delete attachment");
  }
}

async function renderAttachments(noteId) {
  const container = document.getElementById("attachment-list");
  if (!container) return;

  const attachments = await loadAttachments(noteId);

  if (attachments.length === 0) {
    container.innerHTML =
      '<div class="no-attachments">' +
      (t("no_attachments") || "No attachments") +
      "</div>";
    return;
  }

  container.innerHTML = "";

  attachments.forEach((att) => {
    const item = document.createElement("div");
    item.className = "attachment-item";

    const fileSize = formatFileSize(att.file_size);
    const fileIcon = getFileIcon(att.mime_type);

    item.innerHTML = `
            <div class="attachment-icon">${fileIcon}</div>
            <div class="attachment-info">
                <div class="attachment-name">${escapeHtml(att.filename)}</div>
                <div class="attachment-size">${fileSize}</div>
            </div>
            <div class="attachment-actions">
                <button class="btn-icon" onclick="downloadAttachment(${att.id}, '${escapeHtml(att.filename).replace(/'/g, "\\'")}');" title="${t("download") || "Download"}">
                    <i data-feather="download"></i>
                </button>
                <button class="btn-icon" onclick="deleteAttachment(${att.id}, '${noteId}');" title="${t("delete") || "Delete"}">
                    <i data-feather="trash-2"></i>
                </button>
            </div>
        `;

    container.appendChild(item);
  });

  // Refresh Feather Icons
  feather.replace();
}

function formatFileSize(bytes) {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return Math.round((bytes / Math.pow(k, i)) * 100) / 100 + " " + sizes[i];
}

function getFileIcon(mimeType) {
  if (!mimeType) return "📄";
  if (mimeType.startsWith("image/")) return "🖼️";
  if (mimeType.startsWith("video/")) return "🎬";
  if (mimeType.startsWith("audio/")) return "🎵";
  if (mimeType.includes("pdf")) return "📕";
  if (mimeType.includes("word")) return "📘";
  if (mimeType.includes("excel") || mimeType.includes("spreadsheet"))
    return "📗";
  if (mimeType.includes("zip") || mimeType.includes("rar")) return "📦";
  return "📄";
}

// ====== Markdown Auto-Completion Functions ======

// Auto-completion state
let autocompleteState = {
  visible: false,
  selectedIndex: 0,
  suggestions: [],
  triggerPosition: 0,
};

// Markdown auto-completion patterns
const markdownCompletions = [
  {
    trigger: "#",
    suggestions: [
      { text: "# ", label: "H1 Heading", description: "Level 1 heading" },
      { text: "## ", label: "H2 Heading", description: "Level 2 heading" },
      { text: "### ", label: "H3 Heading", description: "Level 3 heading" },
      { text: "#### ", label: "H4 Heading", description: "Level 4 heading" },
    ],
  },
  {
    trigger: "-",
    suggestions: [
      { text: "- ", label: "Bullet List", description: "Unordered list item" },
      { text: "- [ ] ", label: "Task List", description: "Checkbox item" },
      {
        text: "---",
        label: "Horizontal Rule",
        description: "Horizontal divider",
      },
    ],
  },
  {
    trigger: "*",
    suggestions: [
      { text: "**", label: "Bold", description: "Bold text **text**" },
      { text: "*", label: "Italic", description: "Italic text *text*" },
      {
        text: "***",
        label: "Bold Italic",
        description: "Bold italic ***text***",
      },
    ],
  },
  {
    trigger: "1",
    suggestions: [
      { text: "1. ", label: "Numbered List", description: "Ordered list item" },
    ],
  },
  {
    trigger: "[",
    suggestions: [
      { text: "[]()", label: "Link", description: "Insert hyperlink" },
      { text: "![]()", label: "Image", description: "Insert image" },
    ],
  },
  {
    trigger: "`",
    suggestions: [
      { text: "`", label: "Inline Code", description: "Inline code `code`" },
      {
        text: "```\n\n```",
        label: "Code Block",
        description: "Multi-line code block",
      },
    ],
  },
  {
    trigger: ">",
    suggestions: [
      { text: "> ", label: "Blockquote", description: "Quote text" },
    ],
  },
  {
    trigger: "|",
    suggestions: [
      {
        text: "| Header 1 | Header 2 |\n| -------- | -------- |\n| Cell 1   | Cell 2   |",
        label: "Table",
        description: "Insert table",
      },
    ],
  },
];

function handleMarkdownInput(noteId, textarea) {
  // Update note content
  updateNoteDebounced(noteId, "content", textarea.value);

  // Sync preview with source in real-time
  syncPreviewWithSource();

  // Check for auto-completion triggers
  const cursorPos = textarea.selectionStart;
  const textBeforeCursor = textarea.value.substring(0, cursorPos);
  const lineStart = textBeforeCursor.lastIndexOf("\n") + 1;
  const currentLine = textBeforeCursor.substring(lineStart);

  // Only trigger at start of line or after space
  const isAtLineStart = currentLine.trim() === currentLine;
  if (!isAtLineStart) {
    hideAutocomplete();
    return;
  }

  // Check if current line matches any trigger
  let matchedCompletion = null;
  for (const completion of markdownCompletions) {
    if (currentLine.startsWith(completion.trigger)) {
      matchedCompletion = completion;
      break;
    }
  }

  if (matchedCompletion && currentLine.length <= 4) {
    showAutocomplete(textarea, matchedCompletion.suggestions);
  } else {
    hideAutocomplete();
  }
}

function showAutocomplete(textarea, suggestions) {
  const autocompleteEl = document.getElementById("markdown-autocomplete");
  if (!autocompleteEl || suggestions.length === 0) return;

  autocompleteState.visible = true;
  autocompleteState.selectedIndex = 0;
  autocompleteState.suggestions = suggestions;
  autocompleteState.textarea = textarea;

  // Position the autocomplete dropdown
  const rect = textarea.getBoundingClientRect();
  const lineHeight = parseInt(window.getComputedStyle(textarea).lineHeight);
  const cursorPos = getCursorPosition(textarea);

  autocompleteEl.style.display = "block";
  autocompleteEl.style.left = "40px"; // Match editor padding
  autocompleteEl.style.top = cursorPos.top + lineHeight + "px";

  // Render suggestions
  autocompleteEl.innerHTML = suggestions
    .map(
      (item, index) => `
        <div class="autocomplete-item ${index === 0 ? "selected" : ""}" data-index="${index}">
            <div class="autocomplete-label">${escapeHtml(item.label)}</div>
            <div class="autocomplete-description">${escapeHtml(item.description)}</div>
        </div>
    `,
    )
    .join("");

  // Add click handlers
  autocompleteEl.querySelectorAll(".autocomplete-item").forEach((item) => {
    item.addEventListener("click", () => {
      const index = parseInt(item.dataset.index);
      applyCompletion(suggestions[index]);
    });
  });
}

function hideAutocomplete() {
  const autocompleteEl = document.getElementById("markdown-autocomplete");
  if (autocompleteEl) {
    autocompleteEl.style.display = "none";
  }
  autocompleteState.visible = false;
  autocompleteState.suggestions = [];
}

function handleMarkdownKeydown(event) {
  if (!autocompleteState.visible) return;

  const autocompleteEl = document.getElementById("markdown-autocomplete");
  if (!autocompleteEl) return;

  switch (event.key) {
    case "ArrowDown":
      event.preventDefault();
      autocompleteState.selectedIndex =
        (autocompleteState.selectedIndex + 1) %
        autocompleteState.suggestions.length;
      updateAutocompleteSelection();
      break;

    case "ArrowUp":
      event.preventDefault();
      autocompleteState.selectedIndex =
        (autocompleteState.selectedIndex -
          1 +
          autocompleteState.suggestions.length) %
        autocompleteState.suggestions.length;
      updateAutocompleteSelection();
      break;

    case "Enter":
    case "Tab":
      if (autocompleteState.suggestions.length > 0) {
        event.preventDefault();
        applyCompletion(
          autocompleteState.suggestions[autocompleteState.selectedIndex],
        );
      }
      break;

    case "Escape":
      event.preventDefault();
      hideAutocomplete();
      break;
  }
}

function updateAutocompleteSelection() {
  const autocompleteEl = document.getElementById("markdown-autocomplete");
  if (!autocompleteEl) return;

  const items = autocompleteEl.querySelectorAll(".autocomplete-item");
  items.forEach((item, index) => {
    if (index === autocompleteState.selectedIndex) {
      item.classList.add("selected");
    } else {
      item.classList.remove("selected");
    }
  });
}

function applyCompletion(suggestion) {
  const textarea = autocompleteState.textarea;
  if (!textarea) return;

  const cursorPos = textarea.selectionStart;
  const textBeforeCursor = textarea.value.substring(0, cursorPos);
  const textAfterCursor = textarea.value.substring(cursorPos);

  // Find start of current line
  const lineStart = textBeforeCursor.lastIndexOf("\n") + 1;

  // Replace current line with suggestion
  const newText =
    textarea.value.substring(0, lineStart) + suggestion.text + textAfterCursor;
  textarea.value = newText;

  // Update cursor position
  let newCursorPos = lineStart + suggestion.text.length;

  // Special handling for certain completions
  if (suggestion.text.includes("]()")) {
    // Position cursor inside the brackets for links/images
    newCursorPos = lineStart + suggestion.text.indexOf("[") + 1;
  } else if (suggestion.text.includes("```\n\n```")) {
    // Position cursor inside code block
    newCursorPos = lineStart + suggestion.text.indexOf("\n") + 1;
  }

  textarea.setSelectionRange(newCursorPos, newCursorPos);
  textarea.focus();

  // Update note and hide autocomplete
  if (state.currentNote) {
    updateNoteDebounced(state.currentNote.id, "content", textarea.value);
  }
  hideAutocomplete();
}

function getCursorPosition(textarea) {
  const div = document.createElement("div");
  const style = window.getComputedStyle(textarea);

  div.style.position = "absolute";
  div.style.visibility = "hidden";
  div.style.whiteSpace = "pre-wrap";
  div.style.wordWrap = "break-word";
  div.style.font = style.font;
  div.style.padding = style.padding;
  div.style.width = style.width;
  div.style.lineHeight = style.lineHeight;

  const textBeforeCursor = textarea.value.substring(0, textarea.selectionStart);
  div.textContent = textBeforeCursor;

  const span = document.createElement("span");
  span.textContent = ".";
  div.appendChild(span);

  document.body.appendChild(div);

  const position = {
    top: span.offsetTop,
    left: span.offsetLeft,
  };

  document.body.removeChild(div);

  return position;
}

// ====== Long Image Share Feature ======

// ====== Image Export with Preview ======

async function shareAsImage() {
  if (!state.currentNote) return;

  const note = state.currentNote;

  // Show loading indicator
  const loadingModal = showLoadingModal(
    t("generating_image") || "Generating image...",
  );

  try {
    // Generate the preview HTML first
    const previewHTML = await generateImagePreviewHTML(note);

    // Hide loading modal
    closeLoadingModal(loadingModal);

    // Show preview modal
    showImagePreviewModal(previewHTML, note);

  } catch (error) {
    console.error("Failed to generate image preview:", error);
    closeLoadingModal(loadingModal);
    alert(t("generate_image_failed") || "Failed to generate image preview");
  }
}

// Generate HTML for image preview
async function generateImagePreviewHTML(note) {
  // Load export theme CSS content
  let cssContent = "";
  try {
    const cssResponse = await fetch("/static/css/export-theme.css");
    cssContent = await cssResponse.text();
  } catch (e) {
    console.error("Failed to load export-theme.css:", e);
    // Fallback to basic styling
    cssContent = `
      .export-container {
        background: var(--bg-primary);
        padding: 40px;
        border-radius: 16px;
        font-family: Georgia, "Songti SC", serif;
        max-width: 800px;
        margin: 0 auto;
      }
      .export-content {
        font-size: 18px;
        line-height: 1.8;
        color: var(--text-primary);
      }
      .export-content h1 {
        font-size: 32px;
        margin-bottom: 24px;
        border-bottom: 2px solid var(--border-light);
        padding-bottom: 16px;
      }
      .export-content p {
        margin: 20px 0;
      }
    `;
  }

  // Create HTML structure
  let html = `
    <div class="export-container" style="position: relative;">
      <style>${cssContent}</style>
  `;

  // Apply note color if available
  if (note.color) {
    html = html.replace('class="export-container"', `class="export-container" data-color="${note.color}"`);
  }

  // Create export header
  html += `
    <div class="export-header">
      <div>
        <h1 class="export-title">${escapeHtml(note.title || "Untitled Note")}</h1>
      </div>
      <div class="export-meta">
        <div class="export-date">
          <i data-feather="calendar"></i>
          ${new Date(note.updated_at).toLocaleDateString()}
        </div>
      </div>
    </div>
  `;

  // Create content wrapper
  html += '<div class="export-content">';

  // Render note content with enhanced markdown
  const contentHTML = await renderNoteForImage(note);
  html += contentHTML;

  html += '</div>';

  // Create export footer
  html += `
    <div class="export-footer">
      <div class="export-brand">
        <img src="/static/img/logo.png" alt="Smarticky" />
        <span>Smarticky Notes</span>
      </div>
      <div class="export-info">
        <span>${new Date().toLocaleString()}</span>
      </div>
    </div>
  `;

  html += '</div>';

  return html;
}

// Show image preview modal
function showImagePreviewModal(previewHTML, note) {
  // Create modal HTML
  const modalHTML = `
    <div class="modal" id="image-preview-modal">
      <div class="modal-content" style="max-width: 90vw; max-height: 90vh;">
        <div class="modal-header">
          <h2>${t("image_preview") || "Image Preview"}</h2>
          <button class="modal-close" onclick="closeImagePreviewModal()">×</button>
        </div>
        <div class="modal-body" style="padding: 20px; max-height: 70vh; overflow-y: auto;">
          <div id="image-preview-container" style="text-align: center;">
            ${previewHTML}
          </div>
        </div>
        <div class="modal-footer" style="display: flex; justify-content: center; gap: 10px; padding: 20px;">
          <button class="btn btn-primary" onclick="downloadImageFromPreview()">
            <i data-feather="download"></i> ${t("download_image") || "Download Image"}
          </button>
          <button class="btn btn-secondary" onclick="closeImagePreviewModal()">
            <i data-feather="x"></i> ${t("cancel") || "Cancel"}
          </button>
        </div>
      </div>
    </div>
  `;

  // Remove existing modal if any
  const existingModal = document.getElementById('image-preview-modal');
  if (existingModal) {
    existingModal.remove();
  }

  // Add modal to body
  document.body.insertAdjacentHTML('beforeend', modalHTML);

  // Show modal
  const modal = document.getElementById('image-preview-modal');
  modal.style.display = 'flex';

  // Store note data for download
  modal.dataset.noteId = note.id;
  modal.dataset.noteTitle = note.title || "Untitled";
  modal.dataset.noteContent = note.content || "";
  modal.dataset.noteUpdatedAt = note.updated_at;
  modal.dataset.noteColor = note.color || "";

  // Refresh feather icons
  if (typeof feather !== 'undefined') {
    feather.replace();
  }

  // Add click outside to close
  modal.addEventListener('click', function(e) {
    if (e.target === modal) {
      closeImagePreviewModal();
    }
  });
}

// Close image preview modal
function closeImagePreviewModal() {
  const modal = document.getElementById('image-preview-modal');
  if (modal) {
    modal.remove();
  }
}

// Handle click on preview mode to switch to edit mode
function handlePreviewClick(event) {
  // Only switch to source mode if not clicking on links or other interactive elements
  const target = event.target;
  if (target.tagName === 'A' || target.tagName === 'BUTTON' || target.closest('a, button')) {
    return; // Don't switch if clicking on links/buttons
  }

  // Switch to source mode for editing
  state.markdownViewMode = 'source';
  renderEditor();

  // Focus the textarea after switching
  setTimeout(() => {
    const textarea = document.getElementById('markdown-editor');
    if (textarea) {
      textarea.focus();
    }
  }, 100);
}

// Download image from preview
async function downloadImageFromPreview() {
  const modal = document.getElementById('image-preview-modal');
  if (!modal) return;

  const note = {
    id: modal.dataset.noteId,
    title: modal.dataset.noteTitle,
    content: modal.dataset.noteContent,
    updated_at: modal.dataset.noteUpdatedAt,
    color: modal.dataset.noteColor
  };

  // Show loading
  const loadingModal = showLoadingModal(
    t("generating_image") || "Generating image...",
  );

  try {
    // Create container for final rendering
    const container = document.createElement("div");
    container.className = "export-container loading";
    container.style.cssText = `
      position: absolute;
      left: -9999px;
      top: 0;
      width: 900px;
      font-family: Georgia, "Songti SC", "宋体", serif;
      box-sizing: border-box;
      overflow: visible;
    `;

    // Apply note color if available
    if (note.color) {
      container.setAttribute('data-color', note.color);
    }

    // Load CSS content
    let cssContent = "";
    try {
      const cssResponse = await fetch("/static/css/export-theme.css");
      cssContent = await cssResponse.text();
    } catch (e) {
      console.error("Failed to load export-theme.css:", e);
      cssContent = `
        .export-container {
          background: var(--bg-primary);
          padding: 40px;
          border-radius: 16px;
          font-family: Georgia, "Songti SC", serif;
          max-width: 800px;
          margin: 0 auto;
        }
        .export-content {
          font-size: 18px;
          line-height: 1.8;
          color: var(--text-primary);
        }
        .export-content h1 {
          font-size: 32px;
          margin-bottom: 24px;
          border-bottom: 2px solid var(--border-light);
          padding-bottom: 16px;
        }
        .export-content p {
          margin: 20px 0;
        }
      `;
    }

    // Create style element
    const styleEl = document.createElement("style");
    styleEl.textContent = cssContent;
    container.appendChild(styleEl);

    // Create the same HTML structure as preview
    const header = document.createElement("div");
    header.className = "export-header";
    header.innerHTML = `
      <div>
        <h1 class="export-title">${escapeHtml(note.title || "Untitled Note")}</h1>
      </div>
      <div class="export-meta">
        <div class="export-date">
          <i data-feather="calendar"></i>
          ${new Date(note.updated_at).toLocaleDateString()}
        </div>
      </div>
    `;
    container.appendChild(header);

    const contentWrapper = document.createElement("div");
    contentWrapper.className = "export-content";
    const contentHTML = await renderNoteForImage(note);
    contentWrapper.innerHTML = contentHTML;
    container.appendChild(contentWrapper);

    const footer = document.createElement("div");
    footer.className = "export-footer";
    footer.innerHTML = `
      <div class="export-brand">
        <img src="/static/img/logo.png" alt="Smarticky" />
        <span>Smarticky Notes</span>
      </div>
      <div class="export-info">
        <span>${new Date().toLocaleString()}</span>
      </div>
    `;
    container.appendChild(footer);

    document.body.appendChild(container);

    // Use snapdom to capture
    const result = await snapdom(container, {
      backgroundColor: container.style.background,
      scale: 2,
    });

    // Remove temporary container
    document.body.removeChild(container);

    // Download the image
    await result.download({
      format: "png",
      filename: `${note.title || "Untitled"}_${new Date().getTime()}`,
    });

    closeLoadingModal(loadingModal);
    closeImagePreviewModal();

    showNotification(t("image_downloaded") || "Image downloaded successfully", "success");
  } catch (error) {
    console.error("Failed to generate image:", error);
    closeLoadingModal(loadingModal);
    alert(t("generate_image_failed") || "Failed to generate image");
  }
}

async function renderNoteForImage(note) {
  // Title (h1 will be styled by smartisan.css)
  let contentHTML = `<h1>${escapeHtml(note.title || "Untitled")}</h1>`;

  // Content - render markdown using marked.js with smartisan theme
  try {
    const html = marked.parse(note.content || "");
    contentHTML += html;
  } catch (e) {
    console.error("Failed to render markdown for image:", e);
    contentHTML += `<p>${escapeHtml(note.content || "")}</p>`;
  }

  return contentHTML;
}

function showLoadingModal(message) {
  const modal = document.createElement("div");
  modal.className = "modal show";
  modal.style.zIndex = "10000";
  modal.innerHTML = `
        <div class="modal-content modal-sm" style="text-align: center; padding: 40px;">
            <i data-feather="loader" class="rotating" style="width: 48px; height: 48px; color: #5bc0de; margin-bottom: 20px;"></i>
            <p style="font-size: 16px; color: #666; margin: 0;">${message}</p>
        </div>
    `;
  document.body.appendChild(modal);
  feather.replace();
  return modal;
}

function closeLoadingModal(modal) {
  if (modal && modal.parentNode) {
    modal.parentNode.removeChild(modal);
  }
}

// About dialog functions
async function showAbout() {
  // Fetch version info
  try {
    const response = await fetch(`${API_BASE}/version`);
    if (response.ok) {
      const versionInfo = await response.json();

      // Update version info in the modal
      document.getElementById('app-version').textContent = versionInfo.version || 'Unknown';
      document.getElementById('app-build-time').textContent = versionInfo.build_time || 'Unknown';
      document.getElementById('app-commit').textContent = versionInfo.git_commit ? versionInfo.git_commit.substring(0, 8) : 'Unknown';
    } else {
      // Fallback if API call fails
      document.getElementById('app-version').textContent = 'Unknown';
      document.getElementById('app-build-time').textContent = 'Unknown';
      document.getElementById('app-commit').textContent = 'Unknown';
    }
  } catch (error) {
    console.error('Failed to fetch version info:', error);
    document.getElementById('app-version').textContent = 'Error';
    document.getElementById('app-build-time').textContent = 'Error';
    document.getElementById('app-commit').textContent = 'Error';
  }

  // Show the about modal
  showModal('about-modal');
}

// Close mobile editor and return to note list
function closeMobileEditor() {
  if (window.innerWidth <= 768) {
    document.body.classList.remove('mobile-editor-open');
  }
}

