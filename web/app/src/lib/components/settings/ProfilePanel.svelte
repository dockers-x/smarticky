<script lang="ts">
  import {
    createMCPToken,
    deleteMCPToken,
    listMCPTokens,
    type MCPToken,
  } from "../../api/mcp";
  import type { User } from "../../api/types";
  import { updatePassword, updateUser, uploadAvatar } from "../../api/users";
  import { authStore } from "../../stores/auth";
  import { notify } from "../../stores/dialogs";
  import {
    preferencesStore,
    supportedTimeZones,
    t,
  } from "../../stores/preferences";

  export let user: User | null = null;

  let activeUserID = 0;
  let email = "";
  let nickname = "";
  let lazycatUID = "";
  let shareSignature = "";
  let timeZone = "";
  let oldPassword = "";
  let newPassword = "";
  let confirmPassword = "";
  let avatarBusy = false;
  let profileBusy = false;
  let passwordBusy = false;
  let mcpBusy = false;
  let mcpLoadedUserID = 0;
  let mcpTokenName = "";
  let mcpPlainToken = "";
  let mcpTokens: MCPToken[] = [];
  $: timeZoneOptions = supportedTimeZones(timeZone || user?.time_zone);

  $: if (user && user.id !== activeUserID) {
    activeUserID = user.id;
    email = user.email ?? "";
    nickname = user.nickname ?? "";
    lazycatUID = user.lazycat_uid ?? "";
    shareSignature = user.share_signature ?? "Smarticky";
    timeZone = user.time_zone || $preferencesStore.timeZone || "UTC";
    oldPassword = "";
    newPassword = "";
    confirmPassword = "";
  }

  $: if (user && user.id !== mcpLoadedUserID) {
    mcpLoadedUserID = user.id;
    mcpTokenName = "";
    mcpPlainToken = "";
    void loadMcpTokens();
  }

  function updateCurrentUser(fields: Partial<User>): void {
    if (!user) return;
    const nextUser = { ...user, ...fields };
    authStore.setUser(nextUser);
  }

  async function handleAvatarChange(event: Event): Promise<void> {
    if (!user) return;
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0] ?? null;
    if (!file) return;

    if (!file.type.startsWith("image/")) {
      notify(t("imageFileRequired", $preferencesStore.language), "error");
      input.value = "";
      return;
    }

    if (file.size > 2 * 1024 * 1024) {
      notify(t("avatarFileTooLarge", $preferencesStore.language), "error");
      input.value = "";
      return;
    }

    avatarBusy = true;
    try {
      const result = await uploadAvatar(user.id, file);
      updateCurrentUser({ avatar: result.avatar });
      notify(t("avatarUploaded", $preferencesStore.language), "success");
    } catch (error) {
      notify(
        error instanceof Error
          ? error.message
          : t("avatarUploadFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      avatarBusy = false;
      input.value = "";
    }
  }

  async function saveProfile(): Promise<void> {
    if (!user) return;

    if (email.trim() && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) {
      notify(t("emailInvalid", $preferencesStore.language), "error");
      return;
    }

    profileBusy = true;
    try {
      const updated = await updateUser(user.id, {
        email: email.trim(),
        nickname: nickname.trim(),
        lazycat_uid: lazycatUID.trim(),
        share_signature: shareSignature.trim() || "Smarticky",
        time_zone: timeZone || "UTC",
      });
      authStore.setUser(updated);
      notify(t("profileSaved", $preferencesStore.language), "success");
    } catch (error) {
      notify(
        error instanceof Error
          ? error.message
          : t("profileSaveFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      profileBusy = false;
    }
  }

  async function changePassword(): Promise<void> {
    if (!user) return;

    if (!oldPassword || !newPassword || !confirmPassword) {
      notify(t("passwordRequired", $preferencesStore.language), "error");
      return;
    }
    if (newPassword.length < 6) {
      notify(t("passwordTooShort", $preferencesStore.language), "error");
      return;
    }
    if (newPassword !== confirmPassword) {
      notify(t("passwordNotMatch", $preferencesStore.language), "error");
      return;
    }

    passwordBusy = true;
    try {
      await updatePassword(user.id, {
        old_password: oldPassword,
        new_password: newPassword,
      });
      oldPassword = "";
      newPassword = "";
      confirmPassword = "";
      notify(t("passwordChanged", $preferencesStore.language), "success");
    } catch (error) {
      notify(
        error instanceof Error
          ? error.message
          : t("passwordChangeFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      passwordBusy = false;
    }
  }

  async function loadMcpTokens(): Promise<void> {
    if (!user) return;
    mcpBusy = true;
    try {
      mcpTokens = await listMCPTokens();
    } catch (error) {
      notify(
        error instanceof Error
          ? error.message
          : t("mcpTokenLoadFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      mcpBusy = false;
    }
  }

  async function createToken(): Promise<void> {
    if (!user) return;
    mcpBusy = true;
    try {
      const created = await createMCPToken(
        mcpTokenName.trim() ||
          t("mcpTokenDefaultName", $preferencesStore.language),
      );
      mcpPlainToken = created.token;
      mcpTokenName = "";
      await loadMcpTokens();
      notify(t("mcpTokenCreated", $preferencesStore.language), "success");
    } catch (error) {
      notify(
        error instanceof Error
          ? error.message
          : t("mcpTokenCreateFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      mcpBusy = false;
    }
  }

  async function revokeToken(token: MCPToken): Promise<void> {
    if (!window.confirm(t("mcpTokenDeleteConfirm", $preferencesStore.language))) {
      return;
    }

    mcpBusy = true;
    try {
      await deleteMCPToken(token.id);
      mcpTokens = mcpTokens.filter((item) => item.id !== token.id);
      notify(t("mcpTokenDeleted", $preferencesStore.language), "success");
    } catch (error) {
      notify(
        error instanceof Error
          ? error.message
          : t("mcpTokenDeleteFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      mcpBusy = false;
    }
  }

  async function copyToken(): Promise<void> {
    if (!mcpPlainToken) return;
    try {
      await navigator.clipboard.writeText(mcpPlainToken);
      notify(t("mcpTokenCopied", $preferencesStore.language), "success");
    } catch {
      notify(t("mcpTokenCopyFailed", $preferencesStore.language), "error");
    }
  }

  function formatDate(value?: string): string {
    if (!value) return "-";
    return new Intl.DateTimeFormat(
      $preferencesStore.language === "zh" ? "zh-CN" : "en-US",
      {
        dateStyle: "medium",
        timeStyle: "short",
        timeZone: $preferencesStore.timeZone,
      },
    ).format(new Date(value));
  }
</script>

<div class="settings-view">
  {#if user}
    <section class="settings-section">
      <h3>{t("avatar", $preferencesStore.language)}</h3>
      <div class="profile-avatar-row">
        <img
          src={user.avatar || "/static/img/default-avatar.svg"}
          alt={t("avatar", $preferencesStore.language)}
          on:error={(event) => {
            (event.currentTarget as HTMLImageElement).src = "/static/img/default-avatar.svg";
          }}
        />
        <label class="settings-file-button">
          <span>{avatarBusy ? t("saving", $preferencesStore.language) : t("uploadAvatar", $preferencesStore.language)}</span>
          <input
            accept="image/*"
            disabled={avatarBusy}
            type="file"
            on:change={handleAvatarChange}
          />
        </label>
      </div>
      <p class="settings-muted">{t("avatarHint", $preferencesStore.language)}</p>
    </section>

    <section class="settings-section">
      <h3>{t("personalProfile", $preferencesStore.language)}</h3>
      <div class="settings-form">
        <label>
          <span>{t("emailAddress", $preferencesStore.language)}</span>
          <input
            bind:value={email}
            type="email"
            autocomplete="email"
            placeholder={t("enterEmail", $preferencesStore.language)}
          />
        </label>
        <label>
          <span>{t("nickname", $preferencesStore.language)}</span>
          <input
            bind:value={nickname}
            type="text"
            placeholder={t("enterNickname", $preferencesStore.language)}
          />
        </label>
        <label>
          <span>{t("lazycatUid", $preferencesStore.language)}</span>
          <input
            bind:value={lazycatUID}
            type="text"
            placeholder={t("lazycatUidPlaceholder", $preferencesStore.language)}
          />
          <small>{t("lazycatUidHint", $preferencesStore.language)}</small>
        </label>
        <label>
          <span>{t("shareSignature", $preferencesStore.language)}</span>
          <input
            bind:value={shareSignature}
            type="text"
            maxlength="40"
            placeholder={t("shareSignaturePlaceholder", $preferencesStore.language)}
          />
          <small>{t("shareSignatureHint", $preferencesStore.language)}</small>
        </label>
        <label>
          <span>{t("timeZone", $preferencesStore.language)}</span>
          <select bind:value={timeZone}>
            {#each timeZoneOptions as option}
              <option value={option}>{option}</option>
            {/each}
          </select>
          <small>{t("timeZoneHint", $preferencesStore.language)}</small>
        </label>
        <div class="settings-actions">
          <button class="primary" type="button" disabled={profileBusy} on:click={saveProfile}>
            {t("saveSettings", $preferencesStore.language)}
          </button>
        </div>
      </div>
    </section>

    <section class="settings-section">
      <h3>{t("changePassword", $preferencesStore.language)}</h3>
      <div class="settings-form">
        <label>
          <span>{t("oldPassword", $preferencesStore.language)}</span>
          <input
            bind:value={oldPassword}
            type="password"
            autocomplete="current-password"
            placeholder={t("enterOldPassword", $preferencesStore.language)}
          />
        </label>
        <label>
          <span>{t("newPassword", $preferencesStore.language)}</span>
          <input
            bind:value={newPassword}
            type="password"
            autocomplete="new-password"
            placeholder={t("enterNewPassword", $preferencesStore.language)}
          />
        </label>
        <label>
          <span>{t("confirmPassword", $preferencesStore.language)}</span>
          <input
            bind:value={confirmPassword}
            type="password"
            autocomplete="new-password"
            placeholder={t("confirmNewPassword", $preferencesStore.language)}
          />
        </label>
        <div class="settings-actions">
          <button class="primary" type="button" disabled={passwordBusy} on:click={changePassword}>
            {t("changePassword", $preferencesStore.language)}
          </button>
        </div>
      </div>
    </section>

    <section class="settings-section">
      <div class="settings-section__header">
        <h3>{t("mcpAccess", $preferencesStore.language)}</h3>
        <button type="button" disabled={mcpBusy} on:click={loadMcpTokens}>
          {t("refresh", $preferencesStore.language)}
        </button>
      </div>
      <p class="settings-muted">{t("mcpAccessHint", $preferencesStore.language)}</p>

      <div class="settings-form">
        <label>
          <span>{t("mcpTokenName", $preferencesStore.language)}</span>
          <input
            bind:value={mcpTokenName}
            type="text"
            placeholder={t("mcpTokenNamePlaceholder", $preferencesStore.language)}
          />
        </label>
        <div class="settings-actions">
          <button class="primary" type="button" disabled={mcpBusy} on:click={createToken}>
            {t("mcpTokenCreate", $preferencesStore.language)}
          </button>
        </div>
      </div>

      {#if mcpPlainToken}
        <div class="settings-secret-box">
          <span>{t("mcpTokenOneTime", $preferencesStore.language)}</span>
          <code>{mcpPlainToken}</code>
          <button type="button" on:click={copyToken}>{t("copy", $preferencesStore.language)}</button>
        </div>
      {/if}

      <div class="settings-table-wrap">
        <table class="settings-table">
          <thead>
            <tr>
              <th>{t("mcpTokenName", $preferencesStore.language)}</th>
              <th>{t("createdAt", $preferencesStore.language)}</th>
              <th>{t("mcpTokenLastUsed", $preferencesStore.language)}</th>
              <th>{t("actions", $preferencesStore.language)}</th>
            </tr>
          </thead>
          <tbody>
            {#each mcpTokens as token}
              <tr>
                <td>{token.name}</td>
                <td>{formatDate(token.created_at)}</td>
                <td>{formatDate(token.last_used_at)}</td>
                <td>
                  <button class="danger" type="button" disabled={mcpBusy} on:click={() => revokeToken(token)}>
                    {t("delete", $preferencesStore.language)}
                  </button>
                </td>
              </tr>
            {:else}
              <tr>
                <td colspan="4">{t("mcpTokenEmpty", $preferencesStore.language)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </section>
  {:else}
    <p class="settings-empty">{t("sessionExpired", $preferencesStore.language)}</p>
  {/if}
</div>
