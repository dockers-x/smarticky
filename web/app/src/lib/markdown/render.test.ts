import { afterEach, describe, expect, it, vi } from "vitest";
import { decodeDiagramSource } from "./diagrams/placeholders";
import { preserveCodeGroups } from "./codeGroups";
import { renderMarkdown, stripMarkdown } from "./render";
import { protectedImagePlaceholderSrc, protectedImageRuntime } from "./protectedImages";

function getPlaceholder(html: string): HTMLElement {
  const root = document.createElement("div");
  root.innerHTML = html;
  const node = root.querySelector<HTMLElement>("[data-diagram-placeholder='true']");
  if (!node) throw new Error("diagram placeholder missing");
  return node;
}

function getWhiteboardReference(html: string): HTMLElement {
  const root = document.createElement("div");
  root.innerHTML = html;
  const node = root.querySelector<HTMLElement>("[data-whiteboard-reference='true']");
  if (!node) throw new Error("whiteboard reference missing");
  return node;
}

const whiteboardID = "123e4567-e89b-12d3-a456-426614174000";

afterEach(() => {
  globalThis.localStorage?.clear?.();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

function flushPromises(): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, 0));
}

function mockObjectURL(): {
  createObjectURL: ReturnType<typeof vi.fn>;
  revokeObjectURL: ReturnType<typeof vi.fn>;
  restore: () => void;
} {
  const originalCreateObjectURL = URL.createObjectURL;
  const originalRevokeObjectURL = URL.revokeObjectURL;
  const createObjectURL = vi
    .fn()
    .mockReturnValueOnce("blob:first")
    .mockReturnValueOnce("blob:second");
  const revokeObjectURL = vi.fn();

  Object.defineProperty(URL, "createObjectURL", {
    configurable: true,
    value: createObjectURL,
  });
  Object.defineProperty(URL, "revokeObjectURL", {
    configurable: true,
    value: revokeObjectURL,
  });

  return {
    createObjectURL,
    revokeObjectURL,
    restore() {
      if (originalCreateObjectURL) {
        Object.defineProperty(URL, "createObjectURL", {
          configurable: true,
          value: originalCreateObjectURL,
        });
      } else {
        delete (URL as Partial<typeof URL>).createObjectURL;
      }
      if (originalRevokeObjectURL) {
        Object.defineProperty(URL, "revokeObjectURL", {
          configurable: true,
          value: originalRevokeObjectURL,
        });
      } else {
        delete (URL as Partial<typeof URL>).revokeObjectURL;
      }
    },
  };
}

describe("renderMarkdown diagram placeholders", () => {
  it("emits a safe Mermaid placeholder", () => {
    const source = "flowchart TD\n  A --> B";
    const html = renderMarkdown(`\`\`\`mermaid\n${source}\n\`\`\``);
    const node = getPlaceholder(html);

    expect(node.dataset.diagramType).toBe("mermaid");
    expect(decodeDiagramSource(node.dataset.diagramSource || "")).toBe(source);
    expect(html).not.toContain("<script");
  });

  it("emits a Mermaid placeholder for selected Mermaid diagram types", () => {
    const html = renderMarkdown("```flowchart\nA->B\nB->C\n```");
    const node = getPlaceholder(html);

    expect(node.dataset.diagramType).toBe("mermaid");
    expect(decodeDiagramSource(node.dataset.diagramSource || "")).toBe(
      "flowchart TD\nA-->B\nB-->C",
    );
  });

  it("emits a safe drawio placeholder", () => {
    const source = "<mxfile><diagram>safe</diagram></mxfile>";
    const html = renderMarkdown(`\`\`\`drawio\n${source}\n\`\`\``);
    const node = getPlaceholder(html);

    expect(node.dataset.diagramType).toBe("drawio");
    expect(decodeDiagramSource(node.dataset.diagramSource || "")).toBe(source);
  });

  it("accepts drawio XML copied from a .drawio file", () => {
    const source = [
      "<mxfile>",
      '  <diagram name="Page-1">',
      "    <mxGraphModel><root><mxCell id=\"0\"/></root></mxGraphModel>",
      "  </diagram>",
      "</mxfile>",
    ].join("\n");
    const html = renderMarkdown(`\`\`\`drawio\n${source}\n\`\`\``);
    const node = getPlaceholder(html);

    expect(node.dataset.diagramType).toBe("drawio");
    expect(decodeDiagramSource(node.dataset.diagramSource || "")).toBe(source);
  });

  it("emits a safe Excalidraw whiteboard reference", () => {
    const html = renderMarkdown(
      `\`\`\`excalidraw\nwhiteboard: ${whiteboardID}\n\`\`\``,
    );
    const node = getWhiteboardReference(html);

    expect(node.dataset.whiteboardId).toBe(whiteboardID);
    expect(html).not.toContain("<script");
  });

  it("keeps unsupported diagram fences as normal code blocks", () => {
    const html = renderMarkdown("```plantuml\n@startuml\nA -> B\n@enduml\n```");

    expect(html).toContain("<pre>");
    expect(html).toContain("@startuml");
    expect(html).not.toContain("data-diagram-placeholder");
  });

  it("keeps empty supported fences as normal code blocks", () => {
    const html = renderMarkdown("```mermaid\n\n```");

    expect(html).toContain("<pre>");
    expect(html).not.toContain("data-diagram-placeholder");
  });
});

describe("renderMarkdown code groups", () => {
  it("renders VitePress code-group fences as tabs", () => {
    const html = renderMarkdown(
      [
        "::: code-group",
        "```bash [pnpm]",
        "pnpm install",
        "```",
        "```bash [yarn]",
        "yarn install",
        "```",
        ":::",
      ].join("\n"),
    );
    const root = document.createElement("div");
    root.innerHTML = html;

    expect(root.querySelector(".markdown-code-group")).toBeTruthy();
    expect(root.querySelectorAll(".markdown-code-tab")).toHaveLength(2);
    expect(root.querySelector(".markdown-code-tab")?.textContent).toBe("pnpm");
    expect(root.querySelector(".markdown-code-panel.active")?.textContent).toContain(
      "pnpm install",
    );
  });

  it("renders unlabeled code-group fences with fallback tab labels", () => {
    const html = renderMarkdown(
      [
        "::: code-group",
        "```bash",
        "pnpm install",
        "```",
        "```bash",
        "yarn install",
        "```",
        ":::",
      ].join("\n"),
    );
    const root = document.createElement("div");
    root.innerHTML = html;

    expect([...root.querySelectorAll(".markdown-code-tab")].map((node) => node.textContent)).toEqual([
      "pnpm",
      "yarn",
    ]);
    expect(root.textContent).not.toContain("::: code-group");
  });

  it("renders VuePress code-tabs blocks as tabs", () => {
    const html = renderMarkdown(
      [
        "::: code-tabs",
        "@tab yarn",
        "```bash",
        "yarn create vuepress vuepress-starter",
        "```",
        "@tab npm",
        "```bash",
        "npm init vuepress vuepress-starter",
        "```",
        ":::",
      ].join("\n"),
    );
    const root = document.createElement("div");
    root.innerHTML = html;

    expect([...root.querySelectorAll(".markdown-code-tab")].map((node) => node.textContent)).toEqual([
      "yarn",
      "npm",
    ]);
    expect(root.textContent).toContain("npm init vuepress");
  });

  it("escapes code group labels and source", () => {
    const html = renderMarkdown(
      [
        "::: code-group",
        "```bash [<img src=x onerror=alert(1)>]",
        "<script>alert(1)</script>",
        "```",
        ":::",
      ].join("\n"),
    );

    expect(html).not.toContain("<script");
    expect(html).not.toContain("<img");
    expect(html).toContain("&lt;script&gt;alert(1)&lt;/script&gt;");
  });

  it("preserves original code-group tab labels after editor serialization", () => {
    const original = [
      "Intro",
      "::: code-group",
      "```bash [pnpm]",
      "pnpm install",
      "```",
      "```bash [npm]",
      "npm install",
      "```",
      ":::",
      "Outro",
    ].join("\n");
    const serialized = [
      "Intro changed",
      "::: code-group",
      "```bash",
      "pnpm install",
      "```",
      "```bash",
      "npm install",
      "```",
      ":::",
      "Outro",
    ].join("\n");

    const preserved = preserveCodeGroups(serialized, original);

    expect(preserved).toContain("```bash [pnpm]");
    expect(preserved).toContain("```bash [npm]");
    expect(preserved).toContain("Intro changed");
  });

  it("preserves labels for a newly inserted code-group template", () => {
    const inserted = [
      "::: code-group",
      "```bash [pnpm]",
      "pnpm install",
      "```",
      "```bash [yarn]",
      "yarn install",
      "```",
      ":::",
    ].join("\n");
    const serialized = [
      "Intro",
      "::: code-group",
      "```bash",
      "pnpm install",
      "```",
      "```bash",
      "yarn install",
      "```",
      ":::",
    ].join("\n");

    const preserved = preserveCodeGroups(serialized, "Intro", [inserted]);

    expect(preserved).toContain("```bash [pnpm]");
    expect(preserved).toContain("```bash [yarn]");
  });

  it("matches preserved code-groups by code content instead of position", () => {
    const existing = [
      "::: code-group",
      "```bash [old]",
      "old command",
      "```",
      ":::",
    ].join("\n");
    const inserted = [
      "::: code-group",
      "```bash [new]",
      "new command",
      "```",
      ":::",
    ].join("\n");
    const serialized = [
      "::: code-group",
      "```bash",
      "new command",
      "```",
      ":::",
      "Middle",
      "::: code-group",
      "```bash",
      "old command",
      "```",
      ":::",
    ].join("\n");

    const preserved = preserveCodeGroups(serialized, existing, [inserted]);
    const newIndex = preserved.indexOf("```bash [new]");
    const oldIndex = preserved.indexOf("```bash [old]");

    expect(newIndex).toBeGreaterThanOrEqual(0);
    expect(oldIndex).toBeGreaterThan(newIndex);
  });
});

describe("renderMarkdown images", () => {
  it("renders empty image URLs as placeholders instead of broken images", () => {
    const html = renderMarkdown("![Screenshot]()");
    const root = document.createElement("div");
    root.innerHTML = html;

    expect(root.querySelector("img")).toBeNull();
    expect(root.querySelector(".markdown-image-placeholder")?.textContent).toContain(
      "Screenshot",
    );
  });

  it("marks protected attachment images for authorized loading", () => {
    const html = renderMarkdown("![Diagram](/api/attachments/12/download)");
    const root = document.createElement("div");
    root.innerHTML = html;
    const image = root.querySelector("img");

    expect(image?.getAttribute("src")).toBe(protectedImagePlaceholderSrc);
    expect(image?.dataset.authImage).toBe("true");
    expect(image?.dataset.authSrc).toBe("/api/attachments/12/download");
    expect(image?.getAttribute("alt")).toBe("Diagram");
  });

  it("reloads protected images when rendered markdown content changes", async () => {
    const objectURLMock = mockObjectURL();
    const fetchMock = vi.fn(async () => new Response(new Blob(["image"])));
    vi.stubGlobal("fetch", fetchMock);
    vi.stubGlobal("localStorage", {
      clear: vi.fn(),
      getItem: vi.fn((key: string) => (key === "jwt_token" ? "test-token" : null)),
      removeItem: vi.fn(),
      setItem: vi.fn(),
    });

    const root = document.createElement("div");
    root.innerHTML = renderMarkdown("![First](/api/attachments/12/download)");

    const runtime = protectedImageRuntime(root, { contentKey: "first" });
    await flushPromises();

    const firstImage = root.querySelector("img");
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/attachments/12/download",
      expect.objectContaining({ headers: expect.any(Headers) }),
    );
    expect(firstImage?.getAttribute("src")).toBe("blob:first");
    expect(firstImage?.dataset.authImageLoaded).toBe("true");

    root.innerHTML = renderMarkdown("![Second](/api/attachments/13/download)");
    runtime.update({ contentKey: "second" });
    await flushPromises();

    const secondImage = root.querySelector("img");
    expect(fetchMock).toHaveBeenLastCalledWith(
      "/api/attachments/13/download",
      expect.objectContaining({ headers: expect.any(Headers) }),
    );
    expect(objectURLMock.revokeObjectURL).toHaveBeenCalledWith("blob:first");
    expect(secondImage?.getAttribute("src")).toBe("blob:second");

    runtime.destroy();
    expect(objectURLMock.revokeObjectURL).toHaveBeenCalledWith("blob:second");
    objectURLMock.restore();
  });
});

describe("stripMarkdown", () => {
  it("does not count Mermaid, drawio, or whiteboard references as visible prose", () => {
    const markdown = [
      "Visible",
      "```mermaid",
      "flowchart TD",
      "A --> B",
      "```",
      "```excalidraw",
      `whiteboard: ${whiteboardID}`,
      "```",
    ].join("\n");

    expect(stripMarkdown(markdown)).toBe("Visible");
  });
});
