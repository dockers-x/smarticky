<script lang="ts">
  import { diagramRuntime } from "../../markdown/diagrams/runtime";
  import type { DiagramRuntimeState, DiagramTheme } from "../../markdown/diagrams/types";

  export let html = "";
  export let theme: DiagramTheme = "light";
  export let className = "";
  export let onDiagramState: (state: DiagramRuntimeState) => void = () => {};

  $: runtimeOptions = {
    theme,
    contentKey: html,
    onStateChange: onDiagramState,
  };
</script>

<div class={className} use:diagramRuntime={runtimeOptions}>
  {@html html}
</div>
