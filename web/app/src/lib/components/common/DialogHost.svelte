<script lang="ts">
  import { tick } from "svelte";
  import { confirmRequest, inputRequest, notifications } from "../../stores/dialogs";

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
    inputElement?.focus();
    inputElement?.select();
  }

  function resolveConfirm(value: boolean): void {
    $confirmRequest?.resolve(value);
    confirmRequest.set(null);
  }

  function resolveInput(value: string | null): void {
    $inputRequest?.resolve(value);
    inputRequest.set(null);
    inputValue = "";
    inputError = "";
  }

  function submitInput(): void {
    if (!$inputRequest) return;

    const value = inputValue.trim();
    if (!value) {
      inputError = $inputRequest.requiredMessage;
      return;
    }

    resolveInput(value);
  }
</script>

{#if $confirmRequest}
  <div class="dialog-backdrop" role="presentation">
    <div
      class="confirm-dialog"
      role="dialog"
      aria-modal="true"
      aria-labelledby="confirm-dialog-title"
      tabindex="-1"
    >
      <h2 id="confirm-dialog-title">{$confirmRequest.title}</h2>
      <p>{$confirmRequest.message}</p>
      <div class="confirm-dialog__actions">
        <button type="button" on:click={() => resolveConfirm(false)}>
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
{/if}

{#if $inputRequest}
  <div class="dialog-backdrop" role="presentation">
    <div
      class="confirm-dialog input-dialog"
      role="dialog"
      aria-modal="true"
      aria-labelledby="input-dialog-title"
      aria-describedby={inputError ? "input-dialog-error" : undefined}
      tabindex="-1"
      on:keydown={(event) => {
        if (event.key === "Escape") resolveInput(null);
      }}
    >
      <form on:submit|preventDefault={submitInput}>
        <h2 id="input-dialog-title">{$inputRequest.title}</h2>
        {#if $inputRequest.message}
          <p>{$inputRequest.message}</p>
        {/if}
        <label class="input-dialog__field">
          <span>{$inputRequest.label}</span>
          <input
            bind:this={inputElement}
            bind:value={inputValue}
            type="text"
            autocomplete="off"
            placeholder={$inputRequest.placeholder}
            aria-invalid={inputError ? "true" : "false"}
            aria-describedby={inputError ? "input-dialog-error" : undefined}
          />
        </label>
        {#if inputError}
          <p class="input-dialog__error" id="input-dialog-error">{inputError}</p>
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
{/if}

{#if $notifications.length}
  <div class="notification-stack" aria-live="polite" aria-relevant="additions">
    {#each $notifications as item (item.id)}
      <div class:tone-error={item.tone === "error"} class:tone-success={item.tone === "success"} class="notification">
        {item.message}
      </div>
    {/each}
  </div>
{/if}
