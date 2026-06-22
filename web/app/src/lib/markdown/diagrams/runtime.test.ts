import { afterEach, describe, expect, it } from "vitest";
import { createDiagramPlaceholder } from "./placeholders";
import { setDiagramRendererForTest } from "./registry";
import { diagramRuntime } from "./runtime";
import type { DiagramRuntimeState } from "./types";

describe("diagramRuntime", () => {
  afterEach(() => {
    setDiagramRendererForTest("mermaid", null);
    setDiagramRendererForTest("drawio", null);
  });

  it("replaces placeholders with rendered HTML and reports settle state", async () => {
    const root = document.createElement("div");
    root.innerHTML = createDiagramPlaceholder("mermaid", "flowchart TD\nA --> B");
    const states: DiagramRuntimeState[] = [];

    setDiagramRendererForTest("mermaid", {
      type: "mermaid",
      async render() {
        return { html: "<svg data-rendered='mermaid'></svg>" };
      },
    });

    diagramRuntime(root, {
      theme: "light",
      onStateChange: (state) => states.push(state),
    });

    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(root.querySelector("[data-rendered='mermaid']")).toBeTruthy();
    expect(root.querySelector("[data-diagram-placeholder='true']")).toBeNull();
    expect(states.at(-1)).toEqual({ pending: 0, total: 1, settled: true });
  });

  it("renders an inline error when rendering fails", async () => {
    const root = document.createElement("div");
    root.innerHTML = createDiagramPlaceholder("drawio", "<mxfile></mxfile>");

    setDiagramRendererForTest("drawio", {
      type: "drawio",
      async render() {
        throw new Error("invalid drawio xml");
      },
    });

    diagramRuntime(root, { theme: "light" });

    await new Promise((resolve) => setTimeout(resolve, 0));

    const error = root.querySelector(".diagram-error");
    expect(error?.textContent).toContain("drawio");
    expect(error?.textContent).toContain("invalid drawio xml");
  });

  it("rerenders placeholders when the action options change", async () => {
    const root = document.createElement("div");
    root.innerHTML = createDiagramPlaceholder("mermaid", "flowchart TD\nA --> B");
    let renderCount = 0;

    setDiagramRendererForTest("mermaid", {
      type: "mermaid",
      async render() {
        renderCount += 1;
        return { html: `<svg data-rendered="${renderCount}"></svg>` };
      },
    });

    const action = diagramRuntime(root, { theme: "light", contentKey: "one" });
    await new Promise((resolve) => setTimeout(resolve, 0));

    root.innerHTML = createDiagramPlaceholder("mermaid", "flowchart TD\nB --> C");
    action.update({ theme: "light", contentKey: "two" });
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(root.querySelector("[data-rendered='2']")).toBeTruthy();
  });

  it("restores placeholders and rerenders when only the theme changes", async () => {
    const root = document.createElement("div");
    root.innerHTML = createDiagramPlaceholder("mermaid", "flowchart TD\nA --> B");
    const themes: string[] = [];

    setDiagramRendererForTest("mermaid", {
      type: "mermaid",
      async render(request) {
        themes.push(request.theme);
        return { html: `<svg data-theme="${request.theme}"></svg>` };
      },
    });

    const action = diagramRuntime(root, { theme: "light", contentKey: "same" });
    await new Promise((resolve) => setTimeout(resolve, 0));

    action.update({ theme: "dark", contentKey: "same" });
    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(themes).toEqual(["light", "dark"]);
    expect(root.querySelector("[data-theme='dark']")).toBeTruthy();
    expect(root.querySelector("[data-diagram-placeholder='true']")).toBeNull();
  });
});
