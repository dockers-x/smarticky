<script lang="ts">
  import { notify } from "../../stores/dialogs";
  import { preferencesStore, t, type MessageKey } from "../../stores/preferences";

  export let title = "";
  export let content = "";
  export let onClose: () => void = () => {};

  type ShareThemeID = "classic" | "paper" | "night";
  type ShareRatio = "story" | "square";

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

  let themeID: ShareThemeID = "classic";
  let ratioID: ShareRatio = "story";

  $: activeTheme = themes.find((theme) => theme.id === themeID) ?? themes[0];
  $: activeRatio = ratios.find((ratio) => ratio.id === ratioID) ?? ratios[0];
  $: plainTitle = stripMarkdown(title).trim() || t("untitled", $preferencesStore.language);
  $: plainContent =
    stripMarkdown(content).trim() || t("contentEmpty", $preferencesStore.language);
  $: paragraphs = plainContent.split(/\n{2,}/).slice(0, 4);
  $: wordCount = plainContent === t("contentEmpty", $preferencesStore.language) ? 0 : plainContent.length;
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

  async function renderCanvas(): Promise<HTMLCanvasElement> {
    const canvas = document.createElement("canvas");
    const scale = 2;
    const width = activeRatio.width;
    const height = activeRatio.height;
    canvas.width = width * scale;
    canvas.height = height * scale;

    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("Canvas unavailable");
    ctx.scale(scale, scale);
    ctx.fillStyle = activeTheme.background;
    ctx.fillRect(0, 0, width, height);

    const margin = ratioID === "story" ? 108 : 92;
    const surfaceWidth = width - margin * 2;
    const surfaceHeight = height - margin * 2;
    ctx.fillStyle = activeTheme.surface;
    ctx.fillRect(margin, margin, surfaceWidth, surfaceHeight);

    const contentX = margin + 74;
    const contentWidth = surfaceWidth - 148;
    let y = margin + 112;
    const titleFont =
      "600 52px -apple-system, BlinkMacSystemFont, 'PingFang SC', sans-serif";
    const bodyFont = activeTheme.serif
      ? "34px 'Songti SC', 'Noto Serif CJK SC', 'Source Han Serif SC', serif"
      : "32px -apple-system, BlinkMacSystemFont, 'PingFang SC', sans-serif";

    ctx.fillStyle = activeTheme.accent;
    ctx.fillRect(contentX, y - 42, 56, 5);

    ctx.fillStyle = activeTheme.text;
    ctx.font = titleFont;
    for (const line of wrapText(ctx, plainTitle, contentWidth).slice(0, 3)) {
      ctx.fillText(line, contentX, y);
      y += 68;
    }
    y += 34;

    ctx.font = bodyFont;
    const lineHeight = activeTheme.serif ? 62 : 58;
    const maxY = height - margin - 128;
    for (const paragraph of plainContent.split(/\n+/)) {
      for (const line of wrapText(ctx, paragraph, contentWidth)) {
        if (y > maxY) break;
        ctx.fillText(line, contentX, y);
        y += lineHeight;
      }
      y += 28;
      if (y > maxY) break;
    }

    ctx.fillStyle = activeTheme.muted;
    ctx.font = "24px -apple-system, BlinkMacSystemFont, 'PingFang SC', sans-serif";
    ctx.fillText("Smarticky", contentX, height - margin - 54);
    ctx.textAlign = "right";
    ctx.fillText(wordCountLabel, width - margin - 74, height - margin - 54);

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
        class:share-preview--square={ratioID === "square"}
        class="share-preview"
        style={`--share-bg: ${activeTheme.background}; --share-surface: ${activeTheme.surface}; --share-text: ${activeTheme.text}; --share-muted: ${activeTheme.muted}; --share-accent: ${activeTheme.accent};`}
      >
        <article class:serif={activeTheme.serif} class="share-preview__paper">
          <div class="share-preview__mark"></div>
          <h3>{plainTitle}</h3>
          {#each paragraphs as paragraph}
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
