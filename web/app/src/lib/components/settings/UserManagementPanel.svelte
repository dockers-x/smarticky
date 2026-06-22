<script lang="ts">
  import { onMount } from "svelte";
  import {
    createUser,
    deleteUser,
    listUsers,
    type ManagedUser,
  } from "../../api/users";
  import type { User } from "../../api/types";
  import { confirmDialog, notify } from "../../stores/dialogs";
  import { preferencesStore, t } from "../../stores/preferences";

  export let user: User | null = null;

  let users: ManagedUser[] = [];
  let loading = true;
  let creating = false;
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
    if (!username.trim() || !password) {
      notify(t("usernamePasswordRequired", $preferencesStore.language), "error");
      return;
    }
    if (password.length < 6) {
      notify(t("passwordTooShort", $preferencesStore.language), "error");
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
    } catch (createError) {
      notify(
        createError instanceof Error
          ? createError.message
          : t("createUserFailed", $preferencesStore.language),
        "error",
      );
    } finally {
      creating = false;
    }
  }

  async function deleteManagedUser(target: ManagedUser): Promise<void> {
    const confirmed = await confirmDialog({
      title: t("deleteUser", $preferencesStore.language),
      message: `${t("deleteUserConfirm", $preferencesStore.language)} "${target.username}"?`,
      confirmLabel: t("delete", $preferencesStore.language),
      cancelLabel: t("cancel", $preferencesStore.language),
    });
    if (!confirmed) return;

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
    void loadUsers();
  });
</script>

<div class="settings-view">
  {#if user?.role !== "admin"}
    <p class="settings-empty">{t("adminOnly", $preferencesStore.language)}</p>
  {:else}
    <section class="settings-section">
      <h3>{t("createNewUser", $preferencesStore.language)}</h3>
      <div class="settings-form settings-form--grid">
        <label>
          <span>{t("username", $preferencesStore.language)}</span>
          <input
            bind:value={username}
            autocomplete="off"
            type="text"
            placeholder={t("enterUsername", $preferencesStore.language)}
          />
        </label>
        <label>
          <span>{t("email", $preferencesStore.language)}</span>
          <input
            bind:value={email}
            autocomplete="off"
            type="email"
            placeholder={t("enterEmail", $preferencesStore.language)}
          />
        </label>
        <label>
          <span>{t("nickname", $preferencesStore.language)}</span>
          <input
            bind:value={nickname}
            autocomplete="off"
            type="text"
            placeholder={t("enterNickname", $preferencesStore.language)}
          />
        </label>
        <label>
          <span>{t("password", $preferencesStore.language)}</span>
          <input
            bind:value={password}
            autocomplete="new-password"
            type="password"
            placeholder={t("enterPasswordMin", $preferencesStore.language)}
          />
        </label>
        <label>
          <span>{t("role", $preferencesStore.language)}</span>
          <select bind:value={role}>
            <option value="user">{t("userRole", $preferencesStore.language)}</option>
            <option value="admin">{t("admin", $preferencesStore.language)}</option>
          </select>
        </label>
      </div>
      <div class="settings-actions">
        <button class="primary" type="button" disabled={creating} on:click={createNewUser}>
          {t("createUser", $preferencesStore.language)}
        </button>
      </div>
    </section>

    <section class="settings-section">
      <div class="settings-section__header">
        <h3>{t("allUsers", $preferencesStore.language)}</h3>
        <button type="button" disabled={loading} on:click={loadUsers}>
          {t("refresh", $preferencesStore.language)}
        </button>
      </div>
      {#if loading}
        <p class="settings-muted">{t("loading", $preferencesStore.language)}</p>
      {:else if error}
        <p class="settings-error" role="alert">{error}</p>
      {:else if users.length === 0}
        <p class="settings-empty">{t("noUsers", $preferencesStore.language)}</p>
      {:else}
        <div class="settings-table-wrap">
          <table class="settings-table">
            <thead>
              <tr>
                <th>{t("username", $preferencesStore.language)}</th>
                <th>{t("nickname", $preferencesStore.language)}</th>
                <th>{t("email", $preferencesStore.language)}</th>
                <th>{t("role", $preferencesStore.language)}</th>
                <th>{t("createdAt", $preferencesStore.language)}</th>
                <th>{t("actions", $preferencesStore.language)}</th>
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
                        <button class="danger" type="button" on:click={() => deleteManagedUser(item)}>
                          {t("delete", $preferencesStore.language)}
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
