<script lang="ts">
  import { scrollRegion } from "../../actions/dialog";
  import {
    createMCPToken,
    deleteMCPToken,
    listMCPTokens,
    type MCPToken,
  } from "../../api/mcp";
  import type { User } from "../../api/types";
  import { updatePassword, updateUser, uploadAvatar } from "../../api/users";
  import { authStore } from "../../stores/auth";
  import { confirmDialog, notify } from "../../stores/dialogs";
  import PasswordField from "../common/PasswordField.svelte";
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
  let mcpConfirming = false;
  let mcpLoading = false;
  let avatarError = "";
  let profileError = "";
  let passwordError = "";
  let mcpError = "";
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
    if (!user || avatarBusy) return;
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0] ?? null;
    if (!file) return;
    avatarError = "";

    if (!file.type.startsWith("image/")) {
      avatarError = t("imageFileRequired", $preferencesStore.language);
      input.value = "";
      return;
    }

    if (file.size > 2 * 1024 * 1024) {
      avatarError = t("avatarFileTooLarge", $preferencesStore.language);
      input.value = "";
      return;
    }

    avatarBusy = true;
    try {
      const result = await uploadAvatar(user.id, file);
      updateCurrentUser({ avatar: result.avatar });
      notify(t("avatarUploaded", $preferencesStore.language), "success");
    } catch (error) {
      avatarError = error instanceof Error
          ? error.message
          : t("avatarUploadFailed", $preferencesStore.language);
    } finally {
      avatarBusy = false;
      input.value = "";
    }
  }

  async function saveProfile(): Promise<void> {
    if (!user || profileBusy) return;
    profileError = "";

    if (email.trim() && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email.trim())) {
      profileError = t("emailInvalid", $preferencesStore.language);
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
      profileError = error instanceof Error
          ? error.message
          : t("profileSaveFailed", $preferencesStore.language);
    } finally {
      profileBusy = false;
    }
  }

  async function changePassword(): Promise<void> {
    if (!user || passwordBusy) return;
    passwordError = "";

    if (!oldPassword || !newPassword || !confirmPassword) {
      passwordError = t("passwordRequired", $preferencesStore.language);
      return;
    }
    if (newPassword.length < 6) {
      passwordError = t("passwordTooShort", $preferencesStore.language);
      return;
    }
    if (newPassword !== confirmPassword) {
      passwordError = t("passwordNotMatch", $preferencesStore.language);
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
      passwordError = error instanceof Error
          ? error.message
          : t("passwordChangeFailed", $preferencesStore.language);
    } finally {
      passwordBusy = false;
    }
  }

  async function loadMcpTokens(): Promise<void> {
    if (!user) return;
    mcpLoading = true;
    mcpError = "";
    try {
      mcpTokens = await listMCPTokens();
    } catch (error) {
      mcpError = error instanceof Error
          ? error.message
          : t("mcpTokenLoadFailed", $preferencesStore.language);
    } finally {
      mcpLoading = false;
    }
  }

  async function createToken(): Promise<void> {
    if (!user || mcpBusy || mcpLoading || mcpConfirming) return;
    mcpBusy = true;
    mcpError = "";
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
      mcpError = error instanceof Error
          ? error.message
          : t("mcpTokenCreateFailed", $preferencesStore.language);
    } finally {
      mcpBusy = false;
    }
  }

  async function revokeToken(token: MCPToken): Promise<void> {
    if (mcpBusy || mcpLoading || mcpConfirming) return;
    mcpConfirming = true;
    mcpError = "";
    try {
      const confirmed = await confirmDialog({
        title: t("delete", $preferencesStore.language),
        message: t("mcpTokenDeleteConfirm", $preferencesStore.language),
        confirmLabel: t("delete", $preferencesStore.language),
        cancelLabel: t("cancel", $preferencesStore.language),
      });
      if (!confirmed) return;
      mcpBusy = true;
      await deleteMCPToken(token.id);
      mcpTokens = mcpTokens.filter((item) => item.id !== token.id);
      notify(t("mcpTokenDeleted", $preferencesStore.language), "success");
    } catch (error) {
      mcpError = error instanceof Error
          ? error.message
          : t("mcpTokenDeleteFailed", $preferencesStore.language);
    } finally {
      mcpBusy = false;
      mcpConfirming = false;
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
            aria-describedby="profile-avatar-hint"
            on:change={handleAvatarChange}
          />
        </label>
      </div>
      <p class="settings-muted" id="profile-avatar-hint">{t("avatarHint", $preferencesStore.language)}</p>
      {#if avatarError}<p class="settings-error" role="alert">{avatarError}</p>{/if}
    </section>

    <section class="settings-section">
      <h3>{t("personalProfile", $preferencesStore.language)}</h3>
      <form class="settings-form" aria-busy={profileBusy} on:submit|preventDefault={() => void saveProfile()}>
        <label>
          <span>{t("emailAddress", $preferencesStore.language)}</span>
          <input
            bind:value={email}
            type="email"
            autocomplete="email"
            name="email"
            disabled={profileBusy}
            placeholder={t("enterEmail", $preferencesStore.language)}
          />
        </label>
        <label>
          <span>{t("nickname", $preferencesStore.language)}</span>
          <input
            bind:value={nickname}
            name="nickname"
            autocomplete="nickname"
            disabled={profileBusy}
            type="text"
            placeholder={t("enterNickname", $preferencesStore.language)}
          />
        </label>
        <label>
          <span>{t("lazycatUid", $preferencesStore.language)}</span>
          <input
            bind:value={lazycatUID}
            disabled={profileBusy}
            type="text"
            placeholder={t("lazycatUidPlaceholder", $preferencesStore.language)}
          />
          <small>{t("lazycatUidHint", $preferencesStore.language)}</small>
        </label>
        <label>
          <span>{t("shareSignature", $preferencesStore.language)}</span>
          <input
            bind:value={shareSignature}
            disabled={profileBusy}
            type="text"
            maxlength="40"
            placeholder={t("shareSignaturePlaceholder", $preferencesStore.language)}
          />
          <small>{t("shareSignatureHint", $preferencesStore.language)}</small>
        </label>
        <label>
          <span>{t("timeZone", $preferencesStore.language)}</span>
          <select bind:value={timeZone} disabled={profileBusy}>
            {#each timeZoneOptions as option}
              <option value={option}>{option}</option>
            {/each}
          </select>
          <small>{t("timeZoneHint", $preferencesStore.language)}</small>
        </label>
        <div class="settings-actions">
          <button class="primary" type="submit" disabled={profileBusy}>
            {t(profileBusy ? "saving" : "saveSettings", $preferencesStore.language)}
          </button>
        </div>
        {#if profileError}<p class="settings-error" role="alert">{profileError}</p>{/if}
      </form>
    </section>

    <section class="settings-section">
      <h3>{t("changePassword", $preferencesStore.language)}</h3>
      <form class="settings-form" aria-busy={passwordBusy} on:submit|preventDefault={() => void changePassword()}>
        <PasswordField
          bind:value={oldPassword}
          label={t("oldPassword", $preferencesStore.language)}
          autocomplete="current-password"
          placeholder={t("enterOldPassword", $preferencesStore.language)}
          disabled={passwordBusy}
          required
          describedBy={passwordError ? "profile-password-error" : ""}
          showPasswordLabel={t("showPassword", $preferencesStore.language)}
          hidePasswordLabel={t("hidePassword", $preferencesStore.language)}
        />
        <PasswordField
          bind:value={newPassword}
          label={t("newPassword", $preferencesStore.language)}
          autocomplete="new-password"
          placeholder={t("enterNewPassword", $preferencesStore.language)}
          disabled={passwordBusy}
          required
          minlength={6}
          describedBy={passwordError ? "profile-password-error" : ""}
          showPasswordLabel={t("showPassword", $preferencesStore.language)}
          hidePasswordLabel={t("hidePassword", $preferencesStore.language)}
        />
        <PasswordField
          bind:value={confirmPassword}
          label={t("confirmPassword", $preferencesStore.language)}
          autocomplete="new-password"
          placeholder={t("confirmNewPassword", $preferencesStore.language)}
          disabled={passwordBusy}
          required
          minlength={6}
          describedBy={passwordError ? "profile-password-error" : ""}
          showPasswordLabel={t("showPassword", $preferencesStore.language)}
          hidePasswordLabel={t("hidePassword", $preferencesStore.language)}
        />
        <div class="settings-actions">
          <button class="primary" type="submit" disabled={passwordBusy}>
            {t(passwordBusy ? "saving" : "changePassword", $preferencesStore.language)}
          </button>
        </div>
        {#if passwordError}<p class="settings-error" id="profile-password-error" role="alert">{passwordError}</p>{/if}
      </form>
    </section>

    <section class="settings-section">
      <div class="settings-section__header">
        <h3>{t("mcpAccess", $preferencesStore.language)}</h3>
        <button type="button" disabled={mcpBusy || mcpLoading} on:click={loadMcpTokens}>
          {t("refresh", $preferencesStore.language)}
        </button>
      </div>
      <p class="settings-muted">{t("mcpAccessHint", $preferencesStore.language)}</p>

      <form class="settings-form" aria-busy={mcpBusy} on:submit|preventDefault={() => void createToken()}>
        <label>
          <span>{t("mcpTokenName", $preferencesStore.language)}</span>
          <input
            bind:value={mcpTokenName}
            disabled={mcpBusy || mcpLoading}
            type="text"
            placeholder={t("mcpTokenNamePlaceholder", $preferencesStore.language)}
          />
        </label>
        <div class="settings-actions">
          <button class="primary" type="submit" disabled={mcpBusy || mcpLoading}>
            {t(mcpBusy ? "saving" : "mcpTokenCreate", $preferencesStore.language)}
          </button>
        </div>
      </form>
      {#if mcpError}<p class="settings-error" role="alert">{mcpError}</p>{/if}

      {#if mcpPlainToken}
        <div class="settings-secret-box">
          <span>{t("mcpTokenOneTime", $preferencesStore.language)}</span>
          <code>{mcpPlainToken}</code>
          <button type="button" on:click={copyToken}>{t("copy", $preferencesStore.language)}</button>
        </div>
      {/if}

      <div class="settings-table-wrap" role="region" aria-label={t("mcpAccess", $preferencesStore.language)} use:scrollRegion aria-busy={mcpLoading}>
        <table class="settings-table">
          <thead>
            <tr>
              <th scope="col">{t("mcpTokenName", $preferencesStore.language)}</th>
              <th scope="col">{t("createdAt", $preferencesStore.language)}</th>
              <th scope="col">{t("mcpTokenLastUsed", $preferencesStore.language)}</th>
              <th scope="col">{t("actions", $preferencesStore.language)}</th>
            </tr>
          </thead>
          <tbody>
            {#each mcpTokens as token (token.id)}
              <tr>
                <td>{token.name}</td>
                <td>{formatDate(token.created_at)}</td>
                <td>{formatDate(token.last_used_at)}</td>
                <td>
                  <button class="danger" type="button" disabled={mcpBusy || mcpLoading} aria-label={`${t("delete", $preferencesStore.language)} ${token.name}`} on:click={() => revokeToken(token)}>
                    {t("delete", $preferencesStore.language)}
                  </button>
                </td>
              </tr>
            {:else}
              <tr>
                <td colspan="4">{t(mcpLoading ? "loading" : "mcpTokenEmpty", $preferencesStore.language)}</td>
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
