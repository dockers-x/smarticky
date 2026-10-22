<script lang="ts">
  import { onMount, tick } from "svelte";
  import { dialog } from "../../actions/dialog";
  import {
    createBackupTarget,
    createBackupTask,
    deleteBackupTarget,
    deleteBackupTask,
    listBackupFiles,
    listBackupTargets,
    listBackupTasks,
    restoreBackupFile as restoreBackupFileAPI,
    runBackupTask,
    testBackupTarget,
    testUnsavedBackupTarget,
    updateBackupTarget,
    updateBackupTask,
    verifyBackupFile as verifyBackupFileAPI,
    type BackupFileInfo,
    type BackupSchedule,
    type BackupTarget,
    type BackupTargetInput,
    type BackupTargetType,
    type BackupTask,
    type BackupTaskInput,
    type BackupVerificationResult,
  } from "../../api/backup";
  import { confirmDialog, notify } from "../../stores/dialogs";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";
  import PasswordField from "../common/PasswordField.svelte";

  const restorePhrase = "RESTORE";

  const defaultTargetForm: BackupTargetInput = {
    name: "",
    type: "webdav",
    enabled: true,
    webdav_url: "",
    webdav_user: "",
    webdav_password: "",
    s3_endpoint: "",
    s3_region: "",
    s3_bucket: "",
    s3_access_key: "",
    s3_secret_key: "",
  };

  const defaultTaskForm: BackupTaskInput = {
    name: "",
    enabled: true,
    schedule: "manual",
    retention_days: 30,
    max_count: 10,
    target_ids: [],
  };

  let targets: BackupTarget[] = [];
  let tasks: BackupTask[] = [];
  let loading = true;
  let loaded = false;
  let working = false;
  let saving = false;
  let error = "";

  let targetFormOpen = false;
  let editingTargetID: number | null = null;
  let targetForm: BackupTargetInput = { ...defaultTargetForm };
  let targetTestText = "";
  let targetTestOK: boolean | null = null;
  let targetSaveError = "";
  let targetOpener: HTMLElement | null = null;
  let testingTargetID: number | null = null;

  let taskFormOpen = false;
  let editingTaskID: number | null = null;
  let taskForm: BackupTaskInput = { ...defaultTaskForm };
  let taskSaveError = "";
  let taskOpener: HTMLElement | null = null;

  let filesTarget: BackupTarget | null = null;
  let backups: BackupFileInfo[] = [];
  let filesLoading = false;
  let filesError = "";
  let filesRequest = 0;
  let verificationText = "";

  let restoreBackup: BackupFileInfo | null = null;
  let restoreVerification: BackupVerificationResult | null = null;
  let restoreInput = "";
  let restoreRestartRequired = false;
  let restoreError = "";

  $: selectedTargetCount = taskForm.target_ids.length;
  $: canSubmitRestore = restoreInput === restorePhrase && restoreBackup && filesTarget;

  function freshTargetForm(type: BackupTargetType = "webdav"): BackupTargetInput {
    return { ...defaultTargetForm, type };
  }

  function freshTaskForm(): BackupTaskInput {
    return {
      ...defaultTaskForm,
      target_ids: targets.map((target) => target.id),
    };
  }

  async function loadBackupState(): Promise<void> {
    loading = true;
    error = "";
    try {
      const [nextTargets, nextTasks] = await Promise.all([
        listBackupTargets(),
        listBackupTasks(),
      ]);
      targets = nextTargets;
      tasks = nextTasks;
    } catch (loadError) {
      error =
        loadError instanceof Error
          ? loadError.message
          : t("loadFailed", $preferencesStore.language);
    } finally {
      loading = false;
      loaded = true;
    }
  }

  function startCreateTarget(): void {
    if (saving || working) return;
    targetOpener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    editingTargetID = null;
    targetForm = freshTargetForm();
    targetTestText = "";
    targetTestOK = null;
    targetSaveError = "";
    targetFormOpen = true;
    void focusField("backup-target-name");
  }

  function startEditTarget(target: BackupTarget): void {
    if (saving || working) return;
    targetOpener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    editingTargetID = target.id;
    targetForm = {
      ...freshTargetForm(target.type),
      name: target.name,
      enabled: target.enabled,
      webdav_url: target.webdav_url ?? "",
      webdav_user: target.webdav_user ?? "",
      s3_endpoint: target.s3_endpoint ?? "",
      s3_region: target.s3_region ?? "",
      s3_bucket: target.s3_bucket ?? "",
    };
    targetTestText = "";
    targetTestOK = null;
    targetSaveError = "";
    targetFormOpen = true;
    void focusField("backup-target-name");
  }

  async function focusField(id: string): Promise<void> {
    await tick();
    document.getElementById(id)?.focus();
  }

  function closeTargetForm(): void {
    if (saving || working) return;
    targetFormOpen = false;
    targetOpener?.focus({ preventScroll: true });
  }

  function closeTaskForm(): void {
    if (saving || working) return;
    taskFormOpen = false;
    taskOpener?.focus({ preventScroll: true });
  }

  function targetPayload(): BackupTargetInput {
    const payload: BackupTargetInput = {
      ...targetForm,
      name: targetForm.name.trim(),
      type: targetForm.type,
      enabled: Boolean(targetForm.enabled),
    };
    const editingTarget = targets.find((target) => target.id === editingTargetID);
    if (payload.type === "webdav") {
      payload.s3_endpoint = "";
      payload.s3_region = "";
      payload.s3_bucket = "";
      payload.s3_access_key = "";
      payload.s3_secret_key = "";
      if (editingTarget?.has_webdav_password && !payload.webdav_password) {
        delete payload.webdav_password;
      }
    } else {
      payload.webdav_url = "";
      payload.webdav_user = "";
      payload.webdav_password = "";
      if (editingTarget?.has_s3_access_key && !payload.s3_access_key) {
        delete payload.s3_access_key;
      }
      if (editingTarget?.has_s3_secret_key && !payload.s3_secret_key) {
        delete payload.s3_secret_key;
      }
    }
    return payload;
  }

  async function testTargetForm(): Promise<void> {
    if (working || saving) return;
    working = true;
    targetTestText = "";
    targetTestOK = null;
    try {
      const result =
        editingTargetID === null
          ? await testUnsavedBackupTarget(targetPayload())
          : await testBackupTarget(editingTargetID, targetPayload());
      targetTestOK = result.ok;
      targetTestText = result.ok
        ? t("connectionTestSuccess", $preferencesStore.language)
        : t("connectionTestFailed", $preferencesStore.language);
      if (editingTargetID !== null) await loadBackupState();
    } catch (testError) {
      targetTestOK = false;
      targetTestText =
        testError instanceof Error
          ? testError.message
          : t("connectionTestFailed", $preferencesStore.language);
    } finally {
      working = false;
    }
  }

  async function saveTarget(): Promise<void> {
    if (saving || working) return;
    saving = true;
    targetSaveError = "";
    try {
      if (editingTargetID === null) {
        await createBackupTarget(targetPayload());
      } else {
        await updateBackupTarget(editingTargetID, targetPayload());
      }
      targetFormOpen = false;
      notify(t("saved", $preferencesStore.language), "success");
      await loadBackupState();
    } catch (saveError) {
      targetSaveError = saveError instanceof Error
          ? saveError.message
          : t("saveFailed", $preferencesStore.language);
    } finally {
      saving = false;
      if (!targetFormOpen) {
        await tick();
        if (targetOpener?.isConnected) targetOpener.focus({ preventScroll: true });
      }
    }
  }

  async function testSavedTarget(target: BackupTarget): Promise<void> {
    if (working || saving) return;
    working = true;
    testingTargetID = target.id;
    try {
      const result = await testBackupTarget(target.id);
      notify(t(result.ok ? "connectionTestSuccess" : "connectionTestFailed", $preferencesStore.language), result.ok ? "success" : "error");
      await loadBackupState();
    } catch (testError) {
      notify(testError instanceof Error ? testError.message : t("connectionTestFailed", $preferencesStore.language), "error");
    } finally {
      working = false;
      testingTargetID = null;
    }
  }

  async function removeTarget(target: BackupTarget): Promise<void> {
    if (working || saving) return;
    const confirmed = await confirmDialog({
      title: t("delete", $preferencesStore.language),
      message: `${t("deleteBackupTargetConfirm", $preferencesStore.language)} "${target.name}"?`,
      confirmLabel: t("delete", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (!confirmed) return;

    working = true;
    try {
      await deleteBackupTarget(target.id);
      if (filesTarget?.id === target.id) closeFiles();
      await loadBackupState();
    } catch (deleteError) {
      notify(
        deleteError instanceof Error
          ? deleteError.message
          : t("deleteFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      working = false;
    }
  }

  function startCreateTask(): void {
    if (saving || working) return;
    taskOpener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    editingTaskID = null;
    taskForm = freshTaskForm();
    taskFormOpen = true;
    taskSaveError = "";
    void focusField("backup-task-name");
  }

  function startEditTask(task: BackupTask): void {
    if (saving || working) return;
    taskOpener = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    editingTaskID = task.id;
    taskForm = {
      name: task.name,
      enabled: task.enabled,
      schedule: task.schedule,
      retention_days: task.retention_days,
      max_count: task.max_count,
      target_ids: [...task.target_ids],
    };
    taskFormOpen = true;
    taskSaveError = "";
    void focusField("backup-task-name");
  }

  function toggleTaskTarget(targetID: number, checked: boolean): void {
    const ids = new Set(taskForm.target_ids);
    if (checked) ids.add(targetID);
    else ids.delete(targetID);
    taskForm = { ...taskForm, target_ids: Array.from(ids).sort((a, b) => a - b) };
  }

  async function saveTask(): Promise<void> {
    if (saving || working || !selectedTargetCount) return;
    saving = true;
    taskSaveError = "";
    try {
      const payload: BackupTaskInput = {
        ...taskForm,
        name: taskForm.name.trim(),
        retention_days: Number(taskForm.retention_days) || 0,
        max_count: Number(taskForm.max_count) || 0,
      };
      if (editingTaskID === null) {
        await createBackupTask(payload);
      } else {
        await updateBackupTask(editingTaskID, payload);
      }
      taskFormOpen = false;
      notify(t("saved", $preferencesStore.language), "success");
      await loadBackupState();
    } catch (saveError) {
      taskSaveError = saveError instanceof Error
          ? saveError.message
          : t("saveFailed", $preferencesStore.language);
    } finally {
      saving = false;
      if (!taskFormOpen) {
        await tick();
        if (taskOpener?.isConnected) taskOpener.focus({ preventScroll: true });
      }
    }
  }

  async function removeTask(task: BackupTask): Promise<void> {
    if (working || saving) return;
    const confirmed = await confirmDialog({
      title: t("delete", $preferencesStore.language),
      message: `${t("deleteBackupTaskConfirm", $preferencesStore.language)} "${task.name}"?`,
      confirmLabel: t("delete", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (!confirmed) return;

    working = true;
    try {
      await deleteBackupTask(task.id);
      await loadBackupState();
    } catch (deleteError) {
      notify(
        deleteError instanceof Error
          ? deleteError.message
          : t("deleteFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      working = false;
    }
  }

  async function runTask(task: BackupTask): Promise<void> {
    if (working || saving) return;
    working = true;
    try {
      const result = await runBackupTask(task.id);
      const okCount = result.results.filter((item) => item.ok).length;
      notify(
        `${t("backupSuccess", $preferencesStore.language)}: ${okCount}/${result.results.length}`,
        okCount === result.results.length ? "success" : "info",
      );
      await loadBackupState();
    } catch (backupError) {
      notify(
        backupError instanceof Error
          ? backupError.message
          : t("backupFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      working = false;
    }
  }

  async function openFiles(target: BackupTarget): Promise<void> {
    const request = ++filesRequest;
    filesTarget = target;
    backups = [];
    filesLoading = true;
    filesError = "";
    verificationText = "";
    restoreBackup = null;
    restoreVerification = null;
    restoreInput = "";
    try {
      const result = await listBackupFiles(target.id);
      if (request === filesRequest) backups = result.backups ?? [];
    } catch (listError) {
      if (request !== filesRequest) return;
      filesError = listError instanceof Error
          ? listError.message
          : t("loadFailed", $preferencesStore.language);
    } finally {
      if (request === filesRequest) filesLoading = false;
    }
  }

  function closeFiles(): void {
    if (working) return;
    filesRequest += 1;
    filesTarget = null;
    backups = [];
    verificationText = "";
    restoreBackup = null;
    restoreVerification = null;
    restoreInput = "";
    filesError = "";
    filesLoading = false;
  }

  function verificationSummary(result: BackupVerificationResult): string {
    if (!result.valid) return result.error || t("backupVerifyFailed", $preferencesStore.language);
    return [
      t("backupVerifySuccess", $preferencesStore.language),
      `${t("backupFiles", $preferencesStore.language)}: ${result.file_count}`,
      `${t("backupTotalSize", $preferencesStore.language)}: ${formatFileSize(result.total_size)}`,
      ...result.file_checks.map((check) => {
        const marker = check.exists ? "OK" : "MISS";
        return `${marker} ${check.path}${check.error ? ` - ${check.error}` : ""}`;
      }),
    ].join("\n");
  }

  async function verifyBackup(backup: BackupFileInfo): Promise<BackupVerificationResult | null> {
    if (!filesTarget) return null;
    working = true;
    verificationText = "";
    try {
      const result = await verifyBackupFileAPI(filesTarget.id, backup.filename);
      verificationText = verificationSummary(result);
      return result;
    } catch (verifyError) {
      verificationText =
        verifyError instanceof Error
          ? verifyError.message
          : t("backupVerifyFailed", $preferencesStore.language);
      return null;
    } finally {
      working = false;
    }
  }

  async function startRestore(backup: BackupFileInfo): Promise<void> {
    const result = await verifyBackup(backup);
    if (!result?.valid || !filesTarget) {
      notify(t("backupVerifyFailed", $preferencesStore.language), "error");
      return;
    }
    restoreBackup = backup;
    restoreVerification = result;
    restoreInput = "";
    restoreError = "";
  }

  function closeRestore(): void {
    if (working) return;
    restoreBackup = null;
    restoreVerification = null;
    restoreInput = "";
    restoreError = "";
  }

  async function submitRestore(): Promise<void> {
    if (working || !filesTarget || !restoreBackup || restoreInput !== restorePhrase) return;
    working = true;
    restoreError = "";
    try {
      const result = await restoreBackupFileAPI(
        filesTarget.id,
        restoreBackup.filename,
        restoreInput,
      );
      const needsRestart = result.restart_required === true;
      notify(
        result.warning || t("restoreSuccess", $preferencesStore.language),
        needsRestart ? "info" : "success",
      );
      restoreBackup = null;
      restoreVerification = null;
      restoreInput = "";
      restoreRestartRequired = needsRestart;
      if (!needsRestart) {
        await Promise.all([notesStore.load(), notesStore.loadCalendarNotes()]);
      }
    } catch (failure) {
      restoreError = failure instanceof Error ? failure.message : t("restoreFailed", $preferencesStore.language);
    } finally {
      working = false;
    }
  }

  function targetTypeLabel(type: BackupTargetType): string {
    return type === "webdav" ? "WebDAV" : "S3";
  }

  function scheduleLabel(schedule: BackupSchedule): string {
    switch (schedule) {
      case "daily":
        return t("backupDaily", $preferencesStore.language);
      case "weekly":
        return t("backupWeekly", $preferencesStore.language);
      case "monthly":
        return t("backupMonthly", $preferencesStore.language);
      default:
        return t("backupManual", $preferencesStore.language);
    }
  }

  function statusLabel(status: string): string {
    if (status === "success") return t("success", $preferencesStore.language);
    if (status === "failed") return t("failed", $preferencesStore.language);
    return t("backupNever", $preferencesStore.language);
  }

  function targetNames(task: BackupTask): string {
    if (!task.targets.length) return "-";
    return task.targets.map((target) => target.name).join(", ");
  }

  function formatFileSize(bytes: number): string {
    if (!bytes) return "0 B";
    const units = ["B", "KB", "MB", "GB"];
    const index = Math.min(
      Math.floor(Math.log(bytes) / Math.log(1024)),
      units.length - 1,
    );
    return `${Math.round((bytes / 1024 ** index) * 100) / 100} ${units[index]}`;
  }

  function formatDate(value: string | undefined): string {
    if (!value) return "-";
    return new Date(value).toLocaleString(
      $preferencesStore.language === "zh" ? "zh-CN" : "en-US",
    );
  }

  onMount(() => {
    void loadBackupState();
  });
</script>

<div class="settings-view">
  {#if restoreBackup && filesTarget}
    <div class="backup-restore-dialog" role="presentation">
      <div class="backup-restore-dialog__panel" role="dialog" aria-modal="true" aria-labelledby="backup-restore-title" aria-describedby="backup-restore-description" aria-busy={working} tabindex="-1" use:dialog={{ onDismiss: closeRestore, initialFocus: "[data-restore-cancel]", dismissible: !working }}>
        <form class="settings-form" on:submit|preventDefault={() => void submitRestore()}>
        <h3 id="backup-restore-title">{t("restoreDangerTitle", $preferencesStore.language)}</h3>
        <p id="backup-restore-description">{t("restoreDangerMessage", $preferencesStore.language)}</p>
        <div class="settings-kv">
          <span>{t("backupTarget", $preferencesStore.language)}</span>
          <strong>{filesTarget.name} · {targetTypeLabel(filesTarget.type)}</strong>
        </div>
        <div class="settings-kv">
          <span>{t("backupFilename", $preferencesStore.language)}</span>
          <strong title={restoreBackup.filename}>{restoreBackup.filename}</strong>
        </div>
        <div class="settings-kv">
          <span>{t("backupSize", $preferencesStore.language)}</span>
          <strong>{formatFileSize(restoreBackup.size)}</strong>
        </div>
        {#if restoreVerification}
          <pre class="settings-result">{verificationSummary(restoreVerification)}</pre>
        {/if}
        <label class="backup-restore-dialog__input">
          <span>{t("typeRestoreToConfirm", $preferencesStore.language)}</span>
          <input
            bind:value={restoreInput}
            disabled={working}
            type="text"
            autocomplete="off"
            autocapitalize="characters"
            autocorrect="off"
            enterkeyhint="done"
            spellcheck="false"
            placeholder={restorePhrase}
          />
        </label>
        {#if restoreError}<p class="settings-error" role="alert">{restoreError}</p>{/if}
        <div class="settings-actions">
          <button
            type="button"
            data-restore-cancel
            disabled={working}
            on:click={closeRestore}
          >
            {t("cancel", $preferencesStore.language)}
          </button>
          <button
            type="submit"
            class="danger"
            disabled={!canSubmitRestore || working}
          >
            {t(working ? "loading" : "restore", $preferencesStore.language)}
          </button>
        </div>
        </form>
      </div>
    </div>
  {/if}

  {#if restoreRestartRequired}
    <section class="settings-section">
      <div class="settings-section__header">
        <h3>{t("restoreConfirm", $preferencesStore.language)}</h3>
      </div>
      <p class="settings-result" role="status">{t("restoreSuccess", $preferencesStore.language)}</p>
    </section>
  {:else if filesTarget}
    <section class="settings-section">
      <div class="settings-section__header">
        <h3>{t("backupListTitle", $preferencesStore.language)} · {filesTarget.name}</h3>
        <div class="settings-row-actions">
          <button type="button" disabled={filesLoading || working} on:click={() => openFiles(filesTarget!)}>
            {t("refresh", $preferencesStore.language)}
          </button>
          <button type="button" disabled={working} on:click={closeFiles}>
            {t("back", $preferencesStore.language)}
          </button>
        </div>
      </div>
      {#if filesLoading}
        <p class="settings-muted" role="status">{t("loading", $preferencesStore.language)}</p>
      {:else if filesError}
        <p class="settings-error" role="alert">{filesError}</p>
      {:else if backups.length === 0}
        <p class="settings-empty">{t("backupNoFiles", $preferencesStore.language)}</p>
      {:else}
        <div class="backup-list">
          {#each backups as backup (backup.filename)}
            <article class="backup-list-row">
              <div>
                <strong title={backup.filename}>{backup.filename}</strong>
                <span>{formatFileSize(backup.size)} · {formatDate(backup.created_at)}</span>
              </div>
              <div class="settings-row-actions">
                <button type="button" disabled={working} on:click={() => verifyBackup(backup)}>
                  {t("backupVerify", $preferencesStore.language)}
                </button>
                <button type="button" class="danger" disabled={working} on:click={() => startRestore(backup)}>
                  {t("restore", $preferencesStore.language)}
                </button>
              </div>
            </article>
          {/each}
        </div>
      {/if}
      {#if verificationText}
        <pre class="settings-result" role="status">{verificationText}</pre>
      {/if}
    </section>
  {:else if loading && !loaded}
    <p class="settings-muted" role="status">{t("loading", $preferencesStore.language)}</p>
  {:else}
    {#if error}
      <p class="settings-error" role="alert">{error}</p>
      <div class="settings-actions"><button type="button" disabled={loading} on:click={() => void loadBackupState()}>{t("refresh", $preferencesStore.language)}</button></div>
    {/if}
    {#if loading}
      <p class="settings-muted" role="status">{t("loading", $preferencesStore.language)}</p>
    {/if}

    <section class="settings-section">
      <div class="settings-section__header">
        <h3>{t("backupTargets", $preferencesStore.language)}</h3>
        <button type="button" disabled={working || saving} on:click={startCreateTarget}>
          {t("addBackupTarget", $preferencesStore.language)}
        </button>
      </div>

      {#if targets.length === 0}
        <p class="settings-empty">{t("backupNoTargets", $preferencesStore.language)}</p>
      {:else}
        <div class="backup-list">
          {#each targets as target (target.id)}
            <article class="backup-list-row">
              <div>
                <strong>{target.name}</strong>
                <span>
                  {targetTypeLabel(target.type)} · {target.enabled ? t("enabled", $preferencesStore.language) : t("disabled", $preferencesStore.language)}
                  · {t("backupLast", $preferencesStore.language)} {formatDate(target.last_backup_at)}
                  · {statusLabel(target.last_backup_status)}
                </span>
              </div>
              <div class="settings-row-actions">
                <button type="button" disabled={working || saving} on:click={() => void testSavedTarget(target)}>
                  {t(testingTargetID === target.id ? "loading" : "testConnection", $preferencesStore.language)}
                </button>
                <button type="button" disabled={working || saving} on:click={() => openFiles(target)}>
                  {t("backupList", $preferencesStore.language)}
                </button>
                <button type="button" disabled={working || saving} on:click={() => startEditTarget(target)}>
                  {t("edit", $preferencesStore.language)}
                </button>
                <button type="button" class="danger" disabled={working || saving} on:click={() => removeTarget(target)}>
                  {t("delete", $preferencesStore.language)}
                </button>
              </div>
            </article>
          {/each}
        </div>
      {/if}

      {#if targetFormOpen}
        <form class="backup-editor" aria-busy={saving || working} on:input={() => { targetTestText = ""; targetTestOK = null; }} on:submit|preventDefault={() => void saveTarget()}>
          <h3>
            {editingTargetID === null
              ? t("addBackupTarget", $preferencesStore.language)
              : t("editBackupTarget", $preferencesStore.language)}
          </h3>
          <div class="settings-form settings-form--grid">
            <label>
              <span>{t("backupTargetName", $preferencesStore.language)}</span>
              <input id="backup-target-name" bind:value={targetForm.name} type="text" required disabled={saving || working} />
            </label>
            <label>
              <span>{t("backupTargetType", $preferencesStore.language)}</span>
              <select bind:value={targetForm.type} disabled={saving || working}>
                <option value="webdav">WebDAV</option>
                <option value="s3">S3</option>
              </select>
            </label>
            <label class="settings-switch-row">
              <span>{t("enabled", $preferencesStore.language)}</span>
              <input bind:checked={targetForm.enabled} type="checkbox" disabled={saving || working} />
            </label>
            {#if targetForm.type === "webdav"}
              <label>
                <span>{t("webdavUrl", $preferencesStore.language)}</span>
                <input bind:value={targetForm.webdav_url} type="url" placeholder="https://dav.example.com/path" required disabled={saving || working} />
              </label>
              <label>
                <span>{t("webdavUsername", $preferencesStore.language)}</span>
                <input bind:value={targetForm.webdav_user} type="text" autocomplete="username" disabled={saving || working} />
              </label>
              <PasswordField bind:value={targetForm.webdav_password} label={t("webdavPassword", $preferencesStore.language)} autocomplete="current-password" disabled={saving || working} showPasswordLabel={t("showPassword", $preferencesStore.language)} hidePasswordLabel={t("hidePassword", $preferencesStore.language)} />
            {:else}
              <label>
                <span>{t("s3Endpoint", $preferencesStore.language)}</span>
                <input bind:value={targetForm.s3_endpoint} type="text" placeholder="https://s3.example.com" disabled={saving || working} />
              </label>
              <label>
                <span>{t("s3Region", $preferencesStore.language)}</span>
                <input bind:value={targetForm.s3_region} type="text" placeholder="us-east-1" disabled={saving || working} />
              </label>
              <label>
                <span>{t("s3Bucket", $preferencesStore.language)}</span>
                <input bind:value={targetForm.s3_bucket} type="text" disabled={saving || working} />
              </label>
              <PasswordField bind:value={targetForm.s3_access_key} label={t("s3AccessKey", $preferencesStore.language)} autocomplete="off" disabled={saving || working} showPasswordLabel={t("showPassword", $preferencesStore.language)} hidePasswordLabel={t("hidePassword", $preferencesStore.language)} />
              <PasswordField bind:value={targetForm.s3_secret_key} label={t("s3SecretKey", $preferencesStore.language)} autocomplete="off" disabled={saving || working} showPasswordLabel={t("showPassword", $preferencesStore.language)} hidePasswordLabel={t("hidePassword", $preferencesStore.language)} />
            {/if}
          </div>
          {#if targetTestText}
            <p class={targetTestOK === false ? "settings-error" : "settings-empty"} role={targetTestOK === false ? "alert" : "status"}>
              {targetTestText}
            </p>
          {/if}
          {#if targetSaveError}<p class="settings-error" role="alert">{targetSaveError}</p>{/if}
          <div class="settings-actions">
            <button type="button" disabled={working || saving} on:click={testTargetForm}>
              {t(working ? "loading" : "testConnection", $preferencesStore.language)}
            </button>
            <button type="button" disabled={working || saving} on:click={closeTargetForm}>
              {t("cancel", $preferencesStore.language)}
            </button>
            <button type="submit" class="primary" disabled={saving || working}>
              {t(saving ? "saving" : "saveConfig", $preferencesStore.language)}
            </button>
          </div>
        </form>
      {/if}
    </section>

    <section class="settings-section">
      <div class="settings-section__header">
        <h3>{t("backupTasks", $preferencesStore.language)}</h3>
        <button type="button" disabled={working || saving || targets.length === 0} on:click={startCreateTask}>
          {t("addBackupTask", $preferencesStore.language)}
        </button>
      </div>

      {#if tasks.length === 0}
        <p class="settings-empty">{t("backupNoTasks", $preferencesStore.language)}</p>
      {:else}
        <div class="backup-list">
          {#each tasks as task (task.id)}
            <article class="backup-list-row">
              <div>
                <strong>{task.name}</strong>
                <span>
                  {scheduleLabel(task.schedule)} · {task.enabled ? t("enabled", $preferencesStore.language) : t("disabled", $preferencesStore.language)}
                  · {t("backupTargets", $preferencesStore.language)}: {targetNames(task)}
                </span>
                <span>
                  {t("backupRetentionDays", $preferencesStore.language)} {task.retention_days}
                  · {t("backupMaxCount", $preferencesStore.language)} {task.max_count}
                  · {t("backupLast", $preferencesStore.language)} {formatDate(task.last_backup_at)}
                  · {t("backupNext", $preferencesStore.language)} {formatDate(task.next_run_at)}
                </span>
              </div>
              <div class="settings-row-actions">
                <button type="button" disabled={working || saving || task.target_ids.length === 0} on:click={() => runTask(task)}>
                  {t("backupNow", $preferencesStore.language)}
                </button>
                <button type="button" disabled={working || saving} on:click={() => startEditTask(task)}>
                  {t("edit", $preferencesStore.language)}
                </button>
                <button type="button" class="danger" disabled={working || saving} on:click={() => removeTask(task)}>
                  {t("delete", $preferencesStore.language)}
                </button>
              </div>
            </article>
          {/each}
        </div>
      {/if}

      {#if taskFormOpen}
        <form class="backup-editor" aria-busy={saving} on:submit|preventDefault={() => void saveTask()}>
          <h3>
            {editingTaskID === null
              ? t("addBackupTask", $preferencesStore.language)
              : t("editBackupTask", $preferencesStore.language)}
          </h3>
          <div class="settings-form settings-form--grid">
            <label>
              <span>{t("backupTaskName", $preferencesStore.language)}</span>
              <input id="backup-task-name" bind:value={taskForm.name} type="text" required disabled={saving} />
            </label>
            <label>
              <span>{t("backupSchedule", $preferencesStore.language)}</span>
              <select bind:value={taskForm.schedule} disabled={saving}>
                <option value="manual">{t("backupManual", $preferencesStore.language)}</option>
                <option value="daily">{t("backupDaily", $preferencesStore.language)}</option>
                <option value="weekly">{t("backupWeekly", $preferencesStore.language)}</option>
                <option value="monthly">{t("backupMonthly", $preferencesStore.language)}</option>
              </select>
            </label>
            <label class="settings-switch-row">
              <span>{t("enabled", $preferencesStore.language)}</span>
              <input bind:checked={taskForm.enabled} type="checkbox" disabled={saving} />
            </label>
            <label>
              <span>{t("backupRetentionDays", $preferencesStore.language)}</span>
              <small>{t("backupRetentionHelp", $preferencesStore.language)}</small>
              <input bind:value={taskForm.retention_days} min="0" step="1" inputmode="numeric" type="number" disabled={saving} />
            </label>
            <label>
              <span>{t("backupMaxCount", $preferencesStore.language)}</span>
              <small>{t("backupMaxCountHelp", $preferencesStore.language)}</small>
              <input bind:value={taskForm.max_count} min="0" step="1" inputmode="numeric" type="number" disabled={saving} />
            </label>
          </div>
          <div class="backup-target-picker" role="group" aria-label={t("selectedTargets", $preferencesStore.language)}>
            <strong>{t("selectedTargets", $preferencesStore.language)} · {selectedTargetCount}</strong>
            {#each targets as target (target.id)}
              <label>
                <input
                  type="checkbox"
                  disabled={saving}
                  checked={taskForm.target_ids.includes(target.id)}
                  on:change={(event) => toggleTaskTarget(target.id, event.currentTarget.checked)}
                />
                <span>{target.name} · {targetTypeLabel(target.type)}</span>
              </label>
            {/each}
          </div>
          <div class="settings-actions">
            <button type="button" disabled={saving || working} on:click={closeTaskForm}>
              {t("cancel", $preferencesStore.language)}
            </button>
            <button
              type="submit"
              class="primary"
              disabled={saving || working || selectedTargetCount === 0}
            >
              {t(saving ? "saving" : "saveSettings", $preferencesStore.language)}
            </button>
          </div>
          {#if taskSaveError}<p class="settings-error" role="alert">{taskSaveError}</p>{/if}
        </form>
      {/if}
    </section>
  {/if}
</div>
