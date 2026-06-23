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
      if (accounts[0]) await loadTargets(accounts[0]);
    } catch (loadError) {
      error =
        loadError instanceof Error
          ? loadError.message
          : t("loadFailed", $preferencesStore.language);
    } finally {
      loading = false;
    }
  }

  async function loadTargets(account = selectedAccount): Promise<void> {
    targets = [];
    targetID = account?.default_target_id ?? "";
    if (!account) return;
    targetLoading = true;
    try {
      targets = await listNoteConnectionTargets(account.id);
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

  function selectAccount(event: Event): void {
    accountID = Number((event.currentTarget as HTMLSelectElement).value);
    const account = accounts.find((item) => item.id === accountID) ?? null;
    void loadTargets(account);
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
      <div class="connection-push-state">{t("loading", $preferencesStore.language)}</div>
    {:else if accounts.length === 0}
      <div class="connection-push-state">{t("noteConnectionNoAccounts", $preferencesStore.language)}</div>
    {:else}
      <div class="connection-push-form">
        <label>
          <span>{t("noteConnectionSelectAccount", $preferencesStore.language)}</span>
          <select value={accountID} on:change={selectAccount}>
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
        {#if targetLoading}
          <div class="connection-push-state compact">{t("loading", $preferencesStore.language)}</div>
        {/if}
      </div>
      {#if targets.length === 0 && !targetLoading}
        <div class="connection-push-state">{t("noteConnectionNoTargets", $preferencesStore.language)}</div>
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
        disabled={pushing || loading || targetLoading || !selectedAccount || targets.length === 0}
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
    backdrop-filter: blur(2px);
  }

  .connection-push-dialog {
    display: grid;
    gap: 14px;
    width: min(520px, 100%);
    max-height: min(680px, calc(100vh - 36px));
    overflow: auto;
    border: 1px solid var(--color-divider, #e2e0d8);
    border-radius: 8px;
    background: color-mix(in srgb, var(--color-card, #fffefa) 94%, var(--color-surface-secondary, #f4f3ee));
    color: var(--color-text, #1a1a18);
    padding: 18px;
    box-shadow: 0 18px 48px rgb(var(--color-shadow, 26 26 24) / 18%);
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

  .connection-push-dialog h3 {
    font-size: 17px;
    line-height: 1.25;
  }

  .connection-push-dialog header p {
    color: var(--color-text-muted, #8c8c84);
    margin-top: 4px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .connection-push-dialog header button {
    width: 34px;
    height: 34px;
    flex: 0 0 34px;
    display: inline-grid;
    place-items: center;
    border: 0;
    border-radius: 8px;
    background: var(--color-surface-secondary, #f4f3ee);
    color: var(--color-text-muted, #8c8c84);
    padding: 0;
  }

  .connection-push-dialog header button:hover {
    color: var(--color-brand, #e8531a);
  }

  .connection-push-form {
    display: grid;
    gap: 12px;
  }

  .connection-push-form label {
    display: grid;
    gap: 6px;
    color: var(--color-text-secondary, #3a3a34);
    font-size: 12px;
  }

  .connection-push-form select {
    width: 100%;
    min-height: 40px;
    border: 1px solid var(--color-divider, #e2e0d8);
    border-radius: 8px;
    background: var(--color-card, #fffefa);
    color: var(--color-text, #1a1a18);
    padding: 0 10px;
  }

  .connection-push-form select:disabled {
    color: var(--color-text-muted, #8c8c84);
    opacity: 0.72;
  }

  .connection-push-state {
    border: 1px solid var(--color-divider, #e2e0d8);
    border-radius: 8px;
    background: var(--color-surface-secondary, #f4f3ee);
    color: var(--color-text-muted, #8c8c84);
    padding: 12px;
    font-size: 13px;
    line-height: 1.45;
  }

  .connection-push-state.compact {
    padding: 9px 10px;
  }

  .connection-push-error {
    border: 1px solid color-mix(in srgb, var(--color-danger, #cc3333) 24%, var(--color-divider, #e2e0d8));
    border-radius: 8px;
    background: var(--sm-error-bg, #fceaea);
    color: var(--color-danger, #cc3333);
    padding: 10px 12px;
    font-size: 13px;
  }

  .connection-push-dialog footer {
    justify-content: flex-end;
  }

  .connection-push-dialog footer button {
    min-height: 38px;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 7px;
    border: 1px solid var(--color-divider, #e2e0d8);
    border-radius: 8px;
    background: var(--color-card, #fffefa);
    color: var(--color-text-secondary, #3a3a34);
    padding: 0 12px;
    font-size: 13px;
  }

  .connection-push-dialog footer button:hover:not(:disabled) {
    border-color: color-mix(in srgb, var(--color-brand, #e8531a) 28%, var(--color-divider, #e2e0d8));
    background: color-mix(in srgb, var(--color-brand, #e8531a) 8%, var(--color-card, #fffefa));
    color: var(--color-brand, #e8531a);
  }

  .connection-push-dialog footer button:disabled {
    cursor: default;
    opacity: 0.5;
  }

  .connection-push-dialog button.primary {
    border-color: var(--color-brand, #e8531a);
    background: var(--color-brand, #e8531a);
    color: var(--color-on-brand, #fff8f2);
  }

  .connection-push-dialog button.primary:hover:not(:disabled) {
    border-color: var(--color-brand-hover, #c03b0d);
    background: var(--color-brand-hover, #c03b0d);
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
