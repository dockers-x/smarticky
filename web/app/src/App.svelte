<script lang="ts">
  import { onMount } from "svelte";
  import DialogHost from "./lib/components/common/DialogHost.svelte";
  import Workspace from "./lib/components/workspace/Workspace.svelte";
  import { authStore } from "./lib/stores/auth";
  import { preferencesStore, t } from "./lib/stores/preferences";

  onMount(() => {
    preferencesStore.hydrate();
    authStore.hydrate();
  });
</script>

{#if $authStore.loading}
  <main class="app-shell" aria-label="Smarticky workspace">
    <section class="boot-panel">
      <p class="boot-kicker">SMARTICKY</p>
      <h1>Smarticky</h1>
      <p>{t("preparing", $preferencesStore.language)}</p>
    </section>
  </main>
{:else if $authStore.user}
  <Workspace />
{/if}

<DialogHost />
