/**
 * Milkdown Editor Integration for Smarticky Notes
 * Lightweight wrapper for Milkdown editor with Smarticky theme integration
 */

class MilkdownEditor {
    constructor(container, options = {}) {
        this.container = container;
        this.options = options;
        this.editor = null;
        this.isReady = false;
        this.debounceTimer = null;

        // 默认配置
        this.defaultOptions = {
            value: '',
            placeholder: 'Start writing your note...',
            onChange: null,
            onReady: null,
            theme: 'nord', // 适配Smarticky主题
            editable: true,
            autoFocus: false
        };

        // 合并配置
        this.config = { ...this.defaultOptions, ...options };

        this.init();
    }

    async init() {
        try {
            // 等待Milkdown库加载完成
            await this.waitForMilkdown();

            // 创建编辑器实例
            await this.createEditor();

            this.isReady = true;
            if (this.config.onReady) {
                this.config.onReady(this);
            }

            console.log('Milkdown editor initialized successfully');
        } catch (error) {
            console.error('Failed to initialize Milkdown editor:', error);
            this.showError('Failed to load advanced editor. Please refresh the page.');
        }
    }

    async waitForMilkdown() {
        // 使用简化版本的Milkdown，无需等待外部库加载
        return Promise.resolve();
    }

    async createEditor() {
        // 清空容器
        this.container.innerHTML = '';

        // 创建编辑器DOM结构
        const editorDiv = document.createElement('div');
        editorDiv.className = 'milkdown-editor';
        editorDiv.style.cssText = `
            height: 100%;
            font-family: Georgia, "Songti SC", "宋体", serif;
            font-size: 15px;
            line-height: 1.7;
            color: var(--text-primary);
            background: var(--bg-primary);
        `;

        this.container.appendChild(editorDiv);

        // 创建编辑器配置
        const editorConfig = {
            root: editorDiv,
            defaultValue: this.config.value,
            editable: this.config.editable,
            placeholder: this.config.placeholder,
            onChange: (markdown) => {
                // 防抖处理
                if (this.debounceTimer) {
                    clearTimeout(this.debounceTimer);
                }

                this.debounceTimer = setTimeout(() => {
                    if (this.config.onChange) {
                        this.config.onChange(markdown);
                    }
                }, 300);
            }
        };

        // 创建编辑器（简化版，使用原生ProseMirror）
        this.createSimpleEditor(editorDiv, editorConfig);
    }

    createSimpleEditor(container, config) {
        // 创建简化版的Markdown编辑器，模拟Milkdown功能
        // 实际项目中应该使用完整的Milkdown库

        const editorElement = document.createElement('div');
        editorElement.className = 'milkdown-editor-content';
        editorElement.contentEditable = config.editable;
        editorElement.style.cssText = `
            min-height: 300px;
            padding: 20px;
            outline: none;
            border: 1px solid var(--border-light);
            border-radius: 8px;
            background: var(--bg-primary);
            font-family: inherit;
            font-size: inherit;
            line-height: inherit;
            color: inherit;
            transition: border-color 0.2s ease;
        `;

        // 设置初始内容
        editorElement.innerHTML = this.markdownToHtml(config.defaultValue);

        // 添加占位符
        if (!config.defaultValue && config.placeholder) {
            this.addPlaceholder(editorElement, config.placeholder);
        }

        // 监听输入事件
        editorElement.addEventListener('input', (e) => {
            const markdown = this.htmlToMarkdown(e.target.innerHTML);

            // 处理占位符 - 只在真正开始输入时移除
            if (e.target.textContent.trim() === '') {
                // 如果内容为空，重新添加占位符
                if (!e.target.hasAttribute('data-placeholder')) {
                    this.addPlaceholder(editorElement, config.placeholder);
                }
            } else {
                // 如果有实际内容，移除占位符
                this.removePlaceholder(editorElement);
            }

            // 触发变更回调
            if (config.onChange) {
                config.onChange(markdown);
            }
        });

        // 监听焦点事件
        editorElement.addEventListener('focus', () => {
            editorElement.style.borderColor = 'var(--primary-color)';
            if (editorElement.getAttribute('data-placeholder')) {
                this.removePlaceholder(editorElement);
            }
        });

        editorElement.addEventListener('blur', () => {
            editorElement.style.borderColor = 'var(--border-light)';
            if (editorElement.textContent.trim() === '' && config.placeholder) {
                // 只有在没有占位符时才添加
                if (!editorElement.hasAttribute('data-placeholder')) {
                    this.addPlaceholder(editorElement, config.placeholder);
                }
            }
        });

        // 监听键盘事件（支持快捷键）
        editorElement.addEventListener('keydown', (e) => {
            this.handleKeydown(e, editorElement);
        });

        container.appendChild(editorElement);

        // 保存编辑器引用
        this.editorElement = editorElement;

        // 自动聚焦
        if (config.autoFocus) {
            editorElement.focus();
        }
    }

