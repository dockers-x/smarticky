<script lang="ts">
  import { notify } from "../../stores/dialogs";
  import { preferencesStore, t, type MessageKey } from "../../stores/preferences";

  export let title = "";
  export let content = "";
  export let onClose: () => void = () => {};

  type ShareThemeID = "classic" | "paper" | "night";
  type ShareRatio = "story" | "square";

  interface TextBlock {
    lines: string[];
  }

  interface ShareLayout {
    width: number;
    height: number;
    margin: number;
    surfaceWidth: number;
    surfaceHeight: number;
    contentX: number;
    contentWidth: number;
    contentStartY: number;
    footerY: number;
    titleLines: string[];
    bodyBlocks: TextBlock[];
    titleFont: string;
    bodyFont: string;
    lineHeight: number;
    titleLineHeight: number;
    titleGap: number;
    paragraphGap: number;
  }

  interface ShareTheme {
    id: ShareThemeID;
    labelKey: MessageKey;
    background: string;
    surface: string;
    text: string;
    muted: string;
    accent: string;
    serif: boolean;
  }

  const themes: ShareTheme[] = [
    {
      id: "classic",
      labelKey: "shareClassic",
      background: "#f7f6f1",
      surface: "#ffffff",
      text: "#1d1c19",
      muted: "#8b877d",
      accent: "#e8450a",
      serif: true,
    },
    {
      id: "paper",
      labelKey: "sharePaper",
      background: "#eee6d7",
      surface: "#fffaf0",
      text: "#221f19",
      muted: "#8a7862",
      accent: "#bd5c18",
      serif: true,
    },
    {
      id: "night",
      labelKey: "shareNight",
      background: "#171713",
      surface: "#20201b",
      text: "#f5f1e7",
      muted: "#9b968b",
      accent: "#f4831f",
      serif: false,
    },
  ];

  const ratios: { id: ShareRatio; labelKey: MessageKey; width: number; height: number }[] = [
    { id: "story", labelKey: "wideImage", width: 1080, height: 1440 },
    { id: "square", labelKey: "squareImage", width: 1080, height: 1080 },
  ];
  const exportScale = 2;
  const maxCanvasDimension = 32760;

  let themeID: ShareThemeID = "classic";
  let ratioID: ShareRatio = "story";

  $: activeTheme = themes.find((theme) => theme.id === themeID) ?? themes[0];
  $: activeRatio = ratios.find((ratio) => ratio.id === ratioID) ?? ratios[0];
  $: plainTitle =
    stripMarkdown(title).trim() || t("untitled", $preferencesStore.language);
  $: plainContent =
    stripMarkdown(content).trim() || t("contentEmpty", $preferencesStore.language);
  $: contentParagraphs = plainContent
    .split(/\n{2,}/)
    .filter((paragraph) => paragraph.trim());
  $: previewParagraphs =
    ratioID === "story" ? contentParagraphs : contentParagraphs.slice(0, 4);
  $: wordCount =
    plainContent === t("contentEmpty", $preferencesStore.language)
      ? 0
      : plainContent.length;
  $: wordCountLabel = `${wordCount} ${t("wordUnit", $preferencesStore.language)}`;

  function stripMarkdown(value: string): string {
    return value
      .replace(/!\[[^\]]*]\([^)]*\)/g, "")
      .replace(/\[([^\]]+)]\([^)]*\)/g, "$1")
      .replace(/^[#>\s-]*\s*/gm, "")
      .replace(/[*_`~]/g, "")
      .replace(/\n{3,}/g, "\n\n");
  }

  function fileName(): string {
    const safeTitle = plainTitle.replace(/[\\/:*?"<>|]/g, "").slice(0, 24);
    return `${safeTitle || "smarticky"}-share.png`;
  }

  function canvasScaleFor(width: number, height: number): number {
    const largestSide = Math.max(width, height);
    if (largestSide <= 0) return exportScale;
    return Math.min(exportScale, maxCanvasDimension / largestSide);
  }

  function wrapText(
    ctx: CanvasRenderingContext2D,
    text: string,
    maxWidth: number,
  ): string[] {
    const lines: string[] = [];
    let line = "";

    for (const char of text) {
      const next = `${line}${char}`;
      if (ctx.measureText(next).width > maxWidth && line) {
        lines.push(line);
        line = char;
      } else {
        line = next;
      }
    }

    if (line) lines.push(line);
    return lines;
  }

  function createLayout(ctx: CanvasRenderingContext2D): ShareLayout {
    const isLongImage = ratioID === "story";
    const width = activeRatio.width;
    const minHeight = activeRatio.height;
    const margin = isLongImage ? 108 : 92;
    const surfaceWidth = width - margin * 2;
    const contentX = margin + 74;
    const contentWidth = surfaceWidth - 148;
    const contentStartY = margin + 112;
    const titleFont =
      "600 52px -apple-system, BlinkMacSystemFont, 'PingFang SC', sans-serif";
    const bodyFont = activeTheme.serif
      ? "34px 'Songti SC', 'Noto Serif CJK SC', 'Source Han Serif SC', serif"
      : "32px -apple-system, BlinkMacSystemFont, 'PingFang SC', sans-serif";
    const titleLineHeight = 68;
    const titleGap = 34;
    const lineHeight = activeTheme.serif ? 62 : 58;
    const paragraphGap = 28;

    ctx.font = titleFont;
    const titleLines = wrapText(ctx, plainTitle, contentWidth).slice(
      0,
      isLongImage ? undefined : 3,
    );

    ctx.font = bodyFont;
    const bodyBlocks = plainContent
      .split(/\n+/)
      .map((paragraph) => ({ lines: wrapText(ctx, paragraph, contentWidth) }))
      .filter((block) => block.lines.length > 0);

    let y = contentStartY + titleLines.length * titleLineHeight + titleGap;
    const fixedMaxY = minHeight - margin - 128;

    for (const block of bodyBlocks) {
      for (const _line of block.lines) {
        if (!isLongImage && y > fixedMaxY) break;
        y += lineHeight;
      }
      y += paragraphGap;
      if (!isLongImage && y > fixedMaxY) break;
    }

    const footerY = isLongImage
      ? Math.max(y + 40, minHeight - margin - 54)
      : minHeight - margin - 54;
    const height = isLongImage ? Math.ceil(footerY + margin + 54) : minHeight;

    return {
      width,
      height,
      margin,
      surfaceWidth,
      surfaceHeight: height - margin * 2,
      contentX,
      contentWidth,
      contentStartY,
      footerY,
      titleLines,
      bodyBlocks,
      titleFont,
      bodyFont,
      lineHeight,
      titleLineHeight,
      titleGap,
      paragraphGap,
    };
  }

  async function renderCanvas(): Promise<HTMLCanvasElement> {
    const measureCanvas = document.createElement("canvas");
    const measureCtx = measureCanvas.getContext("2d");
    if (!measureCtx) throw new Error("Canvas unavailable");

    const layout = createLayout(measureCtx);
    const canvas = document.createElement("canvas");
    const scale = canvasScaleFor(layout.width, layout.height);
    canvas.width = Math.max(
      1,
      Math.min(maxCanvasDimension, Math.ceil(layout.width * scale)),
    );
    canvas.height = Math.max(
      1,
      Math.min(maxCanvasDimension, Math.ceil(layout.height * scale)),
    );

    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("Canvas unavailable");
    ctx.scale(scale, scale);
    ctx.fillStyle = activeTheme.background;
    ctx.fillRect(0, 0, layout.width, layout.height);

    ctx.fillStyle = activeTheme.surface;
    ctx.fillRect(
      layout.margin,
      layout.margin,
      layout.surfaceWidth,
      layout.surfaceHeight,
    );

    ctx.fillStyle = activeTheme.accent;
    ctx.fillRect(layout.contentX, layout.contentStartY - 42, 56, 5);

    ctx.fillStyle = activeTheme.text;
    ctx.font = layout.titleFont;
    let y = layout.contentStartY;
    for (const line of layout.titleLines) {
      ctx.fillText(line, layout.contentX, y);
      y += layout.titleLineHeight;
    }
    y += layout.titleGap;

    ctx.font = layout.bodyFont;
    const maxY =
      ratioID === "story" ? Infinity : layout.height - layout.margin - 128;
    for (const block of layout.bodyBlocks) {
      for (const line of block.lines) {
        if (y > maxY) break;
        ctx.fillText(line, layout.contentX, y);
        y += layout.lineHeight;
      }
      y += layout.paragraphGap;
      if (y > maxY) break;
    }

    ctx.fillStyle = activeTheme.muted;
    ctx.font = "24px -apple-system, BlinkMacSystemFont, 'PingFang SC', sans-serif";
    ctx.textAlign = "left";
    ctx.fillText("Smarticky", layout.contentX, layout.footerY);
    ctx.textAlign = "right";
    ctx.fillText(
      wordCountLabel,
      layout.width - layout.margin - 74,
      layout.footerY,
    );

    return canvas;
  }

  async function downloadImage(): Promise<void> {
    try {
      const canvas = await renderCanvas();
      const link = document.createElement("a");
      link.download = fileName();
      link.href = canvas.toDataURL("image/png");
      link.click();
      notify(t("generatedImage", $preferencesStore.language), "success");
    } catch {
      notify(t("generateImageFailed", $preferencesStore.language), "error");
    }
  }

  async function copyImage(): Promise<void> {
    if (!navigator.clipboard || typeof ClipboardItem === "undefined") {
      notify(t("copyImageUnsupported", $preferencesStore.language), "info");
      return;
    }

    try {
      const canvas = await renderCanvas();
      const blob = await new Promise<Blob | null>((resolve) =>
        canvas.toBlob(resolve, "image/png"),
      );
      if (!blob) throw new Error("Canvas export failed");
      await navigator.clipboard.write([new ClipboardItem({ "image/png": blob })]);
      notify(t("copiedImage", $preferencesStore.language), "success");
    } catch {
      notify(t("copyImageFailed", $preferencesStore.language), "error");
    }
  }
</script>

<div
  class="share-dialog-backdrop"
  role="presentation"
  on:click={(event) => {
    if (event.currentTarget === event.target) onClose();
  }}
>
  <div
    class="share-dialog"
    role="dialog"
    aria-modal="true"
    aria-label={t("shareDialogLabel", $preferencesStore.language)}
  >
    <header class="share-dialog__header">
      <div>
        <h2>{t("generateImage", $preferencesStore.language)}</h2>
        <p>{wordCountLabel} · {t("shareSmartisanSubtitle", $preferencesStore.language)}</p>
      </div>
      <button type="button" aria-label={t("closeShareImage", $preferencesStore.language)} on:click={onClose}>×</button>
    </header>

    <div class="share-dialog__body">
      <div
        class:share-preview--long={ratioID === "story"}
        class:share-preview--square={ratioID === "square"}
        class="share-preview"
        style={`--share-bg: ${activeTheme.background}; --share-surface: ${activeTheme.surface}; --share-text: ${activeTheme.text}; --share-muted: ${activeTheme.muted}; --share-accent: ${activeTheme.accent};`}
      >
        <article class:serif={activeTheme.serif} class="share-preview__paper">
          <div class="share-preview__mark"></div>
          <h3>{plainTitle}</h3>
          {#each previewParagraphs as paragraph}
            <p>{paragraph}</p>
          {/each}
          <footer>
            <span>Smarticky</span>
            <span>{wordCountLabel}</span>
          </footer>
        </article>
      </div>

      <aside class="share-dialog__controls">
        <div class="share-control-group" aria-label={t("format", $preferencesStore.language)}>
          <span>{t("format", $preferencesStore.language)}</span>
          <div class="share-segmented">
            {#each ratios as ratio}
              <button
                class:active={ratioID === ratio.id}
                type="button"
                aria-pressed={ratioID === ratio.id}
                on:click={() => (ratioID = ratio.id)}
              >
                {t(ratio.labelKey, $preferencesStore.language)}
              </button>
            {/each}
          </div>
        </div>

        <div class="share-control-group" aria-label={t("theme", $preferencesStore.language)}>
          <span>{t("theme", $preferencesStore.language)}</span>
          <div class="share-theme-list">
            {#each themes as theme}
              <button
                class:active={themeID === theme.id}
                type="button"
                aria-pressed={themeID === theme.id}
                on:click={() => (themeID = theme.id)}
              >
                <span
                  class="share-theme-swatch"
                  style={`background: ${theme.surface}; border-color: ${theme.accent};`}
                ></span>
                {t(theme.labelKey, $preferencesStore.language)}
              </button>
            {/each}
          </div>
        </div>

        <div class="share-dialog__actions">
          <button type="button" on:click={copyImage}>
            {t("copyImage", $preferencesStore.language)}
          </button>
          <button class="primary" type="button" on:click={downloadImage}>
            {t("downloadPng", $preferencesStore.language)}
          </button>
        </div>
      </aside>
    </div>
  </div>
</div>
