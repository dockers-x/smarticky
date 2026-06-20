<script lang="ts">
  import { onMount } from "svelte";
  import {
    getBackupConfig,
    listBackups,
    restoreBackup,
    runBackup,
    updateBackupConfig,
    verifyBackup,
    type BackupBackend,
    type BackupFileInfo,
    type BackupSchedule,
  } from "../../api/backup";
  import { confirmDialog, notify } from "../../stores/dialogs";
  import { notesStore } from "../../stores/notes";
  import { preferencesStore, t } from "../../stores/preferences";

  type BackupTab = BackupBackend | "settings";

  interface EditableBackupConfig {
    webdav_url: string;
    webdav_user: string;
    webdav_password: string;
    s3_endpoint: string;
    s3_region: string;
    s3_bucket: string;
    s3_access_key: string;
    s3_secret_key: string;
    auto_backup_enabled: boolean;
    backup_schedule: BackupSchedule;
    backup_retention_days: number;
    backup_max_count: number;
    last_backup_at: string;
  }

  const defaultConfig: EditableBackupConfig = {
    webdav_url: "",
    webdav_user: "",
    webdav_password: "",
    s3_endpoint: "",
    s3_region: "",
    s3_bucket: "",
    s3_access_key: "",
    s3_secret_key: "",
    auto_backup_enabled: false,
    backup_schedule: "daily",
    backup_retention_days: 30,
    backup_max_count: 10,
    last_backup_at: "",
  };

  let activeTab: BackupTab = "webdav";
  let config: EditableBackupConfig = { ...defaultConfig };
  let loading = true;
  let saving = false;
  let working = false;
  let error = "";
  let listMode = false;
  let listBackend: BackupBackend = "webdav";
  let backups: BackupFileInfo[] = [];
  let listLoading = false;
  let verificationText = "";

  $: tabs = [
    { id: "webdav" as BackupTab, label: "WebDAV" },
    { id: "s3" as BackupTab, label: "S3" },
    { id: "settings" as BackupTab, label: t("backupSettings", $preferencesStore.language) },
  ];

  function normalizeSchedule(value: string | undefined): BackupSchedule {
    if (value === "weekly" || value === "manual") return value;
    return "daily";
  }

  async function loadConfig(): Promise<void> {
    loading = true;
    error = "";
    try {
      const next = await getBackupConfig();
      config = {
        webdav_url: next.webdav_url ?? "",
        webdav_user: next.webdav_user ?? "",
        webdav_password: next.webdav_password ?? "",
        s3_endpoint: next.s3_endpoint ?? "",
        s3_region: next.s3_region ?? "",
        s3_bucket: next.s3_bucket ?? "",
        s3_access_key: next.s3_access_key ?? "",
        s3_secret_key: next.s3_secret_key ?? "",
        auto_backup_enabled: Boolean(next.auto_backup_enabled),
        backup_schedule: normalizeSchedule(next.backup_schedule),
        backup_retention_days: next.backup_retention_days ?? 30,
        backup_max_count: next.backup_max_count ?? 10,
        last_backup_at: next.last_backup_at ?? "",
      };
    } catch (loadError) {
      error =
        loadError instanceof Error
          ? loadError.message
          : t("loadFailed", $preferencesStore.language);
    } finally {
      loading = false;
    }
  }

  async function saveConfig(): Promise<void> {
    saving = true;
    error = "";
    try {
      await updateBackupConfig({
        webdav_url: config.webdav_url,
        webdav_user: config.webdav_user,
        webdav_password: config.webdav_password,
        s3_endpoint: config.s3_endpoint,
        s3_region: config.s3_region,
        s3_bucket: config.s3_bucket,
        s3_access_key: config.s3_access_key,
        s3_secret_key: config.s3_secret_key,
        auto_backup_enabled: config.auto_backup_enabled,
        backup_schedule: config.backup_schedule,
        backup_retention_days: Number(config.backup_retention_days) || 0,
        backup_max_count: Number(config.backup_max_count) || 0,
      });
      notify(t("backupConfigSaved", $preferencesStore.language), "success");
      await loadConfig();
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

  async function backupNow(backend: BackupBackend): Promise<void> {
    const confirmed = await confirmDialog({
      title: t("backupNow", $preferencesStore.language),
      message: `${t("backupConfirm", $preferencesStore.language)} ${backend.toUpperCase()}?`,
      confirmLabel: t("backupNow", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (!confirmed) return;

    working = true;
    try {
      const result = await runBackup(backend);
      notify(
        `${t("backupSuccess", $preferencesStore.language)}: ${result.file}`,
        "success",
      );
      await loadConfig();
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

  async function openBackupList(backend: BackupBackend): Promise<void> {
    listMode = true;
    listBackend = backend;
    listLoading = true;
    verificationText = "";
    backups = [];
    try {
      const result = await listBackups(backend);
      backups = result.backups ?? [];
    } catch (listError) {
      notify(
        listError instanceof Error
          ? listError.message
          : t("loadFailed", $preferencesStore.language),
        "error",
      );
      listMode = false;
    } finally {
      listLoading = false;
    }
  }

  async function verifyBackupFile(backup: BackupFileInfo): Promise<void> {
    const confirmed = await confirmDialog({
      title: t("backupVerify", $preferencesStore.language),
      message: `${t("backupVerifyConfirm", $preferencesStore.language)} "${backup.filename}"?`,
      confirmLabel: t("backupVerify", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (!confirmed) return;

    working = true;
    verificationText = "";
    try {
      const result = await verifyBackup(listBackend, backup.filename);
      if (result.valid) {
        verificationText = [
          t("backupVerifySuccess", $preferencesStore.language),
          `${t("backupFiles", $preferencesStore.language)}: ${result.file_count}`,
          `${t("backupTotalSize", $preferencesStore.language)}: ${formatFileSize(result.total_size)}`,
          ...result.file_checks.map((check) => {
            const marker = check.exists ? "OK" : "MISS";
            return `${marker} ${check.path}${check.error ? ` - ${check.error}` : ""}`;
          }),
        ].join("\n");
      } else {
        verificationText =
          result.error || t("backupVerifyFailed", $preferencesStore.language);
      }
    } catch (verifyError) {
      verificationText =
        verifyError instanceof Error
          ? verifyError.message
          : t("backupVerifyFailed", $preferencesStore.language);
    } finally {
      working = false;
    }
  }

  async function restoreBackupFile(backup: BackupFileInfo): Promise<void> {
    const confirmed = await confirmDialog({
      title: t("restore", $preferencesStore.language),
      message: `${t("restoreConfirm", $preferencesStore.language)} "${backup.filename}"?`,
      confirmLabel: t("restore", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (!confirmed) return;

    working = true;
    try {
      const result = await restoreBackup(listBackend, backup.filename);
      notify(
        result.warning || t("restoreSuccess", $preferencesStore.language),
        result.restart_required ? "info" : "success",
      );
      await notesStore.load();
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

  function formatFileSize(bytes: number): string {
    if (!bytes) return "0 B";
    const units = ["B", "KB", "MB", "GB"];
    const index = Math.min(
      Math.floor(Math.log(bytes) / Math.log(1024)),
      units.length - 1,
    );
    return `${Math.round((bytes / 1024 ** index) * 100) / 100} ${units[index]}`;
  }

  function formatDate(value: string): string {
    if (!value) return "-";
    return new Date(value).toLocaleString(
      $preferencesStore.language === "zh" ? "zh-CN" : "en-US",
    );
  }

  onMount(() => {
    void loadConfig();
  });
</script>

<div class="settings-view">
  {#if listMode}
    <div class="settings-actions">
      <button type="button" on:click={() => (listMode = false)}>
        {t("back", $preferencesStore.language)}
      </button>
      <button type="button" disabled={listLoading} on:click={() => openBackupList(listBackend)}>
        {t("refresh", $preferencesStore.language)}
      </button>
    </div>
    <h3>{t("backupListTitle", $preferencesStore.language)} · {listBackend.toUpperCase()}</h3>
    {#if listLoading}
      <p class="settings-muted">{t("loading", $preferencesStore.language)}</p>
    {:else if backups.length === 0}
      <p class="settings-empty">{t("backupNoFiles", $preferencesStore.language)}</p>
    {:else}
      <div class="settings-table-wrap">
        <table class="settings-table">
          <thead>
            <tr>
              <th>{t("backupFilename", $preferencesStore.language)}</th>
              <th>{t("backupSize", $preferencesStore.language)}</th>
              <th>{t("backupDate", $preferencesStore.language)}</th>
              <th>{t("actions", $preferencesStore.language)}</th>
            </tr>
          </thead>
          <tbody>
            {#each backups as backup (backup.filename)}
              <tr>
                <td title={backup.filename}>{backup.filename}</td>
                <td>{formatFileSize(backup.size)}</td>
                <td>{formatDate(backup.created_at)}</td>
                <td>
                  <div class="settings-row-actions">
                    <button type="button" disabled={working} on:click={() => verifyBackupFile(backup)}>
                      {t("backupVerify", $preferencesStore.language)}
                    </button>
                    <button type="button" disabled={working} on:click={() => restoreBackupFile(backup)}>
                      {t("restore", $preferencesStore.language)}
                    </button>
                  </div>
                </td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
    {#if verificationText}
      <pre class="settings-result">{verificationText}</pre>
    {/if}
  {:else if loading}
    <p class="settings-muted">{t("loading", $preferencesStore.language)}</p>
  {:else}
    {#if error}
      <p class="settings-error" role="alert">{error}</p>
    {/if}
    <div class="settings-tabs" role="tablist" aria-label={t("backupTitle", $preferencesStore.language)}>
      {#each tabs as tab}
        <button
          class:active={activeTab === tab.id}
          type="button"
          role="tab"
          aria-selected={activeTab === tab.id}
          on:click={() => (activeTab = tab.id)}
        >
          {tab.label}
        </button>
      {/each}
    </div>

    {#if activeTab === "webdav"}
      <div class="settings-form">
        <label>
          <span>{t("webdavUrl", $preferencesStore.language)}</span>
          <input bind:value={config.webdav_url} type="url" placeholder="https://dav.example.com/path" />
        </label>
        <label>
          <span>{t("webdavUsername", $preferencesStore.language)}</span>
          <input bind:value={config.webdav_user} type="text" autocomplete="username" />
        </label>
        <label>
          <span>{t("webdavPassword", $preferencesStore.language)}</span>
          <input bind:value={config.webdav_password} type="password" autocomplete="current-password" />
        </label>
        <div class="settings-actions">
          <button type="button" class="primary" disabled={saving} on:click={saveConfig}>
            {t("saveConfig", $preferencesStore.language)}
          </button>
          <button type="button" disabled={working} on:click={() => backupNow("webdav")}>
            {t("backupNow", $preferencesStore.language)}
          </button>
          <button type="button" disabled={listLoading} on:click={() => openBackupList("webdav")}>
            {t("backupList", $preferencesStore.language)}
          </button>
        </div>
      </div>
    {:else if activeTab === "s3"}
      <div class="settings-form">
        <label>
          <span>{t("s3Endpoint", $preferencesStore.language)}</span>
          <input bind:value={config.s3_endpoint} type="text" placeholder="https://s3.example.com" />
        </label>
        <label>
          <span>{t("s3Region", $preferencesStore.language)}</span>
          <input bind:value={config.s3_region} type="text" placeholder="us-east-1" />
        </label>
        <label>
          <span>{t("s3Bucket", $preferencesStore.language)}</span>
          <input bind:value={config.s3_bucket} type="text" />
        </label>
        <label>
          <span>{t("s3AccessKey", $preferencesStore.language)}</span>
          <input bind:value={config.s3_access_key} type="text" autocomplete="off" />
        </label>
        <label>
          <span>{t("s3SecretKey", $preferencesStore.language)}</span>
          <input bind:value={config.s3_secret_key} type="password" autocomplete="off" />
        </label>
        <div class="settings-actions">
          <button type="button" class="primary" disabled={saving} on:click={saveConfig}>
            {t("saveConfig", $preferencesStore.language)}
          </button>
          <button type="button" disabled={working} on:click={() => backupNow("s3")}>
            {t("backupNow", $preferencesStore.language)}
          </button>
          <button type="button" disabled={listLoading} on:click={() => openBackupList("s3")}>
            {t("backupList", $preferencesStore.language)}
          </button>
        </div>
      </div>
    {:else}
      <div class="settings-form">
        <label class="settings-switch-row">
          <span>
            <strong>{t("backupAutoEnable", $preferencesStore.language)}</strong>
          </span>
          <input bind:checked={config.auto_backup_enabled} type="checkbox" />
        </label>
        {#if config.auto_backup_enabled}
          <label>
            <span>{t("backupSchedule", $preferencesStore.language)}</span>
            <select bind:value={config.backup_schedule}>
              <option value="daily">{t("backupDaily", $preferencesStore.language)}</option>
              <option value="weekly">{t("backupWeekly", $preferencesStore.language)}</option>
              <option value="manual">{t("backupManual", $preferencesStore.language)}</option>
            </select>
          </label>
          <div class="settings-kv">
            <span>{t("backupLast", $preferencesStore.language)}</span>
            <strong>{formatDate(config.last_backup_at)}</strong>
          </div>
        {/if}
        <label>
          <span>{t("backupRetentionDays", $preferencesStore.language)}</span>
          <small>{t("backupRetentionHelp", $preferencesStore.language)}</small>
          <input bind:value={config.backup_retention_days} min="0" type="number" />
        </label>
        <label>
          <span>{t("backupMaxCount", $preferencesStore.language)}</span>
          <small>{t("backupMaxCountHelp", $preferencesStore.language)}</small>
          <input bind:value={config.backup_max_count} min="0" type="number" />
        </label>
        <div class="settings-actions">
          <button type="button" class="primary" disabled={saving} on:click={saveConfig}>
            {t("saveSettings", $preferencesStore.language)}
          </button>
        </div>
      </div>
    {/if}
  {/if}
</div>