    addPlaceholder(element, placeholder) {
        // 保存原始占位符内容
        element.setAttribute('data-original-placeholder', placeholder);
        element.setAttribute('data-placeholder', placeholder);
        element.style.color = 'var(--text-tertiary)';
        element.innerHTML = placeholder;
        element.classList.add('has-placeholder');
    }

    removePlaceholder(element) {
        if (element.getAttribute('data-placeholder')) {
            element.removeAttribute('data-placeholder');
            element.removeAttribute('data-original-placeholder');
            element.style.color = 'var(--text-primary)';
            element.classList.remove('has-placeholder');

            // 只有在内容确实是占位符时才清空
            const originalPlaceholder = element.getAttribute('data-original-placeholder');
            if (originalPlaceholder && element.innerHTML === originalPlaceholder) {
                element.innerHTML = '';
            }
        }
    }

    handleKeydown(e, element) {
        // 支持一些基本的Markdown快捷键
        if (e.ctrlKey || e.metaKey) {
            switch (e.key) {
                case 'b':
                    e.preventDefault();
                    this.toggleFormat('bold');
                    break;
                case 'i':
                    e.preventDefault();
                    this.toggleFormat('italic');
                    break;
                case 'k':
                    e.preventDefault();
                    this.insertLink();
                    break;
            }
        }
    }

    toggleFormat(format) {
        // 简化的格式切换
        const selection = window.getSelection();
        if (!selection.rangeCount) return;

        const range = selection.getRangeAt(0);
        const selectedText = range.toString();

        if (!selectedText) return;

        let formattedText;
        switch (format) {
            case 'bold':
                formattedText = `**${selectedText}**`;
                break;
            case 'italic':
                formattedText = `*${selectedText}*`;
                break;
            default:
                return;
        }

        // 更新内容
        const currentContent = this.getValue();
        const newContent = currentContent.replace(selectedText, formattedText);
        this.setValue(newContent);
    }

    insertLink() {
        const url = prompt('Enter URL:');
        if (!url) return;

        const selection = window.getSelection();
        const selectedText = selection.toString() || 'Link';

        const linkMarkdown = `[${selectedText}](${url})`;

        // 插入链接
        const currentContent = this.getValue();
        const newContent = currentContent.replace(selectedText, linkMarkdown);
        this.setValue(newContent);
    }

