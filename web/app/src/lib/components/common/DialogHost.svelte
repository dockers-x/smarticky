<script lang="ts">
  import { tick } from "svelte";
  import { X } from "@lucide/svelte";
  import { dialog } from "../../actions/dialog";
  import { confirmRequest, inputRequest, notifications } from "../../stores/dialogs";
  import { preferencesStore } from "../../stores/preferences";

  let inputValue = "";
  let inputError = "";
  let inputElement: HTMLInputElement | null = null;

  $: if ($inputRequest) {
    inputValue = $inputRequest.initialValue ?? "";
    inputError = "";
    void focusInput();
  }

  async function focusInput(): Promise<void> {
    await tick();
    if (document.activeElement === inputElement) inputElement?.select();
  }

  function resolveConfirm(value: boolean): void {
    const request = $confirmRequest;
    confirmRequest.set(null);
    request?.resolve(value);
  }

  function resolveInput(value: string | null): void {
    const request = $inputRequest;
    inputRequest.set(null);
    inputValue = "";
    inputError = "";
    request?.resolve(value);
  }

  function submitInput(): void {
    if (!$inputRequest) return;

    const value = inputValue.trim();
    if (!value) {
      inputError = $inputRequest.requiredMessage;
      inputElement?.focus();
      return;
    }

    resolveInput(value);
  }
</script>

{#if $confirmRequest}
  {#key $confirmRequest}
  <div class="dialog-backdrop" role="presentation">
    <div
      class="confirm-dialog"
      role="dialog"
      aria-modal="true"
      aria-labelledby="confirm-dialog-title"
      aria-describedby="confirm-dialog-message"
      tabindex="-1"
      use:dialog={{ onDismiss: () => resolveConfirm(false), initialFocus: "[data-dialog-cancel]" }}
    >
      <h2 id="confirm-dialog-title">{$confirmRequest.title}</h2>
      <p id="confirm-dialog-message">{$confirmRequest.message}</p>
      <div class="confirm-dialog__actions">
        <button type="button" data-dialog-cancel on:click={() => resolveConfirm(false)}>
          {$confirmRequest.cancelLabel}
        </button>
        <button
          class="danger"
          type="button"
          on:click={() => resolveConfirm(true)}
        >
          {$confirmRequest.confirmLabel}
        </button>
      </div>
    </div>
  </div>
  {/key}
{/if}

{#if $inputRequest}
  {#key $inputRequest}
  <div class="dialog-backdrop" role="presentation">
    <div
      class="confirm-dialog input-dialog"
      role="dialog"
      aria-modal="true"
      aria-labelledby="input-dialog-title"
      aria-describedby={$inputRequest.message ? "input-dialog-message" : undefined}
      tabindex="-1"
      use:dialog={{ onDismiss: () => resolveInput(null), initialFocus: "input" }}
    >
      <form on:submit|preventDefault={submitInput}>
        <h2 id="input-dialog-title">{$inputRequest.title}</h2>
        {#if $inputRequest.message}
          <p id="input-dialog-message">{$inputRequest.message}</p>
        {/if}
        <label class="input-dialog__field">
          <span>{$inputRequest.label}</span>
          <input
            bind:this={inputElement}
            bind:value={inputValue}
            type="text"
            enterkeyhint="done"
            autocomplete="off"
            placeholder={$inputRequest.placeholder}
            aria-invalid={inputError ? "true" : "false"}
            aria-describedby={inputError ? "input-dialog-error" : undefined}
          />
        </label>
        {#if inputError}
          <p class="input-dialog__error" id="input-dialog-error" role="alert">{inputError}</p>
        {/if}
        <div class="confirm-dialog__actions">
          <button type="button" on:click={() => resolveInput(null)}>
            {$inputRequest.cancelLabel}
          </button>
          <button class="primary" type="submit">
            {$inputRequest.confirmLabel}
          </button>
        </div>
      </form>
    </div>
  </div>
  {/key}
{/if}

{#if $notifications.length}
  <div class="notification-stack">
    {#each $notifications as item (item.id)}
      <div class:tone-error={item.tone === "error"} class:tone-success={item.tone === "success"} class="notification" role={item.tone === "error" ? "alert" : "status"} aria-atomic="true">
        <span>{item.message}</span>
        <button
          class="notification__dismiss"
          type="button"
          aria-label={$preferencesStore.language === "zh" ? "关闭通知" : "Dismiss notification"}
          on:click={() => notifications.update((items) => items.filter((entry) => entry.id !== item.id))}
        >
          <X size={16} strokeWidth={1.8} aria-hidden="true" />
        </button>
      </div>
    {/each}
  </div>
{/if}
