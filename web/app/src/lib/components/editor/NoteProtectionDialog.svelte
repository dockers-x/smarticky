<script lang="ts">
  import { onMount } from "svelte";
  import type { ProtectionMode } from "../../api/types";
  import { preferencesStore, t } from "../../stores/preferences";
  import PasswordField from "../common/PasswordField.svelte";

  export let currentMode: ProtectionMode = "none";
  export let busy = false;
  export let error = "";
  export let onClose: () => void;
  export let onSave: (mode: ProtectionMode, password: string) => void | Promise<void>;

  let selectedMode: ProtectionMode = currentMode;
  let password = "";
  let validationError = "";
  let dialogElement: HTMLDivElement | null = null;

  $: requiresPassword = selectedMode === "password" || selectedMode === "encrypted";
  $: passwordError = requiresPassword && validationError ? validationError : "";
  $: visibleError = error;
  $: if (password.trim()) validationError = "";

  async function submit(): Promise<void> {
    if (requiresPassword && !password.trim()) {
      validationError = t("noteProtectionPasswordRequired", $preferencesStore.language);
      return;
    }
    await onSave(selectedMode, password);
  }

  function focusableElements(): HTMLElement[] {
    if (!dialogElement) return [];
    return Array.from(
      dialogElement.querySelectorAll<HTMLElement>(
        'button:not([disabled]), input:not([disabled]), [href], [tabindex]:not([tabindex="-1"])',
      ),
    ).filter((element) => !element.hasAttribute("hidden"));
  }

  function handleKeydown(event: KeyboardEvent): void {
    if (event.key === "Escape" && !busy) {
      event.preventDefault();
      onClose();
      return;
    }
    if (event.key !== "Tab") return;

    const elements = focusableElements();
    if (elements.length === 0) return;

    const first = elements[0];
    const last = elements[elements.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }

  onMount(() => {
    dialogElement?.focus();
  });
</script>

<svelte:window on:keydown={handleKeydown} />

<div class="note-protection-backdrop">
  <div
    bind:this={dialogElement}
    class="note-protection-dialog"
    role="dialog"
    aria-modal="true"
    aria-labelledby="note-protection-title"
    aria-describedby="note-protection-description"
    tabindex="-1"
  >
    <header class="note-protection-dialog__header">
      <div>
        <h2 id="note-protection-title">{t("noteProtection", $preferencesStore.language)}</h2>
        <p id="note-protection-description">{t("noteProtectionHint", $preferencesStore.language)}</p>
      </div>
      <button type="button" aria-label={t("cancel", $preferencesStore.language)} on:click={onClose}>
        ×
      </button>
    </header>

    <form class="note-protection-dialog__body" on:submit|preventDefault={() => void submit()}>
      <label class:active={selectedMode === "none"} class="note-protection-option">
        <input bind:group={selectedMode} type="radio" value="none" disabled={busy} />
        <span>
          <strong>{t("noteProtectionNone", $preferencesStore.language)}</strong>
          <small>{t("noteProtectionNoneHint", $preferencesStore.language)}</small>
        </span>
      </label>
      <label class:active={selectedMode === "password"} class="note-protection-option">
        <input bind:group={selectedMode} type="radio" value="password" disabled={busy} />
        <span>
          <strong>{t("noteProtectionPassword", $preferencesStore.language)}</strong>
          <small>{t("noteProtectionPasswordHint", $preferencesStore.language)}</small>
        </span>
      </label>
      <label class:active={selectedMode === "encrypted"} class="note-protection-option">
        <input bind:group={selectedMode} type="radio" value="encrypted" disabled={busy} />
        <span>
          <strong>{t("noteProtectionEncrypted", $preferencesStore.language)}</strong>
          <small>{t("noteProtectionEncryptedHint", $preferencesStore.language)}</small>
        </span>
      </label>

      {#if requiresPassword}
        <PasswordField
          bind:value={password}
          label={selectedMode === "encrypted"
            ? t("noteEncryptionPassword", $preferencesStore.language)
            : t("noteAccessPassword", $preferencesStore.language)}
          placeholder={t("noteProtectionPasswordPlaceholder", $preferencesStore.language)}
          error={passwordError}
          disabled={busy}
          autocomplete="new-password"
          showPasswordLabel={t("showPassword", $preferencesStore.language)}
          hidePasswordLabel={t("hidePassword", $preferencesStore.language)}
        />
      {/if}

      {#if visibleError}
        <p class="note-protection-error" role="alert">{visibleError}</p>
      {/if}

      <footer class="note-protection-dialog__footer">
        <button type="button" disabled={busy} on:click={onClose}>
          {t("cancel", $preferencesStore.language)}
        </button>
        <button class="primary" type="submit" disabled={busy}>
          {busy ? t("saving", $preferencesStore.language) : t("saveSettings", $preferencesStore.language)}
        </button>
      </footer>
    </form>
  </div>
</div>
