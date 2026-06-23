<script lang="ts">
  import { onMount } from "svelte";
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
  let working = false;
  let saving = false;
  let error = "";

  let targetFormOpen = false;
  let editingTargetID: number | null = null;
  let targetForm: BackupTargetInput = { ...defaultTargetForm };
  let targetTestText = "";
  let targetTestOK: boolean | null = null;

  let taskFormOpen = false;
  let editingTaskID: number | null = null;
  let taskForm: BackupTaskInput = { ...defaultTaskForm };

  let filesTarget: BackupTarget | null = null;
  let backups: BackupFileInfo[] = [];
  let filesLoading = false;
  let verificationText = "";

  let restoreBackup: BackupFileInfo | null = null;
  let restoreVerification: BackupVerificationResult | null = null;
  let restoreInput = "";
  let restoreRestartRequired = false;

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
    }
  }

  function startCreateTarget(): void {
    editingTargetID = null;
    targetForm = freshTargetForm();
    targetTestText = "";
    targetTestOK = null;
    targetFormOpen = true;
  }

  function startEditTarget(target: BackupTarget): void {
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
    targetFormOpen = true;
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
    saving = true;
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

  async function removeTarget(target: BackupTarget): Promise<void> {
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
    editingTaskID = null;
    taskForm = freshTaskForm();
    taskFormOpen = true;
  }

  function startEditTask(task: BackupTask): void {
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
  }

  function toggleTaskTarget(targetID: number, checked: boolean): void {
    const ids = new Set(taskForm.target_ids);
    if (checked) ids.add(targetID);
    else ids.delete(targetID);
    taskForm = { ...taskForm, target_ids: Array.from(ids).sort((a, b) => a - b) };
  }

  async function saveTask(): Promise<void> {
    saving = true;
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

  async function removeTask(task: BackupTask): Promise<void> {
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
    filesTarget = target;
    backups = [];
    filesLoading = true;
    verificationText = "";
    restoreBackup = null;
    restoreVerification = null;
    restoreInput = "";
    try {
      const result = await listBackupFiles(target.id);
      backups = result.backups ?? [];
    } catch (listError) {
      notify(
        listError instanceof Error
          ? listError.message
          : t("loadFailed", $preferencesStore.language),
        "error",
      );
      filesTarget = null;
    } finally {
      filesLoading = false;
    }
  }

  function closeFiles(): void {
    filesTarget = null;
    backups = [];
    verificationText = "";
    restoreBackup = null;
    restoreVerification = null;
    restoreInput = "";
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
  }

  async function submitRestore(): Promise<void> {
    if (!filesTarget || !restoreBackup || restoreInput !== restorePhrase) return;
    working = true;
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
        await notesStore.load();
      }
    } catch (restoreError) {
      notify(
        restoreError instanceof Error
          ? restoreError.message
          : t("restoreFailed", $preferencesStore.language),
        "error",
      );
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
    <div class="backup-restore-dialog" role="dialog" aria-modal="true">
      <div class="backup-restore-dialog__panel">
        <h3>{t("restoreDangerTitle", $preferencesStore.language)}</h3>
        <p>{t("restoreDangerMessage", $preferencesStore.language)}</p>
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
            type="text"
            autocomplete="off"
            spellcheck="false"
            placeholder={restorePhrase}
          />
        </label>
        <div class="settings-actions">
          <button
            type="button"
            on:click={() => {
              restoreBackup = null;
              restoreVerification = null;
              restoreInput = "";
            }}
          >
            {t("cancel", $preferencesStore.language)}
          </button>
          <button
            type="button"
            class="danger"
            disabled={!canSubmitRestore || working}
            on:click={submitRestore}
          >
            {t("restore", $preferencesStore.language)}
          </button>
        </div>
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
          <button type="button" disabled={filesLoading} on:click={() => openFiles(filesTarget!)}>
            {t("refresh", $preferencesStore.language)}
          </button>
          <button type="button" on:click={closeFiles}>
            {t("back", $preferencesStore.language)}
          </button>
        </div>
      </div>
      {#if filesLoading}
        <p class="settings-muted">{t("loading", $preferencesStore.language)}</p>
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
        <pre class="settings-result">{verificationText}</pre>
      {/if}
    </section>
  {:else if loading}
    <p class="settings-muted">{t("loading", $preferencesStore.language)}</p>
  {:else}
    {#if error}
      <p class="settings-error" role="alert">{error}</p>
    {/if}

    <section class="settings-section">
      <div class="settings-section__header">
        <h3>{t("backupTargets", $preferencesStore.language)}</h3>
        <button type="button" on:click={startCreateTarget}>
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
                <button type="button" disabled={working} on:click={() => testBackupTarget(target.id).then(loadBackupState)}>
                  {t("testConnection", $preferencesStore.language)}
                </button>
                <button type="button" disabled={working} on:click={() => openFiles(target)}>
                  {t("backupList", $preferencesStore.language)}
                </button>
                <button type="button" on:click={() => startEditTarget(target)}>
                  {t("edit", $preferencesStore.language)}
                </button>
                <button type="button" class="danger" disabled={working} on:click={() => removeTarget(target)}>
                  {t("delete", $preferencesStore.language)}
                </button>
              </div>
            </article>
          {/each}
        </div>
      {/if}

      {#if targetFormOpen}
        <div class="backup-editor">
          <h3>
            {editingTargetID === null
              ? t("addBackupTarget", $preferencesStore.language)
              : t("editBackupTarget", $preferencesStore.language)}
          </h3>
          <div class="settings-form settings-form--grid">
            <label>
              <span>{t("backupTargetName", $preferencesStore.language)}</span>
              <input bind:value={targetForm.name} type="text" />
            </label>
            <label>
              <span>{t("backupTargetType", $preferencesStore.language)}</span>
              <select bind:value={targetForm.type}>
                <option value="webdav">WebDAV</option>
                <option value="s3">S3</option>
              </select>
            </label>
            <label class="settings-switch-row">
              <span>{t("enabled", $preferencesStore.language)}</span>
              <input bind:checked={targetForm.enabled} type="checkbox" />
            </label>
            {#if targetForm.type === "webdav"}
              <label>
                <span>{t("webdavUrl", $preferencesStore.language)}</span>
                <input bind:value={targetForm.webdav_url} type="url" placeholder="https://dav.example.com/path" />
              </label>
              <label>
                <span>{t("webdavUsername", $preferencesStore.language)}</span>
                <input bind:value={targetForm.webdav_user} type="text" autocomplete="username" />
              </label>
              <label>
                <span>{t("webdavPassword", $preferencesStore.language)}</span>
                <input bind:value={targetForm.webdav_password} type="password" autocomplete="current-password" />
              </label>
            {:else}
              <label>
                <span>{t("s3Endpoint", $preferencesStore.language)}</span>
                <input bind:value={targetForm.s3_endpoint} type="text" placeholder="https://s3.example.com" />
              </label>
              <label>
                <span>{t("s3Region", $preferencesStore.language)}</span>
                <input bind:value={targetForm.s3_region} type="text" placeholder="us-east-1" />
              </label>
              <label>
                <span>{t("s3Bucket", $preferencesStore.language)}</span>
                <input bind:value={targetForm.s3_bucket} type="text" />
              </label>
              <label>
                <span>{t("s3AccessKey", $preferencesStore.language)}</span>
                <input bind:value={targetForm.s3_access_key} type="password" autocomplete="off" />
              </label>
              <label>
                <span>{t("s3SecretKey", $preferencesStore.language)}</span>
                <input bind:value={targetForm.s3_secret_key} type="password" autocomplete="off" />
              </label>
            {/if}
          </div>
          {#if targetTestText}
            <p class={targetTestOK === false ? "settings-error" : "settings-empty"}>
              {targetTestText}
            </p>
          {/if}
          <div class="settings-actions">
            <button type="button" disabled={working} on:click={testTargetForm}>
              {t("testConnection", $preferencesStore.language)}
            </button>
            <button type="button" on:click={() => (targetFormOpen = false)}>
              {t("cancel", $preferencesStore.language)}
            </button>
            <button type="button" class="primary" disabled={saving} on:click={saveTarget}>
              {t("saveConfig", $preferencesStore.language)}
            </button>
          </div>
        </div>
      {/if}
    </section>

    <section class="settings-section">
      <div class="settings-section__header">
        <h3>{t("backupTasks", $preferencesStore.language)}</h3>
        <button type="button" disabled={targets.length === 0} on:click={startCreateTask}>
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
                <button type="button" disabled={working || task.target_ids.length === 0} on:click={() => runTask(task)}>
                  {t("backupNow", $preferencesStore.language)}
                </button>
                <button type="button" on:click={() => startEditTask(task)}>
                  {t("edit", $preferencesStore.language)}
                </button>
                <button type="button" class="danger" disabled={working} on:click={() => removeTask(task)}>
                  {t("delete", $preferencesStore.language)}
                </button>
              </div>
            </article>
          {/each}
        </div>
      {/if}

      {#if taskFormOpen}
        <div class="backup-editor">
          <h3>
            {editingTaskID === null
              ? t("addBackupTask", $preferencesStore.language)
              : t("editBackupTask", $preferencesStore.language)}
          </h3>
          <div class="settings-form settings-form--grid">
            <label>
              <span>{t("backupTaskName", $preferencesStore.language)}</span>
              <input bind:value={taskForm.name} type="text" />
            </label>
            <label>
              <span>{t("backupSchedule", $preferencesStore.language)}</span>
              <select bind:value={taskForm.schedule}>
                <option value="manual">{t("backupManual", $preferencesStore.language)}</option>
                <option value="daily">{t("backupDaily", $preferencesStore.language)}</option>
                <option value="weekly">{t("backupWeekly", $preferencesStore.language)}</option>
                <option value="monthly">{t("backupMonthly", $preferencesStore.language)}</option>
              </select>
            </label>
            <label class="settings-switch-row">
              <span>{t("enabled", $preferencesStore.language)}</span>
              <input bind:checked={taskForm.enabled} type="checkbox" />
            </label>
            <label>
              <span>{t("backupRetentionDays", $preferencesStore.language)}</span>
              <small>{t("backupRetentionHelp", $preferencesStore.language)}</small>
              <input bind:value={taskForm.retention_days} min="0" type="number" />
            </label>
            <label>
              <span>{t("backupMaxCount", $preferencesStore.language)}</span>
              <small>{t("backupMaxCountHelp", $preferencesStore.language)}</small>
              <input bind:value={taskForm.max_count} min="0" type="number" />
            </label>
          </div>
          <div class="backup-target-picker" aria-label={t("selectedTargets", $preferencesStore.language)}>
            <strong>{t("selectedTargets", $preferencesStore.language)} · {selectedTargetCount}</strong>
            {#each targets as target (target.id)}
              <label>
                <input
                  type="checkbox"
                  checked={taskForm.target_ids.includes(target.id)}
                  on:change={(event) => toggleTaskTarget(target.id, event.currentTarget.checked)}
                />
                <span>{target.name} · {targetTypeLabel(target.type)}</span>
              </label>
            {/each}
          </div>
          <div class="settings-actions">
            <button type="button" on:click={() => (taskFormOpen = false)}>
              {t("cancel", $preferencesStore.language)}
            </button>
            <button
              type="button"
              class="primary"
              disabled={saving || selectedTargetCount === 0}
              on:click={saveTask}
            >
              {t("saveSettings", $preferencesStore.language)}
            </button>
          </div>
        </div>
      {/if}
    </section>
  {/if}
</div>
