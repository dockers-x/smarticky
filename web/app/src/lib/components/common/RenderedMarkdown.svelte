<script lang="ts">
  import { diagramRuntime } from "../../markdown/diagrams/runtime";
  import type { DiagramRuntimeState, DiagramTheme } from "../../markdown/diagrams/types";
  import { whiteboardRuntime } from "../../markdown/whiteboards";

  export let html = "";
  export let theme: DiagramTheme = "light";
  export let className = "";
  export let onDiagramState: (state: DiagramRuntimeState) => void = () => {};
  export let onOpenWhiteboard: (whiteboardID: string) => void = () => {};

  $: runtimeOptions = {
    theme,
    contentKey: html,
    onStateChange: onDiagramState,
  };

  $: whiteboardRuntimeOptions = {
    contentKey: html,
    onOpen: onOpenWhiteboard,
  };
</script>

<div
  class={className}
  use:diagramRuntime={runtimeOptions}
  use:whiteboardRuntime={whiteboardRuntimeOptions}
>
  {@html html}
</div>
