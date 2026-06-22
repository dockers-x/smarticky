export interface MermaidDiagramVariant {
  name: string;
  aliases: string[];
  label: string;
  declaration: string;
  group: MermaidDiagramVariantGroup;
  description: {
    zh: string;
    en: string;
  };
  template: string;
}

export type MermaidDiagramVariantGroup =
  | "flow"
  | "structure"
  | "planning"
  | "data"
  | "advanced";

export const mermaidDiagramVariants: MermaidDiagramVariant[] = [
  {
    name: "flowchart",
    aliases: ["graph", "mermaid-flowchart"],
    label: "Flowchart",
    declaration: "flowchart TD",
    group: "flow",
    description: { zh: "流程和判断分支", en: "Process or decision flow" },
    template: `flowchart TD
  A[Start] --> B{Ready?}
  B -- Yes --> C[Ship]
  B -- No --> D[Revise]
  D --> B`,
  },
  {
    name: "sequenceDiagram",
    aliases: ["sequence", "mermaid-sequence"],
    label: "Sequence",
    declaration: "sequenceDiagram",
    group: "flow",
    description: { zh: "按时间展示消息交互", en: "Message exchange over time" },
    template: `sequenceDiagram
  participant Alice
  participant Bob
  Alice->>Bob: Hello
  Bob-->>Alice: Hi`,
  },
  {
    name: "classDiagram",
    aliases: ["class", "mermaid-class"],
    label: "Class",
    declaration: "classDiagram",
    group: "structure",
    description: { zh: "类、属性和关系", en: "Classes and relationships" },
    template: `classDiagram
  class Note {
    +String title
    +save()
  }
  Note <|-- DiagramNote`,
  },
  {
    name: "stateDiagram-v2",
    aliases: ["stateDiagram", "state", "mermaid-state"],
    label: "State",
    declaration: "stateDiagram-v2",
    group: "flow",
    description: { zh: "生命周期和状态变化", en: "Lifecycle and state changes" },
    template: `stateDiagram-v2
  [*] --> Draft
  Draft --> Published
  Published --> [*]`,
  },
  {
    name: "erDiagram",
    aliases: ["er", "mermaid-er"],
    label: "ER",
    declaration: "erDiagram",
    group: "structure",
    description: { zh: "实体和数据关系", en: "Entity relationships" },
    template: `erDiagram
  USER ||--o{ NOTE : owns
  NOTE ||--o{ TAG : has
  USER {
    string id
    string name
  }
  NOTE {
    string id
    string title
  }`,
  },
  {
    name: "gantt",
    aliases: ["mermaid-gantt"],
    label: "Gantt",
    declaration: "gantt",
    group: "planning",
    description: { zh: "项目计划和排期", en: "Project schedule" },
    template: `gantt
  title Release plan
  dateFormat YYYY-MM-DD
  section Build
  Design :a1, 2026-06-22, 2d
  Test :after a1, 2d`,
  },
  {
    name: "journey",
    aliases: ["userJourney", "mermaid-journey"],
    label: "Journey",
    declaration: "journey",
    group: "planning",
    description: { zh: "用户旅程步骤", en: "User experience steps" },
    template: `journey
  title Writing a note
  section Draft
    Capture idea: 5: User
    Refine content: 4: User
  section Share
    Export image: 3: User`,
  },
  {
    name: "pie",
    aliases: ["pieChart", "mermaid-pie"],
    label: "Pie",
    declaration: "pie",
    group: "data",
    description: { zh: "简单占比", en: "Simple proportions" },
    template: `pie title Work split
  "Build" : 45
  "Test" : 30
  "Ship" : 25`,
  },
  {
    name: "gitGraph",
    aliases: ["gitgraph", "mermaid-git"],
    label: "Git graph",
    declaration: "gitGraph",
    group: "planning",
    description: { zh: "分支和合并", en: "Branches and merges" },
    template: `gitGraph
  commit
  branch feature
  checkout feature
  commit
  checkout main
  merge feature`,
  },
  {
    name: "mindmap",
    aliases: ["mermaid-mindmap"],
    label: "Mindmap",
    declaration: "mindmap",
    group: "structure",
    description: { zh: "层级想法", en: "Nested ideas" },
    template: `mindmap
  root((Smarticky))
    Notes
    Diagrams
      Mermaid
      drawio`,
  },
  {
    name: "timeline",
    aliases: ["mermaid-timeline"],
    label: "Timeline",
    declaration: "timeline",
    group: "planning",
    description: { zh: "按顺序记录事件", en: "Events in order" },
    template: `timeline
  title Project timeline
  Planning : Requirements
  Build : Editor templates
  Release : Verify and tag`,
  },
  {
    name: "requirementDiagram",
    aliases: ["requirement", "mermaid-requirement"],
    label: "Requirement",
    declaration: "requirementDiagram",
    group: "planning",
    description: { zh: "需求和验证方式", en: "Requirements and verification" },
    template: `requirementDiagram
  requirement editor_req {
    id: 1
    text: Diagram templates
    risk: medium
    verifymethod: test
  }`,
  },
  {
    name: "C4Context",
    aliases: ["c4", "c4context", "mermaid-c4"],
    label: "C4 context",
    declaration: "C4Context",
    group: "structure",
    description: { zh: "系统上下文", en: "System context" },
    template: `C4Context
  title Smarticky context
  Person(user, "User")
  System(app, "Smarticky")
  Rel(user, app, "Writes notes")`,
  },
  {
    name: "quadrantChart",
    aliases: ["quadrant", "mermaid-quadrant"],
    label: "Quadrant",
    declaration: "quadrantChart",
    group: "data",
    description: { zh: "双轴优先级", en: "Two-axis prioritization" },
    template: `quadrantChart
  title Priority
  x-axis Low Effort --> High Effort
  y-axis Low Impact --> High Impact
  quadrant-1 Plan
  quadrant-2 Do
  quadrant-3 Drop
  quadrant-4 Later
  Templates: [0.35, 0.75]`,
  },
  {
    name: "xychart-beta",
    aliases: ["xychart", "mermaid-xy"],
    label: "XY chart",
    declaration: "xychart-beta",
    group: "data",
    description: { zh: "折线或柱状图", en: "Line or bar chart" },
    template: `xychart-beta
  title "Notes per week"
  x-axis [Mon, Tue, Wed, Thu, Fri]
  y-axis "Notes" 0 --> 10
  line [2, 4, 5, 3, 8]`,
  },
  {
    name: "sankey-beta",
    aliases: ["sankey", "mermaid-sankey"],
    label: "Sankey",
    declaration: "sankey-beta",
    group: "data",
    description: { zh: "流量和转化", en: "Flow volume" },
    template: `sankey-beta
  Draft,Review,8
  Review,Published,5
  Review,Rewrite,3`,
  },
  {
    name: "block-beta",
    aliases: ["block", "mermaid-block"],
    label: "Block",
    declaration: "block-beta",
    group: "advanced",
    description: { zh: "块状布局", en: "Block layout" },
    template: `block-beta
  columns 3
  a["Idea"] b["Draft"] c["Publish"]
  a --> b
  b --> c`,
  },
  {
    name: "architecture-beta",
    aliases: ["architecture", "mermaid-architecture"],
    label: "Architecture",
    declaration: "architecture-beta",
    group: "advanced",
    description: { zh: "架构关系图", en: "Architecture map" },
    template: `architecture-beta
  group app(cloud)[Smarticky]
  service editor(internet)[Editor] in app
  service notes(database)[Notes] in app
  editor:R -- L:notes`,
  },
  {
    name: "kanban",
    aliases: ["mermaid-kanban"],
    label: "Kanban",
    declaration: "kanban",
    group: "planning",
    description: { zh: "看板列和任务", en: "Board columns and tasks" },
    template: `kanban
  Todo
    [Write template]
  Doing
    [Preview diagram]
  Done
    [Export image]`,
  },
  {
    name: "packet",
    aliases: ["packet-beta", "mermaid-packet"],
    label: "Packet",
    declaration: "packet",
    group: "advanced",
    description: { zh: "网络包字段", en: "Network packet fields" },
    template: `packet
  0-15: "Source Port"
  16-31: "Destination Port"
  32-63: "Sequence Number"`,
  },
];

