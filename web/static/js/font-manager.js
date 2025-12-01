// ========================================
// Font Management
// ========================================

// Helper functions to access app.js functions
function getToken() {
    return localStorage.getItem('jwt_token');
}

function i18n(key) {
    // If t() function is available from app.js, use it
    if (typeof t === 'function') {
        return t(key);
    }
    // Otherwise return the key itself
    return key;
}

function getCurrentUserData() {
    // Try to get from app.js global variable
    if (typeof currentUser !== 'undefined' && currentUser) {
        return currentUser;
    }
    // Fallback to localStorage
    const userJson = localStorage.getItem('user');
    return userJson ? JSON.parse(userJson) : null;
}

// Global variable to store loaded fonts
let loadedFonts = [];
let selectedFontFile = null;

// Show font management modal
function showFontManagement() {
    showModal('font-modal');
    loadFonts();
}

// Load all fonts
async function loadFonts() {
    try {
        const response = await fetch(`${API_BASE}/fonts`, {
            headers: {
                'Authorization': `Bearer ${getToken()}`
            }
        });

        if (response.ok) {
            loadedFonts = await response.json();
            renderFontList();
        } else {
            showNotification(i18n('load_failed'), 'error');
        }
    } catch (error) {
        console.error('Failed to load fonts:', error);
        showNotification(i18n('load_failed'), 'error');
    }
}

// Render font list
function renderFontList() {
    const tbody = document.getElementById('font-list-body');
    if (!tbody) return;

    if (loadedFonts.length === 0) {
        tbody.innerHTML = `
            <tr>
                <td colspan="7" style="text-align: center; padding: 40px; color: #999;">
                    ${i18n('no_attachments')}
                </td>
            </tr>
        `;
        return;
    }

    tbody.innerHTML = loadedFonts.map(font => `
        <tr>
            <td>${escapeHtml(font.display_name)}</td>
            <td>
                <div style="font-family: '${font.name}'; font-size: 16px;" id="font-preview-${font.id}">
                    ${font.preview_text}
                </div>
            </td>
            <td>${font.format.toUpperCase()}</td>
            <td>${formatFileSize(font.file_size)}</td>
            <td>${escapeHtml(font.uploaded_by)}</td>
            <td>
                <span style="color: ${font.is_shared ? '#4caf50' : '#999'};">
                    ${font.is_shared ? '✓' : '✗'}
                </span>
            </td>
            <td style="text-align: center;">
                ${(() => {
                    const user = getCurrentUserData();
                    return user && (user.id === font.uploader_id || user.role === 'admin') ? `
                        <button class="btn-danger" onclick="deleteFont('${font.id}')" style="padding: 4px 8px; font-size: 12px;">
                            <i data-feather="trash-2"></i> ${i18n('delete')}
                        </button>
                    ` : '-';
                })()}
            </td>
        </tr>
    `).join('');

    // Reinitialize feather icons
    feather.replace();

    // Load each font dynamically
    loadedFonts.forEach(font => {
        loadFontFace(font);
    });
}

// Load font using FontFace API
async function loadFontFace(font) {
    try {
        // Fetch font with authentication
        const response = await fetch(font.download_url, {
            headers: {
                'Authorization': `Bearer ${getToken()}`
            }
        });

        if (!response.ok) {
            throw new Error(`HTTP error! status: ${response.status}`);
        }

        // Convert to blob and create object URL
        const blob = await response.blob();
        const fontUrl = URL.createObjectURL(blob);

        // Load font using FontFace API
        const fontFace = new FontFace(
            font.name,
            `url(${fontUrl})`
        );
        await fontFace.load();
        document.fonts.add(fontFace);
        console.log(`Font loaded: ${font.name}`);

        // Clean up object URL after a delay (font should be loaded by then)
        setTimeout(() => URL.revokeObjectURL(fontUrl), 5000);
    } catch (error) {
        console.error(`Failed to load font ${font.name}:`, error);
    }
}

// Handle font file selection
document.addEventListener('DOMContentLoaded', () => {
    const fontFileInput = document.getElementById('font-file-input');
    if (fontFileInput) {
        fontFileInput.addEventListener('change', async (e) => {
            const file = e.target.files[0];
            if (!file) {
                selectedFontFile = null;
                document.getElementById('font-preview-section').style.display = 'none';
                return;
            }

            // Validate file size (30MB)
            if (file.size > 30 * 1024 * 1024) {
                showNotification(i18n('font_file_too_large'), 'error');
                fontFileInput.value = '';
                return;
            }

            // Validate file extension
            const validExtensions = ['.ttf', '.otf', '.woff', '.woff2'];
            const fileName = file.name.toLowerCase();
            const isValid = validExtensions.some(ext => fileName.endsWith(ext));

            if (!isValid) {
                showNotification(i18n('font_format_invalid'), 'error');
                fontFileInput.value = '';
                return;
            }

            selectedFontFile = file;

            // Set display name from filename if empty
            const displayNameInput = document.getElementById('font-display-name');
            if (!displayNameInput.value) {
                const nameWithoutExt = file.name.replace(/\.(ttf|otf|woff|woff2)$/i, '');
                displayNameInput.value = nameWithoutExt;
            }

            // Show preview
            await previewFont(file);
        });
    }
});

