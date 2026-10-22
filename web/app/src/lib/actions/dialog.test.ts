import { afterEach, describe, expect, it, vi } from "vitest";
import { dialog, scrollRegion } from "./dialog";

const cleanups: Array<() => void> = [];

function createDialog(content = '<button>Cancel</button><input /><button>Save</button>') {
  const backdrop = document.createElement("div");
  const panel = document.createElement("div");
  panel.setAttribute("role", "dialog");
  panel.innerHTML = content;
  backdrop.append(panel);
  document.body.append(backdrop);
  return { backdrop, panel };
}

function pressKey(key: string, shiftKey = false): KeyboardEvent {
  const event = new KeyboardEvent("keydown", { key, shiftKey, bubbles: true, cancelable: true });
  document.activeElement?.dispatchEvent(event);
  return event;
}

async function flushFocus(): Promise<void> {
  await Promise.resolve();
}

afterEach(async () => {
  for (const cleanup of cleanups.splice(0).reverse()) cleanup();
  await flushFocus();
  document.body.innerHTML = "";
});

describe("dialog action", () => {
  it("makes overflowing table regions keyboard-scrollable without adding unnecessary tab stops", () => {
    const region = document.createElement("div");
    Object.defineProperties(region, {
      scrollWidth: { value: 800 },
      clientWidth: { value: 320 },
    });
    const action = scrollRegion(region);
    expect(region.tabIndex).toBe(0);
    action.destroy();
    expect(region.hasAttribute("tabindex")).toBe(false);

    const fitted = document.createElement("div");
    const fittedAction = scrollRegion(fitted);
    expect(fitted.tabIndex).toBe(-1);
    fittedAction.destroy();
  });

  it("focuses the requested field and restores focus to the opener", async () => {
    const opener = document.createElement("button");
    document.body.append(opener);
    opener.focus();
    const { panel } = createDialog();
    const action = dialog(panel, { onDismiss: vi.fn(), initialFocus: "input" });
    cleanups.push(action.destroy);
    await flushFocus();
    expect(document.activeElement).toBe(panel.querySelector("input"));

    cleanups.pop()?.();
    panel.parentElement?.remove();
    await flushFocus();
    expect(document.activeElement).toBe(opener);
  });

  it("wraps Tab and Shift+Tab while ignoring disabled and hidden controls", async () => {
    const { panel } = createDialog('<button>Cancel</button><button disabled>Disabled</button><div hidden><button>Hidden</button></div><div style="display: none"><button>Hidden by CSS</button></div><button>Save</button>');
    cleanups.push(dialog(panel, { onDismiss: vi.fn() }).destroy);
    await flushFocus();
    const buttons = panel.querySelectorAll("button");
    expect(document.activeElement).toBe(buttons[0]);
    expect(pressKey("Tab", true).defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(buttons[4]);
    expect(pressKey("Tab").defaultPrevented).toBe(true);
    expect(document.activeElement).toBe(buttons[0]);
  });

  it("contains focus even with no available controls or programmatic outside focus", async () => {
    const outside = document.createElement("button");
    document.body.append(outside);
    const { panel } = createDialog('<button disabled>Saving</button>');
    cleanups.push(dialog(panel, { onDismiss: vi.fn() }).destroy);
    await flushFocus();
    expect(document.activeElement).toBe(panel);
    outside.focus();
    expect(document.activeElement).toBe(panel);
    expect(pressKey("Tab").defaultPrevented).toBe(true);
  });

  it("dismisses only the topmost dialog and returns to the parent control", async () => {
    const outerDismiss = vi.fn();
    const innerDismiss = vi.fn();
    const outer = createDialog();
    cleanups.push(dialog(outer.panel, { onDismiss: outerDismiss }).destroy);
    await flushFocus();
    const parentControl = outer.panel.querySelector("input")!;
    parentControl.focus();
    const inner = createDialog();
    const innerAction = dialog(inner.panel, { onDismiss: innerDismiss });
    cleanups.push(innerAction.destroy);
    await flushFocus();

    expect(pressKey("Escape").defaultPrevented).toBe(true);
    expect(innerDismiss).toHaveBeenCalledOnce();
    expect(outerDismiss).not.toHaveBeenCalled();

    cleanups.pop()?.();
    inner.backdrop.remove();
    await flushFocus();
    expect(document.activeElement).toBe(parentControl);
    pressKey("Escape");
    expect(outerDismiss).toHaveBeenCalledOnce();
  });

  it("does not override an Escape handled inside the dialog or IME composition", async () => {
    const onDismiss = vi.fn();
    const { panel } = createDialog();
    cleanups.push(dialog(panel, { onDismiss }).destroy);
    await flushFocus();
    panel.addEventListener("keydown", (event) => event.preventDefault(), { once: true });
    pressKey("Escape");
    document.activeElement?.dispatchEvent(new KeyboardEvent("keydown", {
      key: "Escape", bubbles: true, isComposing: true,
    }));
    expect(onDismiss).not.toHaveBeenCalled();
  });

  it("only dismisses a backdrop click that also began on the backdrop", async () => {
    const onDismiss = vi.fn();
    const { panel, backdrop } = createDialog();
    cleanups.push(dialog(panel, { onDismiss }).destroy);
    await flushFocus();
    panel.dispatchEvent(new MouseEvent("pointerdown", { button: 0, bubbles: true }));
    backdrop.click();
    expect(onDismiss).not.toHaveBeenCalled();

    backdrop.dispatchEvent(new MouseEvent("pointerdown", { button: 0, bubbles: true }));
    backdrop.click();
    expect(onDismiss).toHaveBeenCalledOnce();

    backdrop.dispatchEvent(new MouseEvent("pointerdown", { button: 0, bubbles: true }));
    backdrop.dispatchEvent(new Event("pointercancel"));
    backdrop.click();
    expect(onDismiss).toHaveBeenCalledOnce();
  });

  it("allows dismissal to be disabled during a destructive request", async () => {
    const onDismiss = vi.fn();
    const { panel, backdrop } = createDialog();
    const action = dialog(panel, { onDismiss, dismissible: false });
    cleanups.push(action.destroy);
    await flushFocus();
    pressKey("Escape");
    backdrop.dispatchEvent(new MouseEvent("pointerdown", { button: 0, bubbles: true }));
    backdrop.click();
    expect(onDismiss).not.toHaveBeenCalled();

    action.update({ onDismiss, dismissible: true });
    pressKey("Escape");
    expect(onDismiss).toHaveBeenCalledOnce();
  });

  it("falls back to the parent dialog when the original control disappears", async () => {
    const outer = createDialog();
    cleanups.push(dialog(outer.panel, { onDismiss: vi.fn() }).destroy);
    await flushFocus();
    const inner = createDialog();
    const action = dialog(inner.panel, { onDismiss: vi.fn() });
    cleanups.push(action.destroy);
    await flushFocus();
    outer.panel.innerHTML = "";
    cleanups.pop()?.();
    inner.backdrop.remove();
    await flushFocus();
    expect(document.activeElement).toBe(outer.panel);
  });
});
