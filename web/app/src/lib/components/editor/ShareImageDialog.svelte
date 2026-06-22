<script lang="ts">
  import { toBlob, toPng } from "html-to-image";
  import { onMount, tick } from "svelte";
  import { waitForDiagramSettle } from "../../markdown/diagrams/wait";
  import type { DiagramRuntimeState, DiagramTheme } from "../../markdown/diagrams/types";
  import { renderMarkdown, stripMarkdown } from "../../markdown/render";
  import { notify } from "../../stores/dialogs";
  import { preferencesStore, t, type MessageKey } from "../../stores/preferences";
  import RenderedMarkdown from "../common/RenderedMarkdown.svelte";

  export let title = "";
  export let content = "";
  export let defaultSignature = "Smarticky";
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
    code: string;
    codeText: string;
    divider: string;
    quote: string;
    serif: boolean;
  }

  const themes: ShareTheme[] = [
    {
      id: "classic",
      labelKey: "shareClassic",
      background: "#fafaf8",
      surface: "#fffefa",
      text: "#1d1c19",
      muted: "#5c5c54",
      accent: "#e8531a",
      code: "#f4f3ee",
      codeText: "#3a3a34",
      divider: "#e2e0d8",
      quote: "#fef3ec",
      serif: true,
    },
    {
      id: "paper",
      labelKey: "sharePaper",
      background: "#f4f3ee",
      surface: "#fff8ec",
      text: "#1a1a18",
      muted: "#5c5c54",
      accent: "#c03b0d",
      code: "#eceae2",
      codeText: "#3a3a34",
      divider: "#c8c6bc",
      quote: "#fef3ec",
      serif: true,
    },
    {
      id: "night",
      labelKey: "shareNight",
      background: "#141412",
      surface: "#20201b",
      text: "#f0efe8",
      muted: "#a8a59e",
      accent: "#d45a20",
      code: "#2a2a26",
      codeText: "#d0ceca",
      divider: "#3c3c38",
      quote: "#3d1f0e",
      serif: false,
    },
  ];

  const ratios: {
    id: ShareRatio;
    labelKey: MessageKey;
    width: number;
    minHeight: number;
  }[] = [
    { id: "story", labelKey: "wideImage", width: 1080, minHeight: 1440 },
    { id: "square", labelKey: "squareImage", width: 1080, minHeight: 1080 },
  ];
  const exportScale = 2;
  const maxCanvasDimension = 32760;
  const settledDiagramState: DiagramRuntimeState = {
    pending: 0,
    total: 0,
    settled: true,
  };

  let themeID: ShareThemeID = "classic";
  let ratioID: ShareRatio = "story";
  let exportTarget: HTMLDivElement | null = null;
  let imageBusy = false;
  let exportDiagramState: DiagramRuntimeState = { ...settledDiagramState };
  let diagramTheme: DiagramTheme = "light";
  let signature = "";

  $: activeTheme = themes.find((theme) => theme.id === themeID) ?? themes[0];
  $: activeRatio = ratios.find((ratio) => ratio.id === ratioID) ?? ratios[0];
  $: plainTitle = title.trim() || t("untitled", $preferencesStore.language);
  $: bodyMarkdown =
    removeDuplicateLeadingHeading(content, plainTitle).trim() ||
    t("contentEmpty", $preferencesStore.language);
  $: renderedContent = renderMarkdown(bodyMarkdown);
  $: plainContent = stripMarkdown(content);
  $: wordCount = plainContent.replace(/\s/g, "").length;
  $: wordCountLabel = `${wordCount} ${t("wordUnit", $preferencesStore.language)}`;
  $: shareSignature = signature.trim() || "Smarticky";
  $: diagramTheme = themeID === "night" ? "dark" : "light";
  $: exportBlocked = imageBusy || !exportDiagramState.settled;
  $: themeStyle = [
    `--share-bg: ${activeTheme.background}`,
    `--share-surface: ${activeTheme.surface}`,
    `--share-text: ${activeTheme.text}`,
    `--share-muted: ${activeTheme.muted}`,
    `--share-accent: ${activeTheme.accent}`,
    `--share-code: ${activeTheme.code}`,
    `--share-code-text: ${activeTheme.codeText}`,
    `--share-divider: ${activeTheme.divider}`,
    `--share-quote: ${activeTheme.quote}`,
    `--share-width: ${activeRatio.width}px`,
    `--share-min-height: ${activeRatio.minHeight}px`,
  ].join("; ");

  function fileName(): string {
    const safeTitle = plainTitle.replace(/[\\/:*?"<>|]/g, "").slice(0, 24);
    return `${safeTitle || "smarticky"}-share.png`;
  }

  function normalizeHeading(value: string): string {
    return value
      .replace(/[*_`~\[\]()#]/g, "")
      .replace(/\s+/g, " ")
      .trim();
  }

  function removeDuplicateLeadingHeading(markdown: string, currentTitle: string): string {
    const match = markdown.match(/^\s*#{1,6}\s+(.+?)\s*#*\s*(?:\n+|$)/);
    if (!match) return markdown;

    if (normalizeHeading(match[1]) !== normalizeHeading(currentTitle)) {
      return markdown;
    }

    return markdown.slice(match[0].length).replace(/^\n+/, "");
  }

  function canvasScaleFor(width: number, height: number): number {
    const largestSide = Math.max(width, height);
    if (largestSide <= 0) return exportScale;
    return Math.min(exportScale, maxCanvasDimension / largestSide);
  }

  function setExportDiagramState(state: DiagramRuntimeState): void {
    exportDiagramState = state;
  }

  function delay(ms: number): Promise<void> {
    return new Promise((resolve) => window.setTimeout(resolve, ms));
  }

  async function waitForImages(root: HTMLElement): Promise<void> {
    const deadline = Date.now() + 5000;
    while (
      Date.now() < deadline &&
      root.querySelector("img[data-auth-image-loading='true']")
    ) {
      await delay(80);
    }

    const images = Array.from(root.querySelectorAll<HTMLImageElement>("img"));
    await Promise.all(
      images.map(async (image) => {
        if (image.complete) return;
        await new Promise<void>((resolve) => {
          const done = () => {
            image.removeEventListener("load", done);
            image.removeEventListener("error", done);
            resolve();
          };
          image.addEventListener("load", done, { once: true });
          image.addEventListener("error", done, { once: true });
        });
      }),
    );
  }

  async function exportOptions() {
    await tick();
    await waitForDiagramSettle(() => exportDiagramState);
    await tick();
    if (!exportTarget) throw new Error("Share image target unavailable");
    await waitForImages(exportTarget);

    const width = exportTarget.scrollWidth;
    const height = exportTarget.scrollHeight;
    return {
      backgroundColor: activeTheme.background,
      cacheBust: true,
      height,
      pixelRatio: canvasScaleFor(width, height),
      width,
    };
  }

  async function downloadImage(): Promise<void> {
    if (imageBusy) return;

    imageBusy = true;
    try {
      const options = await exportOptions();
      if (!exportTarget) throw new Error("Share image target unavailable");
      const href = await toPng(exportTarget, options);
      const link = document.createElement("a");
      link.download = fileName();
      link.href = href;
      link.click();
      notify(t("generatedImage", $preferencesStore.language), "success");
    } catch {
      notify(t("generateImageFailed", $preferencesStore.language), "error");
    } finally {
      imageBusy = false;
    }
  }

  async function copyImage(): Promise<void> {
    if (!navigator.clipboard || typeof ClipboardItem === "undefined") {
      notify(t("copyImageUnsupported", $preferencesStore.language), "info");
      return;
    }
    if (imageBusy) return;

    imageBusy = true;
    try {
      const options = await exportOptions();
      if (!exportTarget) throw new Error("Share image target unavailable");
      const blob = await toBlob(exportTarget, options);
      if (!blob) throw new Error("Canvas export failed");
      await navigator.clipboard.write([new ClipboardItem({ "image/png": blob })]);
      notify(t("copiedImage", $preferencesStore.language), "success");
    } catch {
      notify(t("copyImageFailed", $preferencesStore.language), "error");
    } finally {
      imageBusy = false;
    }
  }

  onMount(() => {
    signature = defaultSignature || "Smarticky";
  });
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
        style={themeStyle}
      >
        <div class="share-preview__canvas">
          <article class:serif={activeTheme.serif} class="share-preview__paper">
            <div class="share-preview__mark"></div>
            <h3>{plainTitle}</h3>
            <RenderedMarkdown
              className="share-preview__markdown"
              html={renderedContent}
              theme={diagramTheme}
            />
            <footer>
              <span>{shareSignature}</span>
              <span>{wordCountLabel}</span>
            </footer>
          </article>
        </div>
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

        <label class="share-control-group">
          <span>{t("shareSignatureTemporary", $preferencesStore.language)}</span>
          <input
            class="share-signature-input"
            bind:value={signature}
            type="text"
            maxlength="40"
            placeholder={t("shareSignaturePlaceholder", $preferencesStore.language)}
          />
        </label>

        <div class="share-dialog__actions">
          <button type="button" disabled={exportBlocked} on:click={copyImage}>
            {t("copyImage", $preferencesStore.language)}
          </button>
          <button class="primary" type="button" disabled={exportBlocked} on:click={downloadImage}>
            {t("downloadPng", $preferencesStore.language)}
          </button>
        </div>
      </aside>
    </div>
  </div>
</div>

<div class="share-export-stage" aria-hidden="true">
  <div
    bind:this={exportTarget}
    class:share-export-canvas--long={ratioID === "story"}
    class:share-export-canvas--square={ratioID === "square"}
    class="share-export-canvas"
    style={themeStyle}
  >
    <article class:serif={activeTheme.serif} class="share-preview__paper">
      <div class="share-preview__mark"></div>
      <h3>{plainTitle}</h3>
      <RenderedMarkdown
        className="share-preview__markdown"
        html={renderedContent}
        theme={diagramTheme}
        onDiagramState={setExportDiagramState}
      />
      <footer>
        <span>{shareSignature}</span>
        <span>{wordCountLabel}</span>
      </footer>
    </article>
  </div>
</div>
