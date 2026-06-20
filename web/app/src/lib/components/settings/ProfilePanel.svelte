<script lang="ts">
  import type { User } from "../../api/types";
  import { updatePassword, updateUser, uploadAvatar } from "../../api/users";
  import { authStore } from "../../stores/auth";
  import { notify } from "../../stores/dialogs";
  import { preferencesStore, t } from "../../stores/preferences";

  export let user: User | null = null;

  let activeUserID = 0;
  let email = "";
  let nickname = "";
  let oldPassword = "";
  let newPassword = "";
  let confirmPassword = "";
  let avatarBusy = false;
  let profileBusy = false;
  let passwordBusy = false;

  $: if (user && user.id !== activeUserID) {
    activeUserID = user.id;
    email = user.email ?? "";
    nickname = user.nickname ?? "";
    oldPassword = "";
    newPassword = "";
    confirmPassword = "";
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
  {:else}
    <p class="settings-empty">{t("sessionExpired", $preferencesStore.language)}</p>
  {/if}
</div>
