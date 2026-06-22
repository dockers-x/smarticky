export interface CodeGroupItem {
  label: string;
  lang: string;
  code: string;
}

export interface CodeGroupBlock {
  kind: "code-group" | "code-tabs";
  marker: string;
  startLine: number;
  endLine: number;
  items: CodeGroupItem[];
}

export interface CodeGroupSourceBlock extends CodeGroupBlock {
  explicitLabelCount: number;
  raw: string;
  signature: string;
}

function escapeHtml(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function codeFenceStart(line: string): { fence: string; info: string } | null {
  const match = line.match(/^(`{3,}|~{3,})(.*)$/);
  if (!match) return null;
  return {
    fence: match[1],
    info: match[2].trim(),
  };
}

function isFenceEnd(line: string, fence: string): boolean {
  return line.trim() === fence;
}

function cleanTabLabel(label: string): string {
  return label
    .replace(/^[:\s]*(active\s+)?/i, "")
    .replace(/^["']|["']$/g, "")
    .trim();
}

function codeInfoParts(info: string): { lang: string; label: string } | null {
  const match = info.match(/\[([^\]]+)\]\s*$/);
  if (!match) return null;
  const label = cleanTabLabel(match[1]);
  if (!label) return null;

  return {
    lang: info.slice(0, match.index).trim(),
    label,
  };
}

function collectFencedCode(
  lines: string[],
  startIndex: number,
): { nextIndex: number; lang: string; code: string } | null {
  const start = codeFenceStart(lines[startIndex]);
  if (!start) return null;

  const codeLines: string[] = [];
  let index = startIndex + 1;
  while (index < lines.length && !isFenceEnd(lines[index], start.fence)) {
    codeLines.push(lines[index]);
    index += 1;
  }

  if (index >= lines.length) return null;
  return {
    nextIndex: index + 1,
    lang: start.info.trim(),
    code: codeLines.join("\n"),
  };
}

function stripCodeInfoLabel(info: string): string {
  const parts = codeInfoParts(info);
  return parts ? parts.lang : info.trim();
}

function fallbackCodeGroupLabel(lang: string, code: string, index: number): string {
  const normalizedLang = lang.split(/\s+/)[0].toLowerCase();
  if (["bash", "sh", "shell", "zsh", "console", "terminal"].includes(normalizedLang)) {
    const command = code.trim().match(/^(?:[$>#]\s*)?([A-Za-z][\w.-]*)\b/)?.[1];
    if (command) return command;
  }

  return normalizedLang ? `${normalizedLang}${index === 0 ? "" : ` ${index + 1}`}` : `Tab ${index + 1}`;
}

function signatureForCodeGroupBody(lines: string[]): string {
  const fences: { lang: string; code: string }[] = [];
  let index = 0;

  while (index < lines.length) {
    const start = codeFenceStart(lines[index]);
    if (!start) {
      index += 1;
      continue;
    }

    const fenced = collectFencedCode(lines, index);
    if (!fenced) break;
    fences.push({
      lang: stripCodeInfoLabel(start.info).split(/\s+/)[0] || "",
      code: fenced.code.trim(),
    });
    index = fenced.nextIndex;
  }

  return JSON.stringify(fences);
}

function countExplicitLabels(kind: CodeGroupBlock["kind"], lines: string[]): number {
  if (kind === "code-tabs") {
    return lines.filter((line) => /^@tab(?::active)?\s+(.+)$/.test(line)).length;
  }

  let count = 0;
  for (const line of lines) {
    const start = codeFenceStart(line);
    if (start && codeInfoParts(start.info)) count += 1;
  }
  return count;
}

function parseVitePressCodeGroup(lines: string[]): CodeGroupItem[] {
  const items: CodeGroupItem[] = [];
  let index = 0;

  while (index < lines.length) {
    const start = codeFenceStart(lines[index]);
    if (!start) {
      index += 1;
      continue;
    }

    const fenced = collectFencedCode(lines, index);
    if (!fenced) break;
    const parts = codeInfoParts(start.info);
    const lang = parts?.lang ?? stripCodeInfoLabel(start.info);
    items.push({
      label: parts?.label ?? fallbackCodeGroupLabel(lang, fenced.code, items.length),
      lang,
      code: fenced.code,
    });
    index = fenced.nextIndex;
  }

  return items;
}

function parseVuePressCodeTabs(lines: string[]): CodeGroupItem[] {
  const items: CodeGroupItem[] = [];
  let label = "";
  let index = 0;

  while (index < lines.length) {
    const tab = lines[index].match(/^@tab(?::active)?\s+(.+)$/);
    if (tab) {
      label = cleanTabLabel(tab[1]);
      index += 1;
      continue;
    }

    if (!label || !codeFenceStart(lines[index])) {
      index += 1;
      continue;
    }

    const fenced = collectFencedCode(lines, index);
    if (!fenced) break;
    items.push({
      label,
      lang: fenced.lang,
      code: fenced.code,
    });
    label = "";
    index = fenced.nextIndex;
  }

  return items;
}

export function renderCodeGroup(items: CodeGroupItem[]): string {
  const validItems = items.filter((item) => item.label && item.code.trim());
  if (validItems.length === 0) return "";

  const tabs = validItems
    .map(
      (item, index) =>
        `<button type="button" class="markdown-code-tab${index === 0 ? " active" : ""}" role="tab" aria-selected="${index === 0 ? "true" : "false"}" data-code-tab="${index}">${escapeHtml(item.label)}</button>`,
    )
    .join("");
  const panels = validItems
    .map((item, index) => {
      const lang = item.lang.split(/\s+/)[0] || "";
      const languageClass = lang ? ` class="language-${escapeHtml(lang)}"` : "";
      return [
        `<div class="markdown-code-panel${index === 0 ? " active" : ""}" role="tabpanel" data-code-panel="${index}">`,
        `<pre><code${languageClass}>${escapeHtml(item.code)}</code></pre>`,
        "</div>",
      ].join("");
    })
    .join("");

  return [
    '<div class="markdown-code-group">',
    `<div class="markdown-code-tabs" role="tablist">${tabs}</div>`,
    `<div class="markdown-code-panels">${panels}</div>`,
    "</div>",
  ].join("");
}

export function extractCodeGroupSources(markdown: string): CodeGroupSourceBlock[] {
  const lines = markdown.split(/\r?\n/);
  const groups: CodeGroupSourceBlock[] = [];
  let index = 0;

  while (index < lines.length) {
    const opener = lines[index].match(/^\s*(:{3,})\s+(code-group|code-tabs)(?:[#\s].*)?$/);
    if (!opener) {
      index += 1;
      continue;
    }

    const marker = opener[1];
    const kind = opener[2] as CodeGroupBlock["kind"];
    const body: string[] = [];
    let cursor = index + 1;
    while (cursor < lines.length && lines[cursor].trim() !== marker) {
      body.push(lines[cursor]);
      cursor += 1;
    }

    if (cursor >= lines.length) {
      index += 1;
      continue;
    }

    const items =
      kind === "code-tabs"
        ? parseVuePressCodeTabs(body)
        : parseVitePressCodeGroup(body);
    groups.push({
      kind,
      marker,
      startLine: index,
      endLine: cursor,
      explicitLabelCount: countExplicitLabels(kind, body),
      items,
      raw: lines.slice(index, cursor + 1).join("\n"),
      signature: signatureForCodeGroupBody(body),
    });
    index = cursor + 1;
  }

  return groups;
}

export function extractCodeGroups(markdown: string): CodeGroupBlock[] {
  return extractCodeGroupSources(markdown).filter((group) =>
    Boolean(renderCodeGroup(group.items)),
  );
}

export function preserveCodeGroups(
  nextMarkdown: string,
  previousMarkdown: string,
  extraSources: string[] = [],
): string {
  const previousGroups = [
    ...extractCodeGroupSources(previousMarkdown),
    ...extraSources.flatMap((source) => extractCodeGroupSources(source)),
  ].filter((group) => group.explicitLabelCount > 0 && Boolean(renderCodeGroup(group.items)));
  if (previousGroups.length === 0) return nextMarkdown;

  const lines = nextMarkdown.split(/\r?\n/);
  const output: string[] = [];
  let index = 0;
  const usedReplacements = new Set<number>();

  function findReplacement(
    kind: CodeGroupBlock["kind"],
    signature: string,
  ): CodeGroupSourceBlock | null {
    const matchingIndex = previousGroups.findIndex(
      (group, groupIndex) =>
        !usedReplacements.has(groupIndex) &&
        group.kind === kind &&
        group.signature === signature,
    );
    if (matchingIndex >= 0) {
      usedReplacements.add(matchingIndex);
      return previousGroups[matchingIndex];
    }

    const fallbackIndex = previousGroups.findIndex(
      (group, groupIndex) => !usedReplacements.has(groupIndex) && group.kind === kind,
    );
    if (fallbackIndex >= 0) {
      usedReplacements.add(fallbackIndex);
      return previousGroups[fallbackIndex];
    }

    return null;
  }

  while (index < lines.length) {
    const opener = lines[index].match(/^\s*(:{3,})\s+(code-group|code-tabs)(?:[#\s].*)?$/);
    if (!opener) {
      output.push(lines[index]);
      index += 1;
      continue;
    }

    const marker = opener[1];
    const kind = opener[2] as CodeGroupBlock["kind"];
    const body: string[] = [];
    let cursor = index + 1;
    while (cursor < lines.length && lines[cursor].trim() !== marker) {
      body.push(lines[cursor]);
      cursor += 1;
    }

    if (cursor >= lines.length) {
      output.push(lines[index]);
      index += 1;
      continue;
    }

    const replacement = findReplacement(kind, signatureForCodeGroupBody(body));
    if (replacement) {
      output.push(...replacement.raw.split(/\r?\n/));
    } else {
      output.push(...lines.slice(index, cursor + 1));
    }
    index = cursor + 1;
  }

  return output.join("\n");
}

export function transformCodeGroups(markdown: string): string {
  const lines = markdown.split(/\r?\n/);
  const output: string[] = [];
  let index = 0;

  while (index < lines.length) {
    const opener = lines[index].match(/^\s*(:{3,})\s+(code-group|code-tabs)(?:[#\s].*)?$/);
    if (!opener) {
      output.push(lines[index]);
      index += 1;
      continue;
    }

    const marker = opener[1];
    const kind = opener[2] as CodeGroupBlock["kind"];
    const body: string[] = [];
    let cursor = index + 1;
    while (cursor < lines.length && lines[cursor].trim() !== marker) {
      body.push(lines[cursor]);
      cursor += 1;
    }

    if (cursor >= lines.length) {
      output.push(lines[index]);
      index += 1;
      continue;
    }

    const items =
      kind === "code-tabs"
        ? parseVuePressCodeTabs(body)
        : parseVitePressCodeGroup(body);
    const html = renderCodeGroup(items);
    if (html) {
      output.push("", html, "");
    } else {
      output.push(...lines.slice(index, cursor + 1));
    }
    index = cursor + 1;
  }

  return output.join("\n");
}

export function attachCodeGroupTabs(node: HTMLElement): () => void {
  const handleClick = (event: MouseEvent): void => {
    const target = event.target;
    if (!(target instanceof Element)) return;

    const tab = target.closest<HTMLButtonElement>(".markdown-code-tab");
    if (!tab) return;

    const group = tab.closest<HTMLElement>(".markdown-code-group");
    const index = tab.dataset.codeTab;
    if (!group || index === undefined) return;

    for (const button of group.querySelectorAll<HTMLButtonElement>(".markdown-code-tab")) {
      const active = button.dataset.codeTab === index;
      button.classList.toggle("active", active);
      button.setAttribute("aria-selected", String(active));
    }
    for (const panel of group.querySelectorAll<HTMLElement>(".markdown-code-panel")) {
      panel.classList.toggle("active", panel.dataset.codePanel === index);
    }
  };

  node.addEventListener("click", handleClick);
  return () => node.removeEventListener("click", handleClick);
}
