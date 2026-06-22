import {
  LanguageDescription,
  LanguageSupport,
  StreamLanguage,
  type StringStream,
} from "@codemirror/language";
import type { CodeMirrorFeatureConfig } from "@milkdown/crepe/feature/code-mirror";
import { normalizeDiagramType } from "./fences";
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

export const diagramCodeLanguages = [
  LanguageDescription.of({
    name: "mermaid",
    alias: ["mmd"],
    support: diagramLanguageSupport,
  }),
  LanguageDescription.of({
    name: "drawio",
    alias: ["draw.io"],
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
    languages: diagramCodeLanguages,
    noResultText: "No matching language",
    previewLabel: "Diagram preview",
    previewLoading: loadingMarkup(),
    renderLanguage(language) {
      const type = normalizeDiagramType(language);
      return type ? diagramLanguageLabels[type] : language;
    },
    renderPreview(language: string, content: string, applyPreview: ApplyPreview) {
      const type = normalizeDiagramType(language);
      const source = content.trim();
      if (!type || !source) return null;

      applyPreview(loadingMarkup());
      void render({ type, source, theme: getTheme() })
        .then((result) => applyPreview(result.html))
        .catch((error: unknown) => applyPreview(errorMarkup(type, error)));

      return undefined;
    },
  };
}
