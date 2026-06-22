<script lang="ts">
  import { diagramRuntime } from "../../markdown/diagrams/runtime";
  import type { DiagramRuntimeState, DiagramTheme } from "../../markdown/diagrams/types";
  import { attachCodeGroupTabs } from "../../markdown/codeGroups";
  import { protectedImageRuntime } from "../../markdown/protectedImages";
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
  $: protectedImageRuntimeOptions = {
    contentKey: html,
  };

  function codeGroupTabs(node: HTMLElement): { destroy: () => void } {
    const detach = attachCodeGroupTabs(node);
    return {
      destroy() {
        detach();
      },
    };
  }
</script>

<div
  class={className}
  use:diagramRuntime={runtimeOptions}
  use:whiteboardRuntime={whiteboardRuntimeOptions}
  use:codeGroupTabs
  use:protectedImageRuntime={protectedImageRuntimeOptions}
>
  {@html html}
</div>