// Preview font before uploading
async function previewFont(file) {
    const previewSection = document.getElementById('font-preview-section');
    const previewContainer = document.getElementById('font-preview-container');

    if (!previewSection || !previewContainer) return;

    try {
        // Create a temporary URL for the font file
        const fontUrl = URL.createObjectURL(file);
        const fontName = `preview-${Date.now()}`;

        // Load the font using FontFace API
        const fontFace = new FontFace(fontName, `url(${fontUrl})`);
        await fontFace.load();
        document.fonts.add(fontFace);

        // Apply the font to preview
        previewContainer.style.fontFamily = fontName;
        previewSection.style.display = 'block';

        // Clean up URL after a delay
        setTimeout(() => URL.revokeObjectURL(fontUrl), 5000);
    } catch (error) {
        console.error('Failed to preview font:', error);
        showNotification('Failed to preview font', 'error');
    }
}

// Upload font
async function uploadFont() {
    if (!selectedFontFile) {
        showNotification(i18n('font_file_required'), 'error');
        return;
    }

    const displayName = document.getElementById('font-display-name').value.trim();
    const isShared = document.getElementById('font-is-shared').checked;

    if (!displayName) {
        showNotification('Please enter a display name', 'error');
        return;
    }

    const formData = new FormData();
    formData.append('file', selectedFontFile);
    formData.append('display_name', displayName);
    formData.append('is_shared', isShared ? 'true' : 'false');

    try {
        const response = await fetch(`${API_BASE}/fonts`, {
            method: 'POST',
            headers: {
                'Authorization': `Bearer ${getToken()}`
            },
            body: formData
        });

        if (response.ok) {
            const newFont = await response.json();
            showNotification(i18n('font_upload_success'), 'success');

            // Reset form
            document.getElementById('font-file-input').value = '';
            document.getElementById('font-display-name').value = '';
            document.getElementById('font-is-shared').checked = true;
            document.getElementById('font-preview-section').style.display = 'none';
            selectedFontFile = null;

            // Reload font list
            await loadFonts();

            // Load the new font immediately
            await loadFontFace(newFont);
        } else {
            const error = await response.json();
            showNotification(error.error || i18n('font_upload_failed'), 'error');
        }
    } catch (error) {
        console.error('Failed to upload font:', error);
        showNotification(i18n('font_upload_failed'), 'error');
    }
}

// Delete font
async function deleteFont(fontId) {
    if (!confirm(i18n('font_delete_confirm'))) {
        return;
    }

    try {
        const response = await fetch(`${API_BASE}/fonts/${fontId}`, {
            method: 'DELETE',
            headers: {
                'Authorization': `Bearer ${getToken()}`
            }
        });

        if (response.ok) {
            showNotification(i18n('font_delete_success'), 'success');
            await loadFonts();
        } else {
            const error = await response.json();
            showNotification(error.error || i18n('font_delete_failed'), 'error');
        }
    } catch (error) {
        console.error('Failed to delete font:', error);
        showNotification(i18n('font_delete_failed'), 'error');
    }
}

// Format file size
function formatFileSize(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
}

// Load all custom fonts on page load
async function loadAllCustomFonts() {
    try {
        const response = await fetch(`${API_BASE}/fonts`, {
            headers: {
                'Authorization': `Bearer ${getToken()}`
            }
        });

        if (response.ok) {
            const fonts = await response.json();
            for (const font of fonts) {
                await loadFontFace(font);
            }
            console.log(`Loaded ${fonts.length} custom fonts`);
        }
    } catch (error) {
        console.error('Failed to load custom fonts:', error);
    }
}

// Initialize fonts on page load
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', loadAllCustomFonts);
} else {
    loadAllCustomFonts();
}

// ========================================
// Font Selector for Editor
// ========================================

// Get selected font from localStorage
function getSelectedFont() {
    return localStorage.getItem('selected-font') || 'default';
}

// Set selected font to localStorage and apply it
function setSelectedFont(fontName) {
    localStorage.setItem('selected-font', fontName);
    applyFontToEditor(fontName);
}

// Apply font to editor
function applyFontToEditor(fontName) {
    const editorDiv = document.querySelector('.milkdown');
    const markdownEditor = document.getElementById('markdown-editor');
    const previewContent = document.getElementById('markdown-preview-content');

    const fontFamilyValue = (fontName === 'default' || !fontName)
        ? ''
        : `'${fontName}', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif`;

    // Apply to Milkdown editor (if exists)
    if (editorDiv) {
        editorDiv.style.fontFamily = fontFamilyValue;
    }

    // Apply to traditional markdown editor
    if (markdownEditor) {
        markdownEditor.style.fontFamily = fontFamilyValue;
    }

    // Apply to preview content
    if (previewContent) {
        previewContent.style.fontFamily = fontFamilyValue;
    }
}

