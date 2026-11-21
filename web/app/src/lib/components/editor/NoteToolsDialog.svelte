<script lang="ts">
  import { onMount } from "svelte";
  import { Check, Copy, Play, X } from "@lucide/svelte";
  import { dialog } from "../../actions/dialog";
  import { apiFetch } from "../../api/client";
  import { preferencesStore, t } from "../../stores/preferences";

  export let noteID: string;
  export let onClose: () => void;
  let tools: { id: string }[] = [];
  let selected = "";
  let loading = true;
  let running = false;
  let error = "";
  let output: string | null = null;
  let copied = false;
  let resultField: HTMLTextAreaElement;
  const controller = new AbortController();
  $: zh = $preferencesStore.language === "zh";

  async function listTools() {
    loading = true;
    error = "";
    try {
      tools = await apiFetch("/note-tools", { signal: controller.signal });
      selected = tools[0]?.id ?? "";
    } catch {
      error = zh ? "工具列表加载失败。" : "Could not load tools.";
    } finally { loading = false; }
  }

  async function runTool() {
    if (running || !selected) return;
    running = true;
    error = "";
    output = null;
    copied = false;
    try {
      const result = await apiFetch<{ text: string }>(`/notes/${noteID}/tools/${encodeURIComponent(selected)}`, {
        method: "POST", signal: controller.signal,
      });
      output = result.text;
    } catch {
      error = zh ? "工具未能完成，可以重试。" : "The tool could not finish. Try again.";
    } finally { running = false; }
  }

  async function copyResult() {
    if (!output) return;
    try { await navigator.clipboard.writeText(output); copied = true; }
    catch { resultField.focus(); resultField.select(); }
  }

  onMount(() => {
    void listTools();
    return () => controller.abort();
  });
</script>

<div class="dialog-backdrop" role="presentation">
  <div class="confirm-dialog note-tools" role="dialog" aria-modal="true" aria-labelledby="note-tools-title"
    tabindex="-1" use:dialog={{ onDismiss: onClose, initialFocus: "select" }}>
    <header>
      <h2 id="note-tools-title">{t("noteTools", $preferencesStore.language)}</h2>
      <button class="tools-panel__close" type="button" aria-label={zh ? "关闭" : "Close"} on:click={onClose}><X size={18} aria-hidden="true" /></button>
    </header>
    <label for="note-tool-picker">{zh ? "选择工具" : "Choose a tool"}</label>
    <div class="tool-controls">
      <select id="note-tool-picker" bind:value={selected} disabled={loading || running || !tools.length}
        on:change={() => { output = null; copied = false; error = ""; }}>
        {#each tools as tool}
          <option value={tool.id}>{tool.id === "open-tasks" ? (zh ? "提取未完成待办" : "Extract open tasks") : tool.id}</option>
        {/each}
      </select>
      <button class="run-tool" type="button" disabled={loading || running || !selected} on:click={runTool}>
        <Play size={15} aria-hidden="true" />{running ? (zh ? "运行中…" : "Running…") : (zh ? "运行" : "Run")}
      </button>
    </div>
    {#if loading}<p role="status">{t("loading", $preferencesStore.language)}</p>{/if}
    {#if error}
      <p role="alert">{error}</p>
      {#if !tools.length}<button type="button" on:click={listTools}>{zh ? "重试" : "Retry"}</button>{/if}
    {/if}
    {#if output !== null}
      {#if output}
        <div class="result-label">
          <label for="note-tool-output">{zh ? "结果" : "Result"}</label>
          <button type="button" on:click={copyResult}>
            {#if copied}<Check size={15} aria-hidden="true" />{:else}<Copy size={15} aria-hidden="true" />{/if}
            {copied ? (zh ? "已复制" : "Copied") : (zh ? "复制结果" : "Copy result")}
          </button>
        </div>
        <textarea id="note-tool-output" bind:this={resultField} readonly rows="6" value={output}></textarea>
      {:else}
        <p role="status">{selected === "open-tasks" ? (zh ? "这篇笔记没有未完成的待办。" : "This note has no open tasks.") : (zh ? "工具返回了空结果。" : "The tool returned an empty result.")}</p>
      {/if}
    {/if}
  </div>
</div>

<style>
  .note-tools { width: min(30rem, 100%); display: grid; gap: 12px; }
  header, .result-label { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  h2, p { margin: 0; }
  label { font-size: 0.8125rem; font-weight: 600; }
  .tool-controls { display: grid; grid-template-columns: minmax(0,1fr) auto; gap: 8px; }
  select, textarea { width: 100%; min-width: 0; padding: 10px; border: 1px solid var(--color-divider-em); border-radius: var(--radius-control); color: var(--color-text); background: var(--color-card); }
  textarea { resize: vertical; line-height: 1.6; }
  button { display: inline-flex; align-items: center; justify-content: center; gap: 6px; min-height: 44px; border: 0; border-radius: var(--radius-control); padding-inline: 12px; color: var(--color-text); background: var(--color-surface-secondary); }
  .run-tool { background: var(--color-brand-strong); color: var(--color-on-brand); }
  button:disabled { opacity: .6; }
  .result-label { margin-top: 8px; }
  [role="alert"] { color: var(--color-danger); }
</style>
