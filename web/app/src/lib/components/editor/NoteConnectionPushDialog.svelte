<script lang="ts">
  import { CloudUpload, X } from "@lucide/svelte";
  import { onMount } from "svelte";
  import type { Note } from "../../api/types";
  import {
    listNoteConnectionAccounts,
    listNoteConnectionTargets,
    pushNoteToConnection,
    type NoteConnectionAccount,
    type NoteConnectionProvider,
    type NoteConnectionTarget,
  } from "../../api/noteConnections";
  import { notify } from "../../stores/dialogs";
  import { preferencesStore, t, type MessageKey } from "../../stores/preferences";

  export let note: Note;
  export let onClose: () => void = () => {};

  const providerLabelKeys: Record<NoteConnectionProvider, MessageKey> = {
    siyuan: "siyuan",
    notion: "notion",
    joplin: "joplin",
  };

  let accounts: NoteConnectionAccount[] = [];
  let targets: NoteConnectionTarget[] = [];
  let accountID = 0;
  let targetID = "";
  let loading = true;
  let targetLoading = false;
  let pushing = false;
  let error = "";

  $: selectedAccount = accounts.find((account) => account.id === accountID) ?? null;

  onMount(() => {
    void loadAccounts();
  });

  async function loadAccounts(): Promise<void> {
    loading = true;
    error = "";
    try {
      accounts = (await listNoteConnectionAccounts()).filter(
        (account) => account.enabled && account.has_credentials,
      );
      accountID = accounts[0]?.id ?? 0;
      if (accountID) await loadTargets();
    } catch (loadError) {
      error =
        loadError instanceof Error
          ? loadError.message
          : t("loadFailed", $preferencesStore.language);
    } finally {
      loading = false;
    }
  }

  async function loadTargets(): Promise<void> {
    targets = [];
    targetID = selectedAccount?.default_target_id ?? "";
    if (!selectedAccount) return;
    targetLoading = true;
    try {
      targets = await listNoteConnectionTargets(selectedAccount.id);
      if (!targetID && targets.length > 0) targetID = targets[0].id;
    } catch (targetError) {
      error =
        targetError instanceof Error
          ? targetError.message
          : t("loadFailed", $preferencesStore.language);
    } finally {
      targetLoading = false;
    }
  }

  function providerLabel(provider: NoteConnectionProvider): string {
    return t(providerLabelKeys[provider], $preferencesStore.language);
  }

  async function pushNote(): Promise<void> {
    if (!selectedAccount || !note) return;
    pushing = true;
    error = "";
    try {
      await pushNoteToConnection(selectedAccount.id, note.id, targetID);
      notify(t("noteConnectionPushSuccess", $preferencesStore.language), "success");
      onClose();
    } catch (pushError) {
      error =
        pushError instanceof Error
          ? pushError.message
          : t("saveFailed", $preferencesStore.language);
    } finally {
      pushing = false;
    }
  }
</script>

<div class="connection-push-backdrop" role="presentation" on:click={(event) => {
  if (event.currentTarget === event.target) onClose();
}}>
  <div
    class="connection-push-dialog"
    role="dialog"
    aria-modal="true"
    aria-labelledby="connection-push-title"
  >
    <header>
      <div>
        <h3 id="connection-push-title">{t("noteConnectionPush", $preferencesStore.language)}</h3>
        <p>{note.title || t("untitled", $preferencesStore.language)}</p>
      </div>
      <button type="button" aria-label={t("cancel", $preferencesStore.language)} on:click={onClose}>
        <X size={18} strokeWidth={2} aria-hidden="true" />
      </button>
    </header>

    {#if loading}
      <p class="connection-push-muted">{t("loading", $preferencesStore.language)}</p>
    {:else if accounts.length === 0}
      <p class="connection-push-muted">{t("noteConnectionNoAccounts", $preferencesStore.language)}</p>
    {:else}
      <div class="connection-push-form">
        <label>
          <span>{t("noteConnectionSelectAccount", $preferencesStore.language)}</span>
          <select bind:value={accountID} on:change={() => void loadTargets()}>
            {#each accounts as account (account.id)}
              <option value={account.id}>{account.name} · {providerLabel(account.provider)}</option>
            {/each}
          </select>
        </label>
        <label>
          <span>{t("noteConnectionSelectTarget", $preferencesStore.language)}</span>
          <select bind:value={targetID} disabled={targetLoading || targets.length === 0}>
            {#each targets as target (target.id)}
              <option value={target.id}>{target.name || target.id}</option>
            {/each}
          </select>
        </label>
      </div>
      {#if targets.length === 0 && !targetLoading}
        <p class="connection-push-muted">{t("noteConnectionNoTargets", $preferencesStore.language)}</p>
      {/if}
    {/if}

    {#if error}
      <p class="connection-push-error" role="alert">{error}</p>
    {/if}

    <footer>
      <button type="button" on:click={onClose}>{t("cancel", $preferencesStore.language)}</button>
      <button
        class="primary"
        type="button"
        disabled={pushing || loading || !selectedAccount || targets.length === 0}
        on:click={() => void pushNote()}
      >
        <CloudUpload size={16} strokeWidth={2} aria-hidden="true" />
        {t("noteConnectionPush", $preferencesStore.language)}
      </button>
    </footer>
  </div>
</div>

<style>
  .connection-push-backdrop {
    position: fixed;
    inset: 0;
    z-index: 80;
    display: grid;
    place-items: center;
    padding: 18px;
    background: var(--color-backdrop, rgb(26 26 24 / 28%));
  }

  .connection-push-dialog {
    display: grid;
    gap: 16px;
    width: min(520px, 100%);
    max-height: min(680px, calc(100vh - 36px));
    overflow: auto;
    border: 1px solid var(--color-divider, #e2e0d8);
    border-radius: 8px;
    background: var(--color-card, #fffefa);
    color: var(--color-text, #1a1a18);
    padding: 18px;
  }

  .connection-push-dialog header,
  .connection-push-dialog footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
  }

  .connection-push-dialog h3,
  .connection-push-dialog p {
    margin: 0;
  }

  .connection-push-dialog header p,
  .connection-push-muted {
    color: var(--color-text-muted, #8c8c84);
  }

  .connection-push-form {
    display: grid;
    gap: 12px;
  }

  .connection-push-form label {
    display: grid;
    gap: 6px;
  }

  .connection-push-form select {
    width: 100%;
  }

  .connection-push-error {
    color: var(--color-danger, #cc3333);
  }

  .connection-push-dialog footer {
    justify-content: flex-end;
  }

  .connection-push-dialog button.primary {
    display: inline-flex;
    align-items: center;
    gap: 6px;
    background: var(--color-brand, #e8531a);
    color: var(--color-on-brand, #fff8f2);
  }

  @media (max-width: 640px) {
    .connection-push-backdrop {
      align-items: end;
      padding: 0;
    }

    .connection-push-dialog {
      width: 100%;
      max-height: 88vh;
      border-radius: 8px 8px 0 0;
    }
  }
</style>