// Show font selector dropdown
async function showFontSelector(buttonElement) {
    // Remove existing dropdown if any
    const existingDropdown = document.querySelector('.font-selector-dropdown');
    if (existingDropdown) {
        existingDropdown.remove();
        return;
    }

    // Reload fonts to ensure we have the latest list
    await loadFonts();

    // Create dropdown
    const dropdown = document.createElement('div');
    dropdown.className = 'font-selector-dropdown';
    dropdown.style.cssText = `
        position: absolute;
        background: white;
        border: 1px solid #ddd;
        border-radius: 8px;
        box-shadow: 0 4px 12px rgba(0,0,0,0.15);
        max-height: 400px;
        overflow-y: auto;
        z-index: 1000;
        width: 200px;
    `;

    const currentFont = getSelectedFont();

    // System fonts
    const systemFonts = [
        { name: 'default', display: i18n('system_font') || 'System Font' },
        { name: 'Arial', display: 'Arial' },
        { name: 'Helvetica', display: 'Helvetica' },
        { name: 'Times New Roman', display: 'Times New Roman' },
        { name: 'Georgia', display: 'Georgia' },
        { name: 'Courier New', display: 'Courier New' },
        { name: 'Verdana', display: 'Verdana' }
    ];

    let html = '<div style="padding: 8px; border-bottom: 1px solid #eee; font-weight: bold; font-size: 12px; color: #666;">' +
               (i18n('select_font') || 'Select Font') + '</div>';

    // Add system fonts
    systemFonts.forEach(font => {
        const isSelected = currentFont === font.name;
        html += `
            <div class="font-option ${isSelected ? 'selected' : ''}"
                 onclick="selectFontOption('${font.name}')"
                 style="padding: 10px 15px; cursor: pointer; font-family: '${font.name}'; font-size: 14px;
                        ${isSelected ? 'background: #f0f0f0;' : ''}"
                 onmouseover="this.style.background='#f8f8f8'"
                 onmouseout="this.style.background='${isSelected ? '#f0f0f0' : ''}'">
                ${font.display} ${isSelected ? '✓' : ''}
            </div>
        `;
    });

    // Add custom fonts
    if (loadedFonts && loadedFonts.length > 0) {
        html += '<div style="padding: 8px; border-top: 1px solid #eee; border-bottom: 1px solid #eee; font-weight: bold; font-size: 12px; color: #666; margin-top: 4px;">Custom Fonts</div>';

        loadedFonts.forEach(font => {
            const isSelected = currentFont === font.name;
            html += `
                <div class="font-option ${isSelected ? 'selected' : ''}"
                     onclick="selectFontOption('${font.name}')"
                     style="padding: 10px 15px; cursor: pointer; font-family: '${font.name}'; font-size: 14px;
                            display: flex; align-items: center; justify-content: space-between; gap: 10px;
                            ${isSelected ? 'background: #f0f0f0;' : ''}"
                     onmouseover="this.style.background='#f8f8f8'"
                     onmouseout="this.style.background='${isSelected ? '#f0f0f0' : ''}'">
                    <span style="flex: 1; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
                        ${font.display_name}
                    </span>
                    <span style="display: flex; align-items: center; gap: 8px; flex-shrink: 0;">
                        <small style="color: #999; font-size: 11px; font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;">
                            by ${font.uploaded_by}
                        </small>
                        ${isSelected ? '<span style="color: #5bc0de;">✓</span>' : ''}
                    </span>
                </div>
            `;
        });
    }

    dropdown.innerHTML = html;

    // Position dropdown below button
    const rect = buttonElement.getBoundingClientRect();
    dropdown.style.top = (rect.bottom + window.scrollY) + 'px';
    dropdown.style.left = rect.left + 'px';

    document.body.appendChild(dropdown);

    // Close dropdown when clicking outside
    setTimeout(() => {
        document.addEventListener('click', function closeDropdown(e) {
            if (!dropdown.contains(e.target) && e.target !== buttonElement) {
                dropdown.remove();
                document.removeEventListener('click', closeDropdown);
            }
        });
    }, 0);
}

// Select font option
function selectFontOption(fontName) {
    setSelectedFont(fontName);

    // Remove dropdown
    const dropdown = document.querySelector('.font-selector-dropdown');
    if (dropdown) {
        dropdown.remove();
    }

    showNotification('Font applied', 'success');
}

// Apply saved font on editor load
window.addEventListener('editorReady', function() {
    const savedFont = getSelectedFont();
    if (savedFont && savedFont !== 'default') {
        applyFontToEditor(savedFont);
    }
});

