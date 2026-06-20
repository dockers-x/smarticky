<script lang="ts">
  import { onMount } from "svelte";
  import Workspace from "./lib/components/workspace/Workspace.svelte";
  import { authStore } from "./lib/stores/auth";

  onMount(() => {
    authStore.hydrate();
  });
</script>

{#if $authStore.loading}
  <main class="app-shell" aria-label="Smarticky workspace">
    <section class="boot-panel">
      <p class="boot-kicker">SMARTICKY</p>
      <h1>Smarticky</h1>
      <p>正在准备笔记工作台</p>
    </section>
  </main>
{:else if $authStore.user}
  <Workspace />
{/if}