const variantsByLanguage = new Map<string, MermaidDiagramVariant>();

for (const variant of mermaidDiagramVariants) {
  variantsByLanguage.set(variant.name.toLowerCase(), variant);
  for (const alias of variant.aliases) {
    variantsByLanguage.set(alias.toLowerCase(), variant);
  }
}

const declarationPatterns = [
  /^flowchart\b/i,
  /^graph\b/i,
  /^sequenceDiagram\b/i,
  /^classDiagram\b/i,
  /^stateDiagram(?:-v2)?\b/i,
  /^erDiagram\b/i,
  /^journey\b/i,
  /^gantt\b/i,
  /^pie\b/i,
  /^gitGraph\b/i,
  /^mindmap\b/i,
  /^timeline\b/i,
  /^requirementDiagram\b/i,
  /^C4(?:Context|Container|Component|Dynamic|Deployment)\b/i,
  /^quadrantChart\b/i,
  /^xychart-beta\b/i,
  /^sankey-beta\b/i,
  /^block-beta\b/i,
  /^architecture-beta\b/i,
  /^kanban\b/i,
  /^packet\b/i,
  /^packet-beta\b/i,
];

export function findMermaidDiagramVariant(
  language: string | null | undefined,
): MermaidDiagramVariant | null {
  return variantsByLanguage.get((language || "").trim().toLowerCase()) ?? null;
}

export function isMermaidDiagramLanguage(language: string | null | undefined): boolean {
  const normalized = (language || "").trim().toLowerCase();
  return normalized === "mermaid" || variantsByLanguage.has(normalized);
}

export function hasMermaidDiagramDeclaration(source: string): boolean {
  const firstLine = source.trimStart().split(/\r?\n/, 1)[0].trim();
  return declarationPatterns.some((pattern) => pattern.test(firstLine));
}

function hasFlowchartDeclaration(source: string): boolean {
  const firstLine = source.trimStart().split(/\r?\n/, 1)[0].trim();
  return /^flowchart\b/i.test(firstLine) || /^graph\b/i.test(firstLine);
}

export function normalizeFlowchartShorthand(source: string): string {
  return source.replace(/(^|[^-])->(?!>)/gm, "$1-->");
}

export function materializeMermaidSource(
  language: string | null | undefined,
  source: string,
): string {
  const trimmedSource = source.trim();
  if (!trimmedSource) return trimmedSource;

  if (hasMermaidDiagramDeclaration(trimmedSource)) {
    return hasFlowchartDeclaration(trimmedSource)
      ? normalizeFlowchartShorthand(trimmedSource)
      : trimmedSource;
  }

  const variant = findMermaidDiagramVariant(language);
  if (!variant) return trimmedSource;

  const materialized = `${variant.declaration}\n${trimmedSource}`;
  return variant.name === "flowchart"
    ? normalizeFlowchartShorthand(materialized)
    : materialized;
}

export function createMermaidDiagramFence(variant: MermaidDiagramVariant): string {
  return ["```mermaid", variant.template.trim(), "```"].join("\n");
}
