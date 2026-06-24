import {
  LanguageDescription,
  LanguageSupport,
  StreamLanguage,
  type StringStream,
} from "@codemirror/language";
import { languages as defaultCodeLanguages } from "@codemirror/language-data";
import type { CodeMirrorFeatureConfig } from "@milkdown/crepe/feature/code-mirror";
import { normalizeDiagramType, prepareDiagramSource } from "./fences";
import {
  findMermaidDiagramVariant,
  mermaidDiagramVariants,
} from "./mermaidSyntax";
import {
  createWhiteboardPreviewMarkup,
  isWhiteboardFenceLanguage,
} from "../whiteboards";
import { renderDiagram } from "./registry";
import type {
  DiagramRenderRequest,
  DiagramRenderResult,
  DiagramTheme,
  DiagramType,
} from "./types";

type ApplyPreview = (value: null | string | HTMLElement) => void;
type RenderDiagram = (request: DiagramRenderRequest) => Promise<DiagramRenderResult>;

interface EditorDiagramCodeBlockConfigOptions {
  getTheme: () => DiagramTheme;
  render?: RenderDiagram;
}

const diagramLanguageSupport = new LanguageSupport(
  StreamLanguage.define({
    name: "diagram",
    token(stream: StringStream) {
      stream.skipToEnd();
      return null;
    },
  }),
);

function languageAliases(name: string, aliases: string[] = []): string[] {
  return Array.from(new Set([name, ...aliases]));
}

export const diagramCodeLanguages = [
  LanguageDescription.of({
    name: "mermaid",
    alias: languageAliases("mermaid", ["mmd"]),
    support: diagramLanguageSupport,
  }),
  ...mermaidDiagramVariants.map((variant) =>
    LanguageDescription.of({
      name: variant.name,
      alias: languageAliases(variant.name, variant.aliases),
      support: diagramLanguageSupport,
    }),
  ),
  LanguageDescription.of({
    name: "drawio",
    alias: languageAliases("drawio", ["draw.io"]),
    support: diagramLanguageSupport,
  }),
  LanguageDescription.of({
    name: "excalidraw",
    alias: languageAliases("excalidraw"),
    support: diagramLanguageSupport,
  }),
];

const diagramLanguageLabels: Record<DiagramType, string> = {
  mermaid: "Mermaid",
  drawio: "drawio",
};

function loadingMarkup(): string {
  return `<div class="diagram-loading">Rendering diagram...</div>`;
}

function errorMarkup(type: DiagramType, error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  return `<pre class="diagram-error">Failed to render ${type} diagram: ${escapeHtml(message)}</pre>`;
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&#39;");
}

export function createEditorDiagramCodeBlockConfig({
  getTheme,
  render = renderDiagram,
}: EditorDiagramCodeBlockConfigOptions): CodeMirrorFeatureConfig {
  return {
    languages: [...defaultCodeLanguages, ...diagramCodeLanguages],
    noResultText: "No matching language",
    previewLabel: "Diagram preview",
    previewLoading: loadingMarkup(),
    renderLanguage(language) {
      const mermaidVariant = findMermaidDiagramVariant(language);
      if (mermaidVariant) return mermaidVariant.label;
      if (isWhiteboardFenceLanguage(language)) return "Excalidraw";
      const type = normalizeDiagramType(language);
      return type ? diagramLanguageLabels[type] : language;
    },
    renderPreview(language: string, content: string, applyPreview: ApplyPreview) {
      if (isWhiteboardFenceLanguage(language)) {
        return createWhiteboardPreviewMarkup(content);
      }

      const type = normalizeDiagramType(language);
      const source = prepareDiagramSource(language, content);
      if (!type || !source) return null;

      applyPreview(loadingMarkup());
      void render({ type, source, theme: getTheme() })
        .then((result) => applyPreview(result.html))
        .catch((error: unknown) => applyPreview(errorMarkup(type, error)));

      return undefined;
    },
  };
}
