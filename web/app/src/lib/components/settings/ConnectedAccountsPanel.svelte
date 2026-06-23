<script lang="ts">
  import { Cloud, Download, Pencil, PlugZap, Plus, Trash2 } from "@lucide/svelte";
  import { onMount } from "svelte";
  import {
    createNoteConnectionAccount,
    deleteNoteConnectionAccount,
    importFromNoteConnection,
    listNoteConnectionAccounts,
    listNoteConnectionJobs,
    listNoteConnectionTargets,
    testNoteConnectionAccount,
    testUnsavedNoteConnectionAccount,
    updateNoteConnectionAccount,
    type NoteConnectionAccount,
    type NoteConnectionAccountInput,
    type NoteConnectionJob,
    type NoteConnectionProvider,
    type NoteConnectionTarget,
  } from "../../api/noteConnections";
  import { confirmDialog, notify } from "../../stores/dialogs";
  import { foldersStore } from "../../stores/folders";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t, type MessageKey } from "../../stores/preferences";
  import PasswordField from "../common/PasswordField.svelte";

  const providers: NoteConnectionProvider[] = ["siyuan", "notion", "joplin"];
  const providerLabelKeys: Record<NoteConnectionProvider, MessageKey> = {
    siyuan: "siyuan",
    notion: "notion",
    joplin: "joplin",
  };

  const endpointPlaceholders: Record<NoteConnectionProvider, string> = {
    siyuan: "http://127.0.0.1:6806",
    notion: "",
    joplin: "http://127.0.0.1:41184",
  };

  const defaultForm: NoteConnectionAccountInput = {
    name: "",
    provider: "siyuan",
    endpoint: endpointPlaceholders.siyuan,
    token: "",
    default_target_id: "",
    default_target_name: "",
    enabled: true,
  };

  let accounts: NoteConnectionAccount[] = [];
  let jobs: NoteConnectionJob[] = [];
  let targets: NoteConnectionTarget[] = [];
  let loading = true;
  let working = false;
  let saving = false;
  let error = "";

  let formOpen = false;
  let editingID: number | null = null;
  let form: NoteConnectionAccountInput = { ...defaultForm };
  let testMessage = "";
  let testOK: boolean | null = null;

  let importAccountID: number | null = null;
  let importTargetID = "";
  let importLimit = 50;
  let importMessage = "";

  $: selectedImportAccount = accounts.find((account) => account.id === importAccountID) ?? null;
  $: formProvider = form.provider;

  onMount(() => {
    void loadState();
  });

  async function loadState(): Promise<void> {
    loading = true;
    error = "";
    try {
      const [nextAccounts, nextJobs] = await Promise.all([
        listNoteConnectionAccounts(),
        listNoteConnectionJobs(),
      ]);
      accounts = nextAccounts;
      jobs = nextJobs;
      if (importAccountID && !accounts.some((account) => account.id === importAccountID)) {
        resetImport();
      }
    } catch (loadError) {
      error =
        loadError instanceof Error
          ? loadError.message
          : t("loadFailed", $preferencesStore.language);
    } finally {
      loading = false;
    }
  }

  function providerLabel(provider: NoteConnectionProvider): string {
    return t(providerLabelKeys[provider], $preferencesStore.language);
  }

  function providerFromEvent(event: Event): NoteConnectionProvider {
    return (event.currentTarget as HTMLSelectElement).value as NoteConnectionProvider;
  }

  function startCreate(provider: NoteConnectionProvider = "siyuan"): void {
    editingID = null;
    form = {
      ...defaultForm,
      provider,
      endpoint: endpointPlaceholders[provider],
      name: providerLabel(provider),
    };
    testMessage = "";
    testOK = null;
    formOpen = true;
  }

  function startEdit(account: NoteConnectionAccount): void {
    editingID = account.id;
    form = {
      name: account.name,
      provider: account.provider,
      endpoint: account.endpoint || endpointPlaceholders[account.provider],
      token: "",
      default_target_id: account.default_target_id,
      default_target_name: account.default_target_name,
      enabled: account.enabled,
    };
    testMessage = "";
    testOK = null;
    formOpen = true;
  }

  function closeForm(): void {
    formOpen = false;
    editingID = null;
    testMessage = "";
    testOK = null;
  }

  function formPayload(): NoteConnectionAccountInput {
    const payload: NoteConnectionAccountInput = {
      ...form,
      name: form.name.trim(),
      endpoint: form.provider === "notion" ? "" : form.endpoint.trim(),
      default_target_id: form.default_target_id.trim(),
      default_target_name: form.default_target_name.trim(),
      enabled: Boolean(form.enabled),
    };
    if (!payload.token?.trim()) {
      delete payload.token;
    } else {
      payload.token = payload.token.trim();
    }
    return payload;
  }

  async function testForm(): Promise<void> {
    working = true;
    testMessage = "";
    testOK = null;
    try {
      const payload = formPayload();
      const result =
        editingID === null
          ? await testUnsavedNoteConnectionAccount(payload)
          : await testNoteConnectionAccount(editingID, payload.token);
      testOK = result.status === "success";
      testMessage = testOK
        ? t("connectionTestSuccess", $preferencesStore.language)
        : result.error || t("connectionTestFailed", $preferencesStore.language);
      if (editingID !== null) await loadState();
    } catch (testError) {
      testOK = false;
      testMessage =
        testError instanceof Error
          ? testError.message
          : t("connectionTestFailed", $preferencesStore.language);
    } finally {
      working = false;
    }
  }

  async function saveAccount(): Promise<void> {
    saving = true;
    try {
      const payload = formPayload();
      if (editingID === null) {
        await createNoteConnectionAccount(payload);
      } else {
        await updateNoteConnectionAccount(editingID, payload);
      }
      notify(t("saved", $preferencesStore.language), "success");
      closeForm();
      await loadState();
    } catch (saveError) {
      notify(
        saveError instanceof Error
          ? saveError.message
          : t("saveFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      saving = false;
    }
  }

  async function removeAccount(account: NoteConnectionAccount): Promise<void> {
    const confirmed = await confirmDialog({
      title: t("delete", $preferencesStore.language),
      message: t("noteConnectionDeleteConfirm", $preferencesStore.language),
      confirmLabel: t("delete", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (!confirmed) return;

    try {
      await deleteNoteConnectionAccount(account.id);
      if (importAccountID === account.id) resetImport();
      await loadState();
    } catch (deleteError) {
      notify(
        deleteError instanceof Error
          ? deleteError.message
          : t("deleteFailed", $preferencesStore.language),
        "error",
      );
    }
  }

  async function startImport(account: NoteConnectionAccount): Promise<void> {
    importAccountID = account.id;
    importTargetID = account.default_target_id;
    importMessage = "";
    targets = [];
    working = true;
    try {
      targets = await listNoteConnectionTargets(account.id);
      if (!importTargetID && targets.length > 0) {
        importTargetID = targets[0].id;
      }
    } catch (targetError) {
      notify(
        targetError instanceof Error
          ? targetError.message
          : t("loadFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      working = false;
    }
  }

  function resetImport(): void {
    importAccountID = null;
    importTargetID = "";
    targets = [];
    importMessage = "";
  }

  async function runImport(): Promise<void> {
    if (!selectedImportAccount) return;
    working = true;
    importMessage = "";
    try {
      const result = await importFromNoteConnection(
        selectedImportAccount.id,
        importTargetID,
        importLimit,
      );
      importMessage = `${t("noteConnectionImportSuccess", $preferencesStore.language)}: ${result.imported_count}/${result.total_count}`;
      notify(t("noteConnectionImportSuccess", $preferencesStore.language), "success");
      await Promise.all([notesStore.load(), foldersStore.load(), loadState()]);
    } catch (importError) {
      notify(
        importError instanceof Error
          ? importError.message
          : t("importFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      working = false;
    }
  }

  function formatTime(value?: string): string {
    if (!value) return t("backupNever", $preferencesStore.language);
    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) return value;
    return parsed.toLocaleString($preferencesStore.language === "zh" ? "zh-CN" : "en-US", {
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      timeZone: $preferencesStore.timeZone,
    });
  }
</script>

<section class="connected-panel">
  <div class="connected-panel__header">
    <div>
      <h3>{t("noteConnections", $preferencesStore.language)}</h3>
      <p>{t("noteConnectionsHint", $preferencesStore.language)}</p>
    </div>
    <button type="button" on:click={() => startCreate()}>
      <Plus size={16} strokeWidth={2} aria-hidden="true" />
      {t("noteConnectionAdd", $preferencesStore.language)}
    </button>
  </div>

  {#if error}
    <p class="connected-panel__error" role="alert">{error}</p>
  {/if}

  {#if loading}
    <p class="connected-panel__muted">{t("loading", $preferencesStore.language)}</p>
  {:else if accounts.length === 0}
    <div class="connected-empty">
      <Cloud size={24} strokeWidth={1.8} aria-hidden="true" />
      <p>{t("noteConnectionNoAccounts", $preferencesStore.language)}</p>
      <div class="connected-provider-row">
        {#each providers as provider}
          <button type="button" on:click={() => startCreate(provider)}>
            {providerLabel(provider)}
          </button>
        {/each}
      </div>
    </div>
  {:else}
    <div class="connected-account-list">
      {#each accounts as account (account.id)}
        <article class="connected-account">
          <div class="connected-account__main">
            <span class:failed={account.last_test_status === "failed"} class:success={account.last_test_status === "success"} class="connected-account__status"></span>
            <div>
              <h4>{account.name}</h4>
              <p>
                {providerLabel(account.provider)}
                {#if account.endpoint}
                  · {account.endpoint}
                {/if}
              </p>
              <small>
                {account.has_credentials
                  ? t("noteConnectionSavedCredential", $preferencesStore.language)
                  : t("noteProtectionPasswordRequired", $preferencesStore.language)}
                · {formatTime(account.last_test_at)}
              </small>
            </div>
          </div>
          <div class="connected-account__actions">
            <button type="button" disabled={working} on:click={() => void testNoteConnectionAccount(account.id).then(loadState)}>
              <PlugZap size={15} strokeWidth={2} aria-hidden="true" />
              {t("testConnection", $preferencesStore.language)}
            </button>
            <button type="button" disabled={working || !account.enabled || !account.has_credentials} on:click={() => void startImport(account)}>
              <Download size={15} strokeWidth={2} aria-hidden="true" />
              {t("noteConnectionImport", $preferencesStore.language)}
            </button>
            <button type="button" on:click={() => startEdit(account)}>
              <Pencil size={15} strokeWidth={2} aria-hidden="true" />
              {t("edit", $preferencesStore.language)}
            </button>
            <button class="danger" type="button" on:click={() => void removeAccount(account)}>
              <Trash2 size={15} strokeWidth={2} aria-hidden="true" />
              {t("delete", $preferencesStore.language)}
            </button>
          </div>
        </article>
      {/each}
    </div>
  {/if}

  {#if formOpen}
    <form class="connected-form" on:submit|preventDefault={() => void saveAccount()}>
      <div class="connected-form__header">
        <h4>
          {editingID === null
            ? t("noteConnectionAdd", $preferencesStore.language)
            : t("noteConnectionEdit", $preferencesStore.language)}
        </h4>
        <button type="button" on:click={closeForm}>{t("cancel", $preferencesStore.language)}</button>
      </div>

      <div class="connected-form__grid">
        <label>
          <span>{t("noteConnectionProvider", $preferencesStore.language)}</span>
          <select
            value={form.provider}
            disabled={editingID !== null}
            on:change={(event) => {
              const provider = providerFromEvent(event);
              form.provider = provider;
              form.endpoint = endpointPlaceholders[provider];
              if (!form.name.trim()) form.name = providerLabel(provider);
            }}
          >
            {#each providers as provider}
              <option value={provider}>{providerLabel(provider)}</option>
            {/each}
          </select>
        </label>
        <label>
          <span>{t("mcpTokenName", $preferencesStore.language)}</span>
          <input bind:value={form.name} required />
        </label>
        {#if formProvider !== "notion"}
          <label class="wide">
            <span>{t("noteConnectionEndpoint", $preferencesStore.language)}</span>
            <input bind:value={form.endpoint} placeholder={endpointPlaceholders[formProvider]} />
            <small>{t("noteConnectionEndpointHint", $preferencesStore.language)}</small>
          </label>
        {/if}
        <div class="wide">
          <PasswordField
            bind:value={form.token}
            label={t("noteConnectionApiToken", $preferencesStore.language)}
            placeholder={editingID === null
              ? t("noteConnectionTokenPlaceholder", $preferencesStore.language)
              : t("noteConnectionSavedCredential", $preferencesStore.language)}
            autocomplete="off"
            showPasswordLabel={t("showPassword", $preferencesStore.language)}
            hidePasswordLabel={t("hidePassword", $preferencesStore.language)}
          />
        </div>
      </div>

      {#if testMessage}
        <p class:success={testOK === true} class:failed={testOK === false} class="connected-test-message">
          {testMessage}
        </p>
      {/if}

      <div class="connected-form__actions">
        <button type="button" disabled={working} on:click={() => void testForm()}>
          {t("testConnection", $preferencesStore.language)}
        </button>
        <button class="primary" type="submit" disabled={saving}>
          {t("saveSettings", $preferencesStore.language)}
        </button>
      </div>
    </form>
  {/if}

  {#if selectedImportAccount}
    <section class="connected-import">
      <div class="connected-form__header">
        <div>
          <h4>{t("noteConnectionImport", $preferencesStore.language)}</h4>
          <p>{selectedImportAccount.name}</p>
        </div>
        <button type="button" on:click={resetImport}>{t("cancel", $preferencesStore.language)}</button>
      </div>
      <div class="connected-form__grid">
        <label>
          <span>{t("noteConnectionImportTarget", $preferencesStore.language)}</span>
          <select bind:value={importTargetID} disabled={targets.length === 0}>
            {#each targets as target (target.id)}
              <option value={target.id}>{target.name || target.id}</option>
            {/each}
          </select>
        </label>
        <label>
          <span>{t("noteConnectionImportLimit", $preferencesStore.language)}</span>
          <input bind:value={importLimit} min="1" max="200" type="number" />
        </label>
      </div>
      {#if targets.length === 0}
        <p class="connected-panel__muted">{t("noteConnectionNoTargets", $preferencesStore.language)}</p>
      {/if}
      {#if importMessage}
        <p class="connected-test-message success">{importMessage}</p>
      {/if}
      <div class="connected-form__actions">
        <button class="primary" type="button" disabled={working || targets.length === 0} on:click={() => void runImport()}>
          {t("importStart", $preferencesStore.language)}
        </button>
      </div>
    </section>
  {/if}

  <section class="connected-jobs">
    <h4>{t("noteConnectionJobs", $preferencesStore.language)}</h4>
    {#if jobs.length === 0}
      <p class="connected-panel__muted">{t("noteConnectionNoJobs", $preferencesStore.language)}</p>
    {:else}
      <div class="connected-job-list">
        {#each jobs as job (job.id)}
          <div class="connected-job">
            <span>{providerLabel(job.provider)} · {job.operation}</span>
            <strong>{job.status}</strong>
            <small>
              {job.imported_count + job.pushed_count}/{job.total_count}
              · {formatTime(job.completed_at || job.created_at)}
            </small>
          </div>
        {/each}
      </div>
    {/if}
  </section>
</section>

<style>
  .connected-panel {
    display: grid;
    gap: 18px;
  }

  .connected-panel__header,
  .connected-form__header,
  .connected-form__actions,
  .connected-account,
  .connected-account__main,
  .connected-account__actions,
  .connected-provider-row {
    display: flex;
    align-items: center;
  }

  .connected-panel__header,
  .connected-form__header,
  .connected-account {
    justify-content: space-between;
    gap: 14px;
  }

  .connected-panel__header p,
  .connected-form__header p,
  .connected-account p,
  .connected-account small,
  .connected-panel__muted {
    color: var(--color-text-muted, #8c8c84);
  }

  .connected-panel__header h3,
  .connected-form__header h4,
  .connected-jobs h4,
  .connected-account h4 {
    margin: 0;
  }

  .connected-panel__header p,
  .connected-form__header p,
  .connected-account p,
  .connected-account small {
    margin: 4px 0 0;
  }

  .connected-account-list,
  .connected-job-list {
    display: grid;
    gap: 10px;
  }

  .connected-account,
  .connected-form,
  .connected-import,
  .connected-empty,
  .connected-jobs {
    border: 1px solid var(--color-divider, #e2e0d8);
    border-radius: 8px;
    padding: 14px;
    background: var(--color-card, #fffefa);
  }

  .connected-account__main {
    gap: 10px;
    min-width: 0;
  }

  .connected-account__main > div {
    min-width: 0;
  }

  .connected-account__main p {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .connected-account__actions,
  .connected-form__actions,
  .connected-provider-row {
    gap: 8px;
    flex-wrap: wrap;
  }

  .connected-account__status {
    width: 9px;
    height: 9px;
    border-radius: 999px;
    background: var(--color-text-muted, #8c8c84);
    flex: 0 0 auto;
  }

  .connected-account__status.success,
  .connected-test-message.success {
    color: var(--sm-success, #2d7a4f);
  }

  .connected-account__status.success {
    background: var(--sm-success, #2d7a4f);
  }

  .connected-account__status.failed {
    background: var(--color-danger, #cc3333);
  }

  .connected-test-message.failed,
  .connected-panel__error {
    color: var(--color-danger, #cc3333);
  }

  .connected-form,
  .connected-import {
    display: grid;
    gap: 14px;
  }

  .connected-form__grid {
    display: grid;
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 12px;
  }

  .connected-form__grid label,
  .connected-form__grid .wide {
    display: grid;
    gap: 6px;
  }

  .connected-form__grid .wide {
    grid-column: 1 / -1;
  }

  .connected-form__grid input,
  .connected-form__grid select {
    width: 100%;
  }

  .connected-empty {
    display: grid;
    justify-items: start;
    gap: 10px;
  }

  .connected-job {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 4px 10px;
    border-top: 1px solid var(--color-divider, #e2e0d8);
    padding-top: 10px;
  }

  .connected-job small {
    color: var(--color-text-muted, #8c8c84);
    grid-column: 1 / -1;
  }

  button.danger {
    color: var(--color-danger, #cc3333);
  }

  button.primary {
    background: var(--color-brand, #e8531a);
    color: var(--color-on-brand, #fff8f2);
  }

  @media (max-width: 720px) {
    .connected-panel__header,
    .connected-account {
      align-items: stretch;
      flex-direction: column;
    }

    .connected-account__actions {
      align-items: stretch;
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }

    .connected-form__grid {
      grid-template-columns: 1fr;
    }
  }
</style>
