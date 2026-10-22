<script lang="ts">
  import { Eye, EyeOff } from "@lucide/svelte";
  import { tick } from "svelte";

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
  export let name = "";
  export let required = false;
  export let minlength: number | undefined = undefined;
  export let describedBy = "";
  export let showPasswordLabel = "Show password";
  export let hidePasswordLabel = "Hide password";

  let visible = false;
  let inputElement: HTMLInputElement;
  const generatedID = `password-${Math.random().toString(36).slice(2)}`;
  $: fieldID = id || generatedID;
  $: description = [describedBy, error ? `${fieldID}-error` : ""].filter(Boolean).join(" ") || undefined;

  async function toggleVisibility(): Promise<void> {
    const wasFocused = document.activeElement === inputElement;
    const selectionStart = inputElement.selectionStart;
    const selectionEnd = inputElement.selectionEnd;
    visible = !visible;
    await tick();
    if (wasFocused) {
      inputElement.focus({ preventScroll: true });
      inputElement.setSelectionRange(selectionStart, selectionEnd);
    }
  }
</script>

<div class="password-field">
  {#if label}
    <label class="password-field__label" for={fieldID}>{label}</label>
  {/if}
  <div class:error={Boolean(error)} class="password-field__control">
    <input
      id={fieldID}
      bind:this={inputElement}
      bind:value
      type={visible ? "text" : "password"}
      {placeholder}
      {disabled}
      {autocomplete}
      {name}
      {required}
      {minlength}
      aria-label={label ? undefined : placeholder || "Password"}
      aria-invalid={Boolean(error)}
      aria-describedby={description}
    />
    <button
      class="password-field__toggle"
      type="button"
      aria-label={visible ? hidePasswordLabel : showPasswordLabel}
      aria-controls={fieldID}
      aria-pressed={visible}
      disabled={disabled}
      on:pointerdown={(event) => {
        if (event.button === 0 && document.activeElement === inputElement) event.preventDefault();
      }}
      on:click={() => void toggleVisibility()}
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
