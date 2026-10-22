<script lang="ts">
  import { onMount } from "svelte";
  import { scrollRegion } from "../../actions/dialog";
  import {
    createUser,
    deleteUser,
    listUsers,
    type ManagedUser,
  } from "../../api/users";
  import type { User } from "../../api/types";
  import { confirmDialog, notify } from "../../stores/dialogs";
  import { preferencesStore, t } from "../../stores/preferences";
  import PasswordField from "../common/PasswordField.svelte";

  export let user: User | null = null;

  let users: ManagedUser[] = [];
  let loading = true;
  let creating = false;
  let deletingUserID: number | null = null;
  let confirmingUserDeletion = false;
  let createError = "";
  let error = "";
  let username = "";
  let email = "";
  let nickname = "";
  let password = "";
  let role: "admin" | "user" = "user";

  async function loadUsers(): Promise<void> {
    loading = true;
    error = "";
    try {
      users = await listUsers();
    } catch (loadError) {
      error =
        loadError instanceof Error
          ? loadError.message
          : t("loadFailed", $preferencesStore.language);
    } finally {
      loading = false;
    }
  }

  function resetForm(): void {
    username = "";
    email = "";
    nickname = "";
    password = "";
    role = "user";
  }

  async function createNewUser(): Promise<void> {
    if (creating) return;
    createError = "";
    if (!username.trim() || !password) {
      createError = t("usernamePasswordRequired", $preferencesStore.language);
      return;
    }
    if (password.length < 6) {
      createError = t("passwordTooShort", $preferencesStore.language);
      return;
    }

    creating = true;
    try {
      await createUser({
        username: username.trim(),
        email: email.trim(),
        nickname: nickname.trim(),
        password,
        role,
      });
      resetForm();
      notify(t("userCreated", $preferencesStore.language), "success");
      await loadUsers();
    } catch (failure) {
      createError = failure instanceof Error ? failure.message : t("createUserFailed", $preferencesStore.language);
    } finally {
      creating = false;
    }
  }

  async function deleteManagedUser(target: ManagedUser): Promise<void> {
    if (deletingUserID !== null || confirmingUserDeletion) return;
    confirmingUserDeletion = true;
    const confirmed = await confirmDialog({
      title: t("deleteUser", $preferencesStore.language),
      message: `${t("deleteUserConfirm", $preferencesStore.language)} "${target.username}"?`,
      confirmLabel: t("delete", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    confirmingUserDeletion = false;
    if (!confirmed) return;

    deletingUserID = target.id;
    try {
      await deleteUser(target.id);
      notify(t("userDeleted", $preferencesStore.language), "success");
      await loadUsers();
    } catch (deleteError) {
      notify(
        deleteError instanceof Error
          ? deleteError.message
          : t("deleteUserFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      deletingUserID = null;
    }
  }

  function formatDate(value: string | undefined): string {
    if (!value) return "-";
    return new Date(value).toLocaleDateString(
      $preferencesStore.language === "zh" ? "zh-CN" : "en-US",
      { timeZone: $preferencesStore.timeZone },
    );
  }

  onMount(() => {
    if (user?.role === "admin") void loadUsers();
    else loading = false;
  });
</script>

<div class="settings-view">
  {#if user?.role !== "admin"}
    <p class="settings-empty">{t("adminOnly", $preferencesStore.language)}</p>
  {:else}
    <section class="settings-section">
      <h3>{t("createNewUser", $preferencesStore.language)}</h3>
      <form class="settings-form settings-form--grid" aria-busy={creating} on:submit|preventDefault={() => void createNewUser()}>
        <label>
          <span>{t("username", $preferencesStore.language)}</span>
          <input
            bind:value={username}
            name="username"
            required
            disabled={creating}
            autocomplete="off"
            type="text"
            placeholder={t("enterUsername", $preferencesStore.language)}
          />
        </label>
        <label>
          <span>{t("email", $preferencesStore.language)}</span>
          <input
            bind:value={email}
            name="email"
            disabled={creating}
            autocomplete="off"
            type="email"
            placeholder={t("enterEmail", $preferencesStore.language)}
          />
        </label>
        <label>
          <span>{t("nickname", $preferencesStore.language)}</span>
          <input
            bind:value={nickname}
            name="nickname"
            disabled={creating}
            autocomplete="off"
            type="text"
            placeholder={t("enterNickname", $preferencesStore.language)}
          />
        </label>
        <PasswordField
          bind:value={password}
          label={t("password", $preferencesStore.language)}
          autocomplete="new-password"
          placeholder={t("enterPasswordMin", $preferencesStore.language)}
          required
          minlength={6}
          disabled={creating}
          showPasswordLabel={t("showPassword", $preferencesStore.language)}
          hidePasswordLabel={t("hidePassword", $preferencesStore.language)}
        />
        <label>
          <span>{t("role", $preferencesStore.language)}</span>
          <select bind:value={role} disabled={creating}>
            <option value="user">{t("userRole", $preferencesStore.language)}</option>
            <option value="admin">{t("admin", $preferencesStore.language)}</option>
          </select>
        </label>
        <div class="settings-actions">
          <button class="primary" type="submit" disabled={creating}>
            {t(creating ? "saving" : "createUser", $preferencesStore.language)}
          </button>
        </div>
        {#if createError}<p class="settings-error" role="alert">{createError}</p>{/if}
      </form>
    </section>

    <section class="settings-section">
      <div class="settings-section__header">
        <h3>{t("allUsers", $preferencesStore.language)}</h3>
        <button type="button" disabled={loading || creating || deletingUserID !== null} on:click={loadUsers}>
          {t("refresh", $preferencesStore.language)}
        </button>
      </div>
      {#if loading}
        <p class="settings-muted" role="status">{t("loading", $preferencesStore.language)}</p>
      {:else if error}
        <p class="settings-error" role="alert">{error}</p>
      {:else if users.length === 0}
        <p class="settings-empty">{t("noUsers", $preferencesStore.language)}</p>
      {:else}
        <div class="settings-table-wrap" role="region" aria-label={t("allUsers", $preferencesStore.language)} use:scrollRegion>
          <table class="settings-table">
            <thead>
              <tr>
                <th scope="col">{t("username", $preferencesStore.language)}</th>
                <th scope="col">{t("nickname", $preferencesStore.language)}</th>
                <th scope="col">{t("email", $preferencesStore.language)}</th>
                <th scope="col">{t("role", $preferencesStore.language)}</th>
                <th scope="col">{t("createdAt", $preferencesStore.language)}</th>
                <th scope="col">{t("actions", $preferencesStore.language)}</th>
              </tr>
            </thead>
            <tbody>
              {#each users as item (item.id)}
                <tr>
                  <td>
                    <div class="user-cell">
                      <img
                        src={item.avatar || "/static/img/default-avatar.svg"}
                        alt=""
                        on:error={(event) => {
                          (event.currentTarget as HTMLImageElement).src = "/static/img/default-avatar.svg";
                        }}
                      />
                      <strong>{item.username}</strong>
                    </div>
                  </td>
                  <td>{item.nickname || "-"}</td>
                  <td>{item.email || "-"}</td>
                  <td>
                    <span class:admin={item.role === "admin"} class="role-badge">
                      {item.role === "admin"
                        ? t("admin", $preferencesStore.language)
                        : t("userRole", $preferencesStore.language)}
                    </span>
                  </td>
                  <td>{formatDate(item.created_at)}</td>
                  <td>
                    {#if item.id === user.id}
                      <span class="settings-muted">{t("currentUser", $preferencesStore.language)}</span>
                    {:else}
                      <div class="settings-row-actions">
                        <button class="danger" type="button" disabled={deletingUserID !== null} aria-label={`${t("delete", $preferencesStore.language)} ${item.username}`} on:click={() => deleteManagedUser(item)}>
                          {t(deletingUserID === item.id ? "saving" : "delete", $preferencesStore.language)}
                        </button>
                      </div>
                    {/if}
                  </td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>
  {/if}
</div>
