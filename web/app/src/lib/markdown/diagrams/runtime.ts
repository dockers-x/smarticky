import { decodeDiagramSource } from "./placeholders";
import { renderDiagram } from "./registry";
import type { DiagramRuntimeState, DiagramTheme, DiagramType } from "./types";

export interface DiagramRuntimeOptions {
  theme: DiagramTheme;
  contentKey?: string;
  onStateChange?: (state: DiagramRuntimeState) => void;
}

function emitState(options: DiagramRuntimeOptions, pending: number, total: number): void {
  options.onStateChange?.({
    pending,
    total,
    settled: pending === 0,
  });
}

function escapeHtml(value: string): string {
  return value.replace(/[&<>"']/g, (char) => {
    const entities: Record<string, string> = {
      "&": "&amp;",
      "<": "&lt;",
      ">": "&gt;",
      '"': "&quot;",
      "'": "&#039;",
    };
    return entities[char] || char;
  });
}

function errorHtml(type: DiagramType, error: unknown): string {
  const message = error instanceof Error ? error.message : String(error);
  return `<pre class="diagram-error">Failed to render ${type} diagram: ${escapeHtml(message)}</pre>`;
}

async function renderRoot(
  node: HTMLElement,
  options: DiagramRuntimeOptions,
  runToken: { active: boolean },
): Promise<void> {
  const placeholders = Array.from(
    node.querySelectorAll<HTMLElement>("[data-diagram-placeholder='true']"),
  );
  const total = placeholders.length;
  let pending = total;
  emitState(options, pending, total);

  await Promise.all(
    placeholders.map(async (placeholder) => {
      const type = placeholder.dataset.diagramType as DiagramType | undefined;
      const encodedSource = placeholder.dataset.diagramSource || "";
      if (!type || (type !== "mermaid" && type !== "drawio")) {
        pending -= 1;
        emitState(options, pending, total);
        return;
      }

      try {
        const source = decodeDiagramSource(encodedSource);
        const result = await renderDiagram({
          type,
          source,
          theme: options.theme,
        });
        if (runToken.active) {
          placeholder.outerHTML = result.html;
        }
      } catch (error) {
        if (runToken.active) {
          placeholder.outerHTML = errorHtml(type, error);
        }
      } finally {
        pending -= 1;
        if (runToken.active) {
          emitState(options, pending, total);
        }
      }
    }),
  );

  if (runToken.active) {
    emitState(options, 0, total);
  }
}

export function diagramRuntime(node: HTMLElement, initialOptions: DiagramRuntimeOptions) {
  let options = initialOptions;
  let runToken = { active: true };
  let sourceHTML = node.innerHTML;

  function scheduleRender(restoreSource = false): void {
    runToken.active = false;
    runToken = { active: true };
    queueMicrotask(() => {
      if (restoreSource) {
        node.innerHTML = sourceHTML;
      }
      void renderRoot(node, options, runToken);
    });
  }

  scheduleRender();

  return {
    update(nextOptions: DiagramRuntimeOptions): void {
      const contentChanged = nextOptions.contentKey !== options.contentKey;
      const themeChanged = nextOptions.theme !== options.theme;
      options = nextOptions;
      if (contentChanged) {
        sourceHTML = node.innerHTML;
      }
      scheduleRender(contentChanged || themeChanged);
    },
    destroy(): void {
      runToken.active = false;
    },
  };
}
