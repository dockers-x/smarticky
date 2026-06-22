<script lang="ts">
  import { Eye, EyeOff } from "@lucide/svelte";

  export let value = "";
  export let label = "";
  export let placeholder = "";
  export let error = "";
  export let disabled = false;
  export let autocomplete:
    | "current-password"
    | "new-password"
    | "one-time-code"
    | "off"
    | "username" = "current-password";
  export let id = "";
  export let showPasswordLabel = "Show password";
  export let hidePasswordLabel = "Hide password";

  let visible = false;
  const fieldID = id || `password-${Math.random().toString(36).slice(2)}`;
</script>

<div class="password-field">
  {#if label}
    <label class="password-field__label" for={fieldID}>{label}</label>
  {/if}
  <div class:error={Boolean(error)} class="password-field__control">
    <input
      id={fieldID}
      bind:value
      type={visible ? "text" : "password"}
      {placeholder}
      {disabled}
      {autocomplete}
      aria-invalid={Boolean(error)}
      aria-describedby={error ? `${fieldID}-error` : undefined}
    />
    <button
      class="password-field__toggle"
      type="button"
      aria-label={visible ? hidePasswordLabel : showPasswordLabel}
      disabled={disabled}
      on:click={() => (visible = !visible)}
    >
      {#if visible}
        <EyeOff size={17} strokeWidth={2} aria-hidden="true" />
      {:else}
        <Eye size={17} strokeWidth={2} aria-hidden="true" />
      {/if}
    </button>
  </div>
  {#if error}
    <p id={`${fieldID}-error`} class="password-field__error" role="alert">{error}</p>
  {/if}
</div>
