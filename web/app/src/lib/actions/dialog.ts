export interface DialogOptions {
  onDismiss: () => void;
  initialFocus?: string | HTMLElement | null;
  dismissible?: boolean;
}

const openDialogs: HTMLElement[] = [];
const focusableSelector =
  'a[href], button, input:not([type="hidden"]), select, textarea, [tabindex], [contenteditable="true"]';

function isAvailable(element: HTMLElement): boolean {
  if (
    element.matches(":disabled") ||
    element.closest("[hidden], [inert]")
  ) {
    return false;
  }

  for (let parent: HTMLElement | null = element; parent; parent = parent.parentElement) {
    const style = getComputedStyle(parent);
    if (style.display === "none" || style.visibility === "hidden") return false;
  }
  return true;
}

function canFocus(element: HTMLElement): boolean {
  return element.tabIndex >= 0 && isAvailable(element);
}

export function scrollRegion(node: HTMLElement) {
  const previousTabIndex = node.getAttribute("tabindex");
  const update = () => {
    node.tabIndex = node.scrollWidth > node.clientWidth ? 0 : -1;
  };
  const observer = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(update);
  observer?.observe(node);
  if (node.firstElementChild) observer?.observe(node.firstElementChild);
  update();
  return {
    destroy() {
      observer?.disconnect();
      if (previousTabIndex === null) node.removeAttribute("tabindex");
      else node.setAttribute("tabindex", previousTabIndex);
    },
  };
}

export function dialog(node: HTMLElement, initialOptions: DialogOptions) {
  let options = initialOptions;
  let lastFocused: HTMLElement | null = null;
  let backdropPressed = false;
  const previousFocus = document.activeElement;
  const previousTabIndex = node.getAttribute("tabindex");
  const backdrop = node.parentElement;

  if (previousTabIndex === null) node.tabIndex = -1;
  openDialogs.push(node);

  function isTopDialog(): boolean {
    return openDialogs.at(-1) === node;
  }

  function focusableElements(): HTMLElement[] {
    return Array.from(node.querySelectorAll<HTMLElement>(focusableSelector)).filter(canFocus);
  }

  function focusInside(): void {
    const requested = typeof options.initialFocus === "string"
      ? node.querySelector<HTMLElement>(options.initialFocus)
      : options.initialFocus;
    const target = lastFocused && node.contains(lastFocused) && canFocus(lastFocused)
      ? lastFocused
      : requested && node.contains(requested) && isAvailable(requested)
        ? requested
        : focusableElements()[0] ?? node;
    target.focus({ preventScroll: true });
  }

  function handleKeydown(event: KeyboardEvent): void {
    if (!isTopDialog() || event.defaultPrevented || event.isComposing) return;

    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      if (options.dismissible !== false) options.onDismiss();
      return;
    }
    if (event.key !== "Tab") return;

    const elements = focusableElements();
    const first = elements[0];
    const last = elements.at(-1);
    const active = document.activeElement;

    if (!first || !last) {
      event.preventDefault();
      node.focus({ preventScroll: true });
    } else if (event.shiftKey && (active === first || !elements.includes(active as HTMLElement))) {
      event.preventDefault();
      last.focus({ preventScroll: true });
    } else if (!event.shiftKey && (active === last || !elements.includes(active as HTMLElement))) {
      event.preventDefault();
      first.focus({ preventScroll: true });
    }
  }

  function handleFocus(event: FocusEvent): void {
    if (!isTopDialog()) return;
    if (event.target instanceof HTMLElement && node.contains(event.target)) {
      lastFocused = event.target;
    } else {
      focusInside();
    }
  }

  function handlePointerdown(event: PointerEvent): void {
    backdropPressed = event.target === backdrop && event.button === 0 && event.isPrimary !== false;
  }

  function handlePointercancel(): void {
    backdropPressed = false;
  }

  function handleBackdropClick(event: MouseEvent): void {
    const shouldDismiss = backdropPressed && event.target === backdrop;
    backdropPressed = false;
    if (!shouldDismiss || !isTopDialog()) return;
    event.stopPropagation();
    if (options.dismissible !== false) options.onDismiss();
  }

  document.addEventListener("keydown", handleKeydown);
  document.addEventListener("focusin", handleFocus);
  backdrop?.addEventListener("pointerdown", handlePointerdown);
  backdrop?.addEventListener("pointercancel", handlePointercancel);
  backdrop?.addEventListener("click", handleBackdropClick);

  queueMicrotask(() => {
    if (node.isConnected && isTopDialog()) focusInside();
  });

  return {
    update(nextOptions: DialogOptions) {
      options = nextOptions;
    },
    destroy() {
      const wasTop = isTopDialog();
      const index = openDialogs.indexOf(node);
      if (index !== -1) openDialogs.splice(index, 1);
      document.removeEventListener("keydown", handleKeydown);
      document.removeEventListener("focusin", handleFocus);
      backdrop?.removeEventListener("pointerdown", handlePointerdown);
      backdrop?.removeEventListener("pointercancel", handlePointercancel);
      backdrop?.removeEventListener("click", handleBackdropClick);
      if (previousTabIndex === null) node.removeAttribute("tabindex");

      if (!wasTop) return;
      queueMicrotask(() => {
        const parentDialog = openDialogs.at(-1);
        if (
          previousFocus instanceof HTMLElement &&
          previousFocus.isConnected &&
          isAvailable(previousFocus) &&
          (!parentDialog || parentDialog.contains(previousFocus))
        ) {
          previousFocus.focus({ preventScroll: true });
        } else if (parentDialog && !parentDialog.contains(document.activeElement)) {
          parentDialog.focus({ preventScroll: true });
        }
      });
    },
  };
}
