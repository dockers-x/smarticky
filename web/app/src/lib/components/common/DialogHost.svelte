<script lang="ts">
  import { confirmRequest, notifications } from "../../stores/dialogs";

  function resolveConfirm(value: boolean): void {
    $confirmRequest?.resolve(value);
    confirmRequest.set(null);
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

{#if $notifications.length}
  <div class="notification-stack" aria-live="polite" aria-relevant="additions">
    {#each $notifications as item (item.id)}
      <div class:tone-error={item.tone === "error"} class:tone-success={item.tone === "success"} class="notification">
        {item.message}
      </div>
    {/each}
  </div>
{/if}
