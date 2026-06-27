<script lang="ts">
  import { Cloud, Download, Pencil, PlugZap, Plus, Trash2 } from "@lucide/svelte";
  import { onDestroy, onMount } from "svelte";
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
  let importPreserveRemoteHierarchy = true;
  let importMessage = "";
  let importing = false;
  let importProgressJob: NoteConnectionJob | null = null;
  let importPollTimer: ReturnType<typeof setInterval> | null = null;

  $: selectedImportAccount = accounts.find((account) => account.id === importAccountID) ?? null;
  $: formProvider = form.provider;
  $: visibleImportJob =
    importProgressJob ??
    jobs.find(
      (job) =>
        job.operation === "import" &&
        job.status === "running" &&
        job.account_id === importAccountID,
    ) ??
    null;
  $: importProcessedCount = visibleImportJob
    ? jobCount(visibleImportJob, "imported_count") +
      jobCount(visibleImportJob, "skipped_count") +
      jobCount(visibleImportJob, "failed_count")
    : 0;
  $: importProgressTotal = visibleImportJob ? jobCount(visibleImportJob, "total_count") : 0;
  $: importProgressPercent =
    importProgressTotal > 0
      ? Math.min(100, Math.round((importProcessedCount / importProgressTotal) * 100))
      : 0;

  onMount(() => {
    void loadState();
  });

  onDestroy(() => {
    stopImportPolling();
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

  function accountStatusLabel(account: NoteConnectionAccount): string {
    if (!account.enabled) return t("disabled", $preferencesStore.language);
    if (account.last_test_status === "success") return t("success", $preferencesStore.language);
    if (account.last_test_status === "failed") return t("failed", $preferencesStore.language);
    return t("backupNever", $preferencesStore.language);
  }

  function jobStatusLabel(status: NoteConnectionJob["status"]): string {
    switch (status) {
      case "pending":
        return t("preparing", $preferencesStore.language);
      case "running":
        return t("importing", $preferencesStore.language);
      case "completed":
        return t("done", $preferencesStore.language);
      case "completed_with_errors":
        return t("importCompletedPartial", $preferencesStore.language);
      case "failed":
        return t("failed", $preferencesStore.language);
      default:
        return status;
    }
  }

  function jobOperationLabel(operation: NoteConnectionJob["operation"]): string {
    return operation === "import"
      ? t("import", $preferencesStore.language)
      : t("noteConnectionPush", $preferencesStore.language);
  }

  function jobProcessedCount(job: NoteConnectionJob): number {
    return (
      jobCount(job, "imported_count") +
      jobCount(job, "pushed_count") +
      jobCount(job, "skipped_count") +
      jobCount(job, "failed_count")
    );
  }

  function jobCount(job: NoteConnectionJob, key: keyof Pick<NoteConnectionJob, "total_count" | "imported_count" | "pushed_count" | "skipped_count" | "failed_count">): number {
    const value = job[key];
    return typeof value === "number" && Number.isFinite(value) ? value : 0;
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
    if (importing) return;
    importAccountID = null;
    importTargetID = "";
    importPreserveRemoteHierarchy = true;
    targets = [];
    importMessage = "";
    importProgressJob = null;
  }

  async function runImport(): Promise<void> {
    if (!selectedImportAccount || importing) return;
    importing = true;
    importMessage = "";
    importProgressJob = null;
    startImportPolling(selectedImportAccount.id);
    try {
      const result = await importFromNoteConnection(
        selectedImportAccount.id,
        importTargetID,
        importLimit,
        importPreserveRemoteHierarchy,
      );
      importMessage = `${t("noteConnectionImportSuccess", $preferencesStore.language)}: ${result.imported_count}/${result.total_count}`;
      notify(t("noteConnectionImportSuccess", $preferencesStore.language), "success");
      await Promise.all([
        notesStore.load(),
        notesStore.loadCalendarNotes(),
        foldersStore.load(),
        loadState(),
      ]);
    } catch (importError) {
      notify(
        importError instanceof Error
          ? importError.message
          : t("importFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      importing = false;
      stopImportPolling();
    }
  }

  function startImportPolling(accountID: number): void {
    stopImportPolling();
    const poll = async () => {
      try {
        const nextJobs = await listNoteConnectionJobs();
        jobs = nextJobs;
        importProgressJob =
          nextJobs.find(
            (job) =>
              job.account_id === accountID &&
              job.operation === "import" &&
              job.status === "running",
          ) ??
          nextJobs.find((job) => job.account_id === accountID && job.operation === "import") ??
          importProgressJob;
      } catch {
        // The import request itself reports failures; polling is only for progress.
      }
    };
    void poll();
    importPollTimer = setInterval(poll, 1000);
  }

  function stopImportPolling(): void {
    if (importPollTimer) {
      clearInterval(importPollTimer);
      importPollTimer = null;
    }
  }

  function formatTime(value?: string): string {
    if (!value) return t("backupNever", $preferencesStore.language);
    const parsed = new Date(value);
    if (Number.isNaN(parsed.getTime())) return value;
    if (parsed.getUTCFullYear() <= 1) return t("backupNever", $preferencesStore.language);
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
            <span class="connected-account__provider">{providerLabel(account.provider)}</span>
            <div>
              <div class="connected-account__title-row">
                <h4>{account.name}</h4>
                <span
                  class:disabled={!account.enabled}
                  class:failed={account.last_test_status === "failed"}
                  class:success={account.last_test_status === "success"}
                  class="connected-account__status"
                >
                  {accountStatusLabel(account)}
                </span>
              </div>
              <p>
                {account.endpoint || account.default_target_name || account.default_target_id || providerLabel(account.provider)}
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
        <button type="button" disabled={importing} on:click={resetImport}>{t("cancel", $preferencesStore.language)}</button>
      </div>
      <div class="connected-form__grid">
        <label>
          <span>{t("noteConnectionImportTarget", $preferencesStore.language)}</span>
          <select bind:value={importTargetID} disabled={importing || targets.length === 0}>
            {#each targets as target (target.id)}
              <option value={target.id}>{target.name || target.id}</option>
            {/each}
          </select>
        </label>
        <label>
          <span>{t("noteConnectionImportLimit", $preferencesStore.language)}</span>
          <input bind:value={importLimit} disabled={importing} min="1" max="200" type="number" />
        </label>
      </div>
      <label class="connected-import-toggle">
        <input
          bind:checked={importPreserveRemoteHierarchy}
          disabled={importing}
          type="checkbox"
        />
        <span aria-hidden="true"></span>
        <div>
          <strong>{t("noteConnectionPreserveHierarchy", $preferencesStore.language)}</strong>
          <small>{t("noteConnectionPreserveHierarchyHint", $preferencesStore.language)}</small>
        </div>
      </label>
      {#if targets.length === 0}
        <p class="connected-panel__muted">{t("noteConnectionNoTargets", $preferencesStore.language)}</p>
      {/if}
      {#if importing || visibleImportJob}
        <div
          class:indeterminate={importProgressTotal === 0}
          class="connected-import-progress"
          role="progressbar"
          aria-valuemin="0"
          aria-valuemax={importProgressTotal || 100}
          aria-valuenow={importProgressTotal ? importProcessedCount : undefined}
        >
          <span style={`width: ${importProgressTotal ? importProgressPercent : 36}%`}></span>
        </div>
        <p class="connected-panel__muted">
          {t("importing", $preferencesStore.language)}
          {#if importProgressTotal > 0}
            · {importProcessedCount}/{importProgressTotal}
          {/if}
        </p>
      {/if}
      {#if importMessage}
        <p class="connected-test-message success">{importMessage}</p>
      {/if}
      <div class="connected-form__actions">
        <button class="primary" type="button" disabled={working || importing || targets.length === 0} on:click={() => void runImport()}>
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
            <span>{providerLabel(job.provider)} · {jobOperationLabel(job.operation)}</span>
            <strong class:failed={job.status === "failed"} class:success={job.status === "completed"}>
              {jobStatusLabel(job.status)}
            </strong>
            <small>
              {jobProcessedCount(job)}/{jobCount(job, "total_count") || jobProcessedCount(job)}
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
    min-width: 0;
    width: 100%;
    display: grid;
    gap: 16px;
    color: var(--color-text, #1a1a18);
    overflow: hidden;
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
    gap: 16px;
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
    min-width: 0;
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
    gap: 12px;
  }

  .connected-account,
  .connected-form,
  .connected-import,
  .connected-empty,
  .connected-jobs {
    min-width: 0;
    width: 100%;
    box-sizing: border-box;
    border: 1px solid var(--color-divider, #e2e0d8);
    border-radius: 8px;
    background: color-mix(in srgb, var(--color-card, #fffefa) 92%, var(--color-surface-secondary, #f4f3ee));
    box-shadow: 0 1px 0 rgb(var(--color-shadow, 26 26 24) / 4%);
  }

  .connected-account,
  .connected-empty,
  .connected-jobs {
    padding: 14px;
  }

  .connected-form,
  .connected-import {
    padding: 16px;
  }

  .connected-panel button,
  .connected-form__grid input,
  .connected-form__grid select {
    min-height: 38px;
    border: 1px solid var(--color-divider, #e2e0d8);
    border-radius: 8px;
    background: var(--color-card, #fffefa);
    color: var(--color-text, #1a1a18);
  }

  .connected-panel button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 7px;
    padding: 0 12px;
    font-size: 13px;
    line-height: 1;
    transition:
      background 140ms ease,
      border-color 140ms ease,
      color 140ms ease,
      opacity 140ms ease;
  }

  .connected-panel button:hover:not(:disabled) {
    border-color: color-mix(in srgb, var(--color-brand, #e8531a) 28%, var(--color-divider, #e2e0d8));
    background: color-mix(in srgb, var(--color-brand, #e8531a) 8%, var(--color-card, #fffefa));
    color: var(--color-brand, #e8531a);
  }

  .connected-panel button:disabled {
    cursor: default;
    opacity: 0.5;
  }

  .connected-account__main {
    gap: 12px;
    min-width: 0;
  }

  .connected-account__main > div {
    min-width: 0;
    display: grid;
    gap: 4px;
  }

  .connected-account__provider {
    width: 48px;
    height: 48px;
    flex: 0 0 48px;
    display: inline-grid;
    place-items: center;
    border: 1px solid var(--color-divider, #e2e0d8);
    border-radius: 8px;
    background: var(--color-surface-secondary, #f4f3ee);
    color: var(--color-brand, #e8531a);
    font-size: 12px;
    font-weight: 600;
    text-align: center;
  }

  .connected-account__title-row {
    min-width: 0;
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .connected-account__title-row h4 {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
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
    min-height: 24px;
    flex: 0 0 auto;
    display: inline-flex;
    align-items: center;
    border-radius: 999px;
    background: var(--color-surface-secondary, #f4f3ee);
    color: var(--color-text-muted, #8c8c84);
    padding: 0 8px;
    font-size: 11px;
    font-weight: 500;
    line-height: 1;
  }

  .connected-account__status.success,
  .connected-test-message.success {
    color: var(--sm-success, #2d7a4f);
  }

  .connected-account__status.success {
    background: var(--sm-success-bg, #eaf5ee);
  }

  .connected-account__status.failed {
    background: var(--sm-error-bg, #fceaea);
    color: var(--color-danger, #cc3333);
  }

  .connected-account__status.disabled {
    background: var(--color-surface-muted, #eceae2);
    color: var(--color-text-muted, #8c8c84);
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
    min-width: 0;
    display: grid;
    gap: 6px;
    color: var(--color-text-secondary, #3a3a34);
    font-size: 12px;
  }

  .connected-form__grid .wide {
    grid-column: 1 / -1;
  }

  .connected-form__grid input,
  .connected-form__grid select {
    box-sizing: border-box;
    min-width: 0;
    width: 100%;
    padding: 0 10px;
  }

  .connected-form__grid input:disabled,
  .connected-form__grid select:disabled {
    color: var(--color-text-muted, #8c8c84);
    opacity: 0.72;
  }

  .connected-form__grid small {
    color: var(--color-text-muted, #8c8c84);
    line-height: 1.5;
  }

  .connected-import-toggle {
    min-width: 0;
    display: grid;
    grid-template-columns: 38px minmax(0, 1fr);
    align-items: center;
    gap: 12px;
    border: 1px solid var(--color-divider, #e2e0d8);
    border-radius: 8px;
    background: var(--color-card, #fffefa);
    padding: 12px;
  }

  .connected-import-toggle input {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip: rect(0 0 0 0);
    white-space: nowrap;
    clip-path: inset(50%);
  }

  .connected-import-toggle > span {
    position: relative;
    width: 38px;
    height: 22px;
    border-radius: 999px;
    background: var(--color-surface-muted, #eceae2);
    transition: background 140ms ease;
  }

  .connected-import-toggle > span::after {
    content: "";
    position: absolute;
    top: 3px;
    left: 3px;
    width: 16px;
    height: 16px;
    border-radius: 999px;
    background: var(--color-card, #fffefa);
    box-shadow: 0 1px 3px rgb(var(--color-shadow, 26 26 24) / 18%);
    transition: transform 140ms ease;
  }

  .connected-import-toggle input:checked + span {
    background: var(--color-brand, #e8531a);
  }

  .connected-import-toggle input:checked + span::after {
    transform: translateX(16px);
  }

  .connected-import-toggle input:disabled + span {
    opacity: 0.55;
  }

  .connected-import-toggle strong,
  .connected-import-toggle small {
    display: block;
  }

  .connected-import-toggle strong {
    color: var(--color-text, #1a1a18);
    font-size: 13px;
    font-weight: 500;
  }

  .connected-import-toggle small {
    margin-top: 3px;
    color: var(--color-text-muted, #8c8c84);
    font-size: 12px;
    line-height: 1.45;
  }

  .connected-import-progress {
    position: relative;
    height: 8px;
    overflow: hidden;
    border-radius: 999px;
    background: var(--color-surface-secondary, #f1eee7);
  }

  .connected-import-progress span {
    position: absolute;
    inset: 0 auto 0 0;
    border-radius: inherit;
    background: var(--color-brand, #e8531a);
    transition: width 160ms ease;
  }

  .connected-import-progress.indeterminate span {
    animation: connected-progress-slide 1.1s ease-in-out infinite;
  }

  @keyframes connected-progress-slide {
    0% {
      left: -36%;
      width: 36%;
    }
    50% {
      left: 32%;
      width: 48%;
    }
    100% {
      left: 100%;
      width: 36%;
    }
  }

  .connected-empty {
    display: grid;
    justify-items: start;
    gap: 10px;
    color: var(--color-text-muted, #8c8c84);
  }

  .connected-job {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto;
    gap: 6px 10px;
    border: 1px solid var(--color-divider, #e2e0d8);
    border-radius: 8px;
    background: var(--color-card, #fffefa);
    padding: 10px;
  }

  .connected-job span {
    min-width: 0;
    overflow: hidden;
    color: var(--color-text-secondary, #3a3a34);
    font-size: 13px;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .connected-job strong {
    min-height: 24px;
    align-self: start;
    display: inline-flex;
    align-items: center;
    border-radius: 999px;
    background: var(--color-surface-secondary, #f4f3ee);
    color: var(--color-text-muted, #8c8c84);
    padding: 0 8px;
    font-size: 11px;
    font-weight: 500;
  }

  .connected-job strong.success {
    background: var(--sm-success-bg, #eaf5ee);
    color: var(--sm-success, #2d7a4f);
  }

  .connected-job strong.failed {
    background: var(--sm-error-bg, #fceaea);
    color: var(--color-danger, #cc3333);
  }

  .connected-job small {
    color: var(--color-text-muted, #8c8c84);
    grid-column: 1 / -1;
    font-size: 12px;
  }

  button.danger {
    color: var(--color-danger, #cc3333);
  }

  button.danger:hover:not(:disabled) {
    border-color: color-mix(in srgb, var(--color-danger, #cc3333) 28%, var(--color-divider, #e2e0d8));
    background: var(--sm-error-bg, #fceaea);
    color: var(--color-danger, #cc3333);
  }

  button.primary {
    border-color: var(--color-brand, #e8531a);
    background: var(--color-brand, #e8531a);
    color: var(--color-on-brand, #fff8f2);
  }

  button.primary:hover:not(:disabled) {
    border-color: var(--color-brand-hover, #c03b0d);
    background: var(--color-brand-hover, #c03b0d);
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

    .connected-account__main {
      align-items: flex-start;
    }

    .connected-form__grid {
      grid-template-columns: 1fr;
    }
  }
</style>