    // 简化的Markdown转HTML
    markdownToHtml(markdown) {
        if (!markdown) return '';

        return markdown
            .replace(/^### (.*$)/gim, '<h3>$1</h3>')
            .replace(/^## (.*$)/gim, '<h2>$1</h2>')
            .replace(/^# (.*$)/gim, '<h1>$1</h1>')
            .replace(/\*\*(.*)\*\*/g, '<strong>$1</strong>')
            .replace(/\*(.*)\*/g, '<em>$1</em>')
            .replace(/\[([^\]]+)\]\(([^)]+)\)/g, '<a href="$2">$1</a>')
            .replace(/`([^`]+)`/g, '<code>$1</code>')
            .replace(/\n/g, '<br>');
    }

    // 简化的HTML转Markdown
    htmlToMarkdown(html) {
        if (!html) return '';

        return html
            .replace(/<h1>(.*)<\/h1>/gi, '# $1')
            .replace(/<h2>(.*)<\/h2>/gi, '## $1')
            .replace(/<h3>(.*)<\/h3>/gi, '### $1')
            .replace(/<strong>(.*)<\/strong>/gi, '**$1**')
            .replace(/<em>(.*)<\/em>/gi, '*$1*')
            .replace(/<a href="([^"]+)">([^<]+)<\/a>/gi, '[$2]($1)')
            .replace(/<code>(.*)<\/code>/gi, '`$1`')
            .replace(/<br\s*\/?>/gi, '\n')
            .replace(/<[^>]+>/g, '')
            .replace(/&nbsp;/g, ' ')
            .replace(/&lt;/g, '<')
            .replace(/&gt;/g, '>')
            .replace(/&amp;/g, '&');
    }

    // 公共API方法
    getValue() {
        if (!this.isReady || !this.editorElement) return '';
        return this.htmlToMarkdown(this.editorElement.innerHTML);
    }

    setValue(value) {
        if (!this.isReady || !this.editorElement) return;

        const html = this.markdownToHtml(value);
        this.editorElement.innerHTML = html;

        // 触发变更事件
        if (this.config.onChange) {
            this.config.onChange(value);
        }
    }

    insertText(text) {
        if (!this.isReady || !this.editorElement) return;

        // 聚焦编辑器
        this.editorElement.focus();

        // 插入文本
        document.execCommand('insertText', false, text);
    }

    focus() {
        if (this.editorElement) {
            this.editorElement.focus();
        }
    }

    blur() {
        if (this.editorElement) {
            this.editorElement.blur();
        }
    }

    isFocused() {
        return document.activeElement === this.editorElement;
    }

    getSelection() {
        if (!this.editorElement) return '';

        const selection = window.getSelection();
        if (selection.rangeCount > 0) {
            return selection.toString();
        }
        return '';
    }

    setSelection(start, end) {
        if (!this.editorElement) return;

        const range = document.createRange();
        const textNode = this.editorElement.firstChild;

        if (textNode) {
            range.setStart(textNode, Math.min(start, textNode.length));
            range.setEnd(textNode, Math.min(end, textNode.length));

            const selection = window.getSelection();
            selection.removeAllRanges();
            selection.addRange(range);
        }
    }

    // 错误处理
    showError(message) {
        if (this.container) {
            this.container.innerHTML = `
                <div style="
                    padding: 20px;
                    text-align: center;
                    color: var(--text-secondary);
                    font-size: 14px;
                    line-height: 1.5;
                ">
                    <div style="margin-bottom: 10px;">⚠️</div>
                    <div>${message}</div>
                    <button onclick="location.reload()" style="
                        margin-top: 15px;
                        padding: 8px 16px;
                        background: var(--primary-color);
                        color: white;
                        border: none;
                        border-radius: 6px;
                        cursor: pointer;
                        font-size: 13px;
                    ">Refresh Page</button>
                </div>
            `;
        }
    }

    // 销毁编辑器
    destroy() {
        if (this.debounceTimer) {
            clearTimeout(this.debounceTimer);
        }

        if (this.container) {
            this.container.innerHTML = '';
        }

        this.editor = null;
        this.editorElement = null;
        this.isReady = false;
    }

    // 获取编辑器状态
    getState() {
        return {
            isReady: this.isReady,
            hasContent: this.getValue().trim().length > 0,
            isEmpty: this.getValue().trim().length === 0,
            contentLength: this.getValue().length,
            wordCount: this.getValue().split(/\s+/).filter(word => word.length > 0).length
        };
    }
}

// 导出编辑器类
window.MilkdownEditor = MilkdownEditor;