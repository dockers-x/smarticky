<script lang="ts">
  import { onDestroy, onMount, tick } from "svelte";
  import type {
    Core,
    ElementDefinition,
    EventObject,
    StylesheetJson,
  } from "cytoscape";

  type IndexGraphNodeType = "root" | "note" | "tag" | "folder" | "protection" | "relation";
  type IndexGraphLinkKind = "membership" | "backlink";

  interface IndexGraphNode {
    id: string;
    type: IndexGraphNodeType;
    label: string;
    count: number;
  }

  interface IndexGraphLink {
    source: string;
    target: string;
    kind?: IndexGraphLinkKind;
  }

  export let nodes: IndexGraphNode[] = [];
  export let links: IndexGraphLink[] = [];
  export let selectedNodeID = "root";
  export let ariaLabel = "";
  export let theme = "light";
  export let onSelectNode: (nodeID: string) => void = () => {};

  let container: HTMLDivElement;
  let cy: Core | null = null;
  let resizeObserver: ResizeObserver | null = null;
  let mounted = false;
  let destroyed = false;
  let lastElementSignature = "";
  let resizeFrame = 0;

  function cssVar(name: string, fallback: string): string {
    if (!container) return fallback;
    const value = getComputedStyle(container).getPropertyValue(name).trim();
    return value || fallback;
  }

  function shortLabel(label: string, maxLength: number): string {
    const trimmed = label.trim();
    if (trimmed.length <= maxLength) return trimmed;
    return `${trimmed.slice(0, Math.max(1, maxLength - 1))}...`;
  }

  function buildElements(): ElementDefinition[] {
    return [
      ...nodes.map((node) => ({
        group: "nodes" as const,
        data: {
          id: node.id,
          label: shortLabel(node.label, node.type === "note" ? 28 : 20),
          fullLabel: node.label,
          type: node.type,
          count: node.count,
        },
        classes: `index-cy-node index-cy-node--${node.type}`,
      })),
      ...links.map((link, index) => ({
        group: "edges" as const,
        data: {
          id: `${link.source}->${link.target}:${index}`,
          source: link.source,
          target: link.target,
          kind: link.kind ?? "membership",
        },
        classes: `index-cy-link index-cy-link--${link.kind ?? "membership"}`,
      })),
    ];
  }

  function elementSignature(): string {
    return JSON.stringify({
      nodes: nodes.map((node) => [node.id, node.type, node.label, node.count]),
      links: links.map((link) => [link.source, link.target, link.kind ?? "membership"]),
    });
  }

  function graphStyles(): StylesheetJson {
    const page = cssVar("--color-page", "#ffffff");
    const card = cssVar("--color-card", "#ffffff");
    const divider = cssVar("--color-divider", "#d7dce2");
    const dividerEm = cssVar("--color-divider-em", "#aeb7c2");
    const text = cssVar("--color-text", "#20242a");
    const muted = cssVar("--color-text-muted", "#69717d");
    const brand = cssVar("--color-brand", "#2563eb");
    const ink = cssVar("--color-ink-accent", "#304656");
    const graphPrimary = cssVar("--color-graph-primary", "#2d776f");
    const graphSecondary = cssVar("--color-graph-secondary", "#3d6f9f");
    const info = cssVar("--sm-info", "#2563eb");
    const infoBg = cssVar("--sm-info-bg", "#eaf2ff");
    const success = cssVar("--sm-success", "#138a55");
    const successBg = cssVar("--sm-success-bg", "#e8f7ef");
    const warning = cssVar("--sm-warning", "#b7791f");
    const warningBg = cssVar("--sm-warning-bg", "#fff4d8");

    return [
      {
        selector: "core",
        style: {
          "active-bg-color": brand,
          "active-bg-opacity": 0,
          "active-bg-size": 0,
          "selection-box-color": brand,
          "selection-box-opacity": 0.08,
          "selection-box-border-color": brand,
          "selection-box-border-width": 1,
          "outside-texture-bg-color": page,
          "outside-texture-bg-opacity": 0,
        },
      },
      {
        selector: "edge",
        style: {
          width: 1.4,
          "line-color": dividerEm,
          opacity: 0.5,
          "curve-style": "bezier",
          "target-arrow-shape": "none",
          "overlay-opacity": 0,
        },
      },
      {
        selector: 'edge[kind = "backlink"]',
        style: {
          width: 1.8,
          "line-color": graphSecondary,
          opacity: 0.5,
          "target-arrow-shape": "triangle",
          "target-arrow-color": graphSecondary,
          "arrow-scale": 0.82,
          "curve-style": "bezier",
        },
      },
      {
        selector: "node",
        style: {
          width: 18,
          height: 18,
          label: "data(label)",
          color: muted,
          "background-color": card,
          "border-color": dividerEm,
          "border-width": 1.5,
          "font-size": 11,
          "font-weight": 500,
          "min-zoomed-font-size": 8,
          "text-background-color": page,
          "text-background-opacity": 0.86,
          "text-background-padding": "3px",
          "text-margin-y": 8,
          "text-max-width": "96px",
          "text-valign": "bottom",
          "text-wrap": "ellipsis",
          "overlay-opacity": 0,
        },
      },
      {
        selector: 'node[type = "root"]',
        style: {
          width: 48,
          height: 48,
          color: text,
          "background-color": ink,
          "border-color": ink,
          "border-width": 2,
          "font-size": 12,
          "font-weight": 700,
          "text-background-opacity": 0.92,
        },
      },
      {
        selector: 'node[type = "note"]',
        style: {
          width: 13,
          height: 13,
          label: "",
          "background-color": muted,
          "border-color": card,
          "border-width": 1.5,
        },
      },
      {
        selector: 'node[type = "tag"]',
        style: {
          width: "mapData(count, 1, 48, 20, 42)",
          height: "mapData(count, 1, 48, 20, 42)",
          "background-color": infoBg,
          "border-color": graphSecondary || info,
        },
      },
      {
        selector: 'node[type = "folder"]',
        style: {
          width: "mapData(count, 1, 48, 20, 42)",
          height: "mapData(count, 1, 48, 20, 42)",
          "background-color": successBg,
          "border-color": graphPrimary || success,
        },
      },
      {
        selector: 'node[type = "protection"]',
        style: {
          width: "mapData(count, 1, 48, 20, 42)",
          height: "mapData(count, 1, 48, 20, 42)",
          "background-color": warningBg,
          "border-color": warning,
        },
      },
      {
        selector: 'node[type = "relation"]',
        style: {
          width: "mapData(count, 1, 48, 24, 46)",
          height: "mapData(count, 1, 48, 24, 46)",
          "background-color": warningBg,
          "border-color": graphPrimary,
        },
      },
      {
        selector: ".is-neighbor",
        style: {
          opacity: 1,
        },
      },
      {
        selector: ".is-dimmed",
        style: {
          opacity: 0.18,
        },
      },
      {
        selector: "edge.is-neighbor",
        style: {
          width: 2.2,
          "line-color": graphSecondary,
          "target-arrow-color": graphSecondary,
          opacity: 0.82,
        },
      },
      {
        selector: "node.is-active",
        style: {
          label: "data(label)",
          color: text,
          "border-color": brand,
          "border-width": 4,
          "font-size": 12,
          "font-weight": 700,
          "text-background-opacity": 0.94,
        },
      },
      {
        selector: 'node[type = "note"].is-active',
        style: {
          width: 24,
          height: 24,
          "background-color": graphSecondary,
          "border-color": graphSecondary,
        },
      },
      {
        selector: ":selected",
        style: {
          "border-color": brand,
          "border-width": 4,
        },
      },
    ];
  }

  function runLayout(fit = true): void {
    if (!cy) return;
    cy.layout({
      name: "concentric",
      animate: false,
      fit,
      padding: 34,
      minNodeSpacing: 26,
      avoidOverlap: true,
      concentric(node) {
        const type = node.data("type");
        if (type === "root") return 4;
        if (type === "note") return 3;
        return 2;
      },
      levelWidth() {
        return 1;
      },
    }).run();
  }

  function applySelection(fitSelection = false): void {
    if (!cy) return;
    cy.elements().removeClass("is-active is-neighbor is-dimmed");

    const selected = cy.getElementById(selectedNodeID);
    if (selected.empty()) {
      if (fitSelection) cy.fit(undefined, 34);
      return;
    }

    const neighborhood = selected.closedNeighborhood();
    selected.addClass("is-active");
    neighborhood.addClass("is-neighbor");
    cy.elements().difference(neighborhood).addClass("is-dimmed");

    if (fitSelection) {
      cy.stop();
      cy.animate(
        {
          fit: {
            eles: selectedNodeID === "root" ? cy.elements() : neighborhood,
            padding: selectedNodeID === "root" ? 34 : 54,
          },
        },
        { duration: 180 },
      );
    }
  }

  function fitGraphToContainer(): void {
    if (!cy) return;
    cy.resize();
    cy.fit(undefined, 34);
    applySelection(false);
  }

  function scheduleGraphFit(): void {
    if (resizeFrame) cancelAnimationFrame(resizeFrame);
    resizeFrame = requestAnimationFrame(() => {
      resizeFrame = requestAnimationFrame(() => {
        resizeFrame = 0;
        fitGraphToContainer();
      });
    });
  }

  function updateGraph(): void {
    if (!cy) return;
    const signature = elementSignature();
    if (signature === lastElementSignature) return;

    lastElementSignature = signature;
    cy.batch(() => {
      cy?.elements().remove();
      cy?.add(buildElements());
    });
    runLayout(true);
    applySelection(false);
  }

  function handleNodeTap(event: EventObject): void {
    const id = event.target.id();
    if (id) onSelectNode(id);
  }

  function handleBackgroundTap(event: EventObject): void {
    if (event.target === cy) onSelectNode("root");
  }

  onMount(() => {
    void import("cytoscape").then((module) => {
      if (destroyed) return;
      cy = module.default({
        container,
        elements: buildElements(),
        layout: { name: "concentric" },
        maxZoom: 2.4,
        minZoom: 0.28,
        style: graphStyles(),
        wheelSensitivity: 0.18,
      });
      cy.on("tap", "node", handleNodeTap);
      cy.on("tap", handleBackgroundTap);
      runLayout(true);
      applySelection(false);
      resizeObserver = new ResizeObserver(() => {
        scheduleGraphFit();
      });
      resizeObserver.observe(container);
      mounted = true;
      lastElementSignature = elementSignature();
    });
  });

  onDestroy(() => {
    destroyed = true;
    if (resizeFrame) cancelAnimationFrame(resizeFrame);
    resizeFrame = 0;
    resizeObserver?.disconnect();
    resizeObserver = null;
    cy?.destroy();
    cy = null;
  });

  $: if (mounted) {
    updateGraph();
  }

  $: if (mounted && cy) {
    selectedNodeID;
    applySelection(true);
  }

  $: if (mounted && cy) {
    theme;
    void tick().then(() => {
      if (!cy) return;
      cy.style(graphStyles());
      applySelection(false);
    });
  }
</script>

<div class="index-graph-canvas" bind:this={container} role="img" aria-label={ariaLabel}></div>

<div class="visually-hidden" aria-label={ariaLabel}>
  {#each nodes as node (node.id)}
    <button type="button" on:click={() => onSelectNode(node.id)}>
      {node.label}
    </button>
  {/each}
</div>
