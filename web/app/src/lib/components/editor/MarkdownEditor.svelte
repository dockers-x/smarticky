<script lang="ts">
  import { defaultKeymap, history, historyKeymap } from "@codemirror/commands";
  import { markdown } from "@codemirror/lang-markdown";
  import { EditorState } from "@codemirror/state";
  import { EditorView, keymap } from "@codemirror/view";
  import { onDestroy, onMount } from "svelte";

  export let value = "";
  export let onChange: (value: string) => void = () => {};
  export let bindView: (view: EditorView) => void = () => {};

  let host: HTMLDivElement;
  let view: EditorView | null = null;
  let applyingExternalValue = false;

  onMount(() => {
    view = new EditorView({
      parent: host,
      state: EditorState.create({
        doc: value,
        extensions: [
          history(),
          markdown(),
          keymap.of([...defaultKeymap, ...historyKeymap]),
          EditorView.updateListener.of((update) => {
            if (update.docChanged && !applyingExternalValue) {
              onChange(update.state.doc.toString());
            }
          }),
          EditorView.lineWrapping,
        ],
      }),
    });
    bindView(view);
  });

  $: if (view && value !== view.state.doc.toString()) {
    applyingExternalValue = true;
    view.dispatch({
      changes: { from: 0, to: view.state.doc.length, insert: value },
    });
    applyingExternalValue = false;
  }

  onDestroy(() => {
    view?.destroy();
  });
</script>

<div class="markdown-editor-host" bind:this={host}></div>
