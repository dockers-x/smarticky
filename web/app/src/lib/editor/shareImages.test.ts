import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { waitForShareImages } from "./shareImages";

function createImage(complete = false): HTMLImageElement {
  const image = document.createElement("img");
  Object.defineProperty(image, "complete", { configurable: true, value: complete });
  return image;
}

function trackListeners(target: EventTarget) {
  const added = vi.spyOn(target, "addEventListener");
  const removed = vi.spyOn(target, "removeEventListener");
  return (firedEvents: string[] = []) => {
    for (const [eventType, listener, options] of added.mock.calls) {
      const explicitlyRemoved = removed.mock.calls.some(
        ([removedType, removedListener]) =>
          removedType === eventType && removedListener === listener,
      );
      const automaticallyRemoved = typeof options === "object" && (
        (options.once && firedEvents.includes(eventType)) || options.signal?.aborted
      );
      expect(Boolean(explicitlyRemoved || automaticallyRemoved)).toBe(true);
    }
  };
}

describe("waitForShareImages", () => {
  let root: HTMLDivElement;
  let controller: AbortController;

  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(0);
    root = document.createElement("div");
    document.body.append(root);
    controller = new AbortController();
  });

  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.restoreAllMocks();
    document.body.replaceChildren();
  });

  it("returns immediately for completed images without retaining resources", async () => {
    root.append(createImage(true), createImage(true));
    const expectSignalCleaned = trackListeners(controller.signal);

    await expect(waitForShareImages(root, controller.signal)).resolves.toBeUndefined();

    expectSignalCleaned();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("waits for every pending image to load and removes its listeners", async () => {
    const firstImage = createImage();
    const secondImage = createImage();
    root.append(firstImage, secondImage);
    const expectFirstCleaned = trackListeners(firstImage);
    const expectSecondCleaned = trackListeners(secondImage);
    const expectSignalCleaned = trackListeners(controller.signal);
    let settled = false;
    const waiting = waitForShareImages(root, controller.signal).then(() => {
      settled = true;
    });
    await vi.advanceTimersByTimeAsync(0);

    firstImage.dispatchEvent(new Event("load"));
    await vi.advanceTimersByTimeAsync(0);
    expect(settled).toBe(false);

    secondImage.dispatchEvent(new Event("load"));
    await expect(waiting).resolves.toBeUndefined();

    expectFirstCleaned(["load"]);
    expectSecondCleaned(["load"]);
    expectSignalCleaned();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("accepts an image error so the export library can render its fallback", async () => {
    const image = createImage();
    root.append(image);
    const expectImageCleaned = trackListeners(image);
    const expectSignalCleaned = trackListeners(controller.signal);
    const waiting = waitForShareImages(root, controller.signal);
    await vi.advanceTimersByTimeAsync(0);

    image.dispatchEvent(new Event("error"));
    await expect(waiting).resolves.toBeUndefined();

    expectImageCleaned(["error"]);
    expectSignalCleaned();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("rejects at the default deadline and removes pending image listeners", async () => {
    const image = createImage();
    root.append(image);
    const expectImageCleaned = trackListeners(image);
    const expectSignalCleaned = trackListeners(controller.signal);
    let settled = false;
    const waiting = waitForShareImages(root, controller.signal);
    void waiting.then(() => { settled = true; }, () => { settled = true; });
    const rejected = expect(waiting).rejects.toMatchObject({ name: "Error" });

    await vi.advanceTimersByTimeAsync(4999);
    expect(settled).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    await rejected;

    expectImageCleaned();
    expectSignalCleaned();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("rejects an already aborted request without leaving timers or listeners", async () => {
    const image = createImage();
    root.append(image);
    const expectImageCleaned = trackListeners(image);
    const expectSignalCleaned = trackListeners(controller.signal);
    controller.abort();

    await expect(waitForShareImages(root, controller.signal)).rejects.toMatchObject({
      name: "AbortError",
    });

    expectImageCleaned();
    expectSignalCleaned(["abort"]);
    expect(vi.getTimerCount()).toBe(0);
  });

  it("aborts pending image loading and removes all outstanding listeners", async () => {
    const image = createImage();
    root.append(image);
    const expectImageCleaned = trackListeners(image);
    const expectSignalCleaned = trackListeners(controller.signal);
    const waiting = waitForShareImages(root, controller.signal);
    const rejected = expect(waiting).rejects.toMatchObject({ name: "AbortError" });
    await vi.advanceTimersByTimeAsync(0);

    controller.abort();
    await rejected;

    expectImageCleaned();
    expectSignalCleaned(["abort"]);
    expect(vi.getTimerCount()).toBe(0);
  });

  it("waits for an authenticated placeholder to be replaced before inspecting images", async () => {
    const placeholder = createImage();
    placeholder.dataset.authImageLoading = "true";
    root.append(placeholder);
    const expectSignalCleaned = trackListeners(controller.signal);
    let settled = false;
    const waiting = waitForShareImages(root, controller.signal).then(() => {
      settled = true;
    });

    await vi.advanceTimersByTimeAsync(79);
    expect(settled).toBe(false);
    placeholder.replaceWith(createImage(true));
    await vi.advanceTimersByTimeAsync(1);
    await expect(waiting).resolves.toBeUndefined();

    expectSignalCleaned();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("shares one deadline between placeholder polling and pending image loading", async () => {
    const image = createImage();
    image.dataset.authImageLoading = "true";
    root.append(image);
    const expectImageCleaned = trackListeners(image);
    const expectSignalCleaned = trackListeners(controller.signal);
    let settled = false;
    const waiting = waitForShareImages(root, controller.signal, 1000);
    void waiting.then(() => { settled = true; }, () => { settled = true; });
    const rejected = expect(waiting).rejects.toMatchObject({ name: "Error" });

    await vi.advanceTimersByTimeAsync(880);
    delete image.dataset.authImageLoading;
    await vi.advanceTimersByTimeAsync(80);
    await vi.advanceTimersByTimeAsync(39);
    expect(settled).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    await rejected;

    expectImageCleaned();
    expectSignalCleaned();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("times out an authenticated placeholder that never resolves", async () => {
    const image = createImage();
    image.dataset.authImageLoading = "true";
    root.append(image);
    const expectImageCleaned = trackListeners(image);
    const expectSignalCleaned = trackListeners(controller.signal);
    const waiting = waitForShareImages(root, controller.signal, 1000);
    const rejected = expect(waiting).rejects.toMatchObject({ name: "Error" });

    await vi.advanceTimersByTimeAsync(1000);
    await rejected;

    expectImageCleaned();
    expectSignalCleaned();
    expect(vi.getTimerCount()).toBe(0);
  });

  it("aborts authenticated placeholder polling and clears its resources", async () => {
    const image = createImage();
    image.dataset.authImageLoading = "true";
    root.append(image);
    const expectImageCleaned = trackListeners(image);
    const expectSignalCleaned = trackListeners(controller.signal);
    const waiting = waitForShareImages(root, controller.signal);
    const rejected = expect(waiting).rejects.toMatchObject({ name: "AbortError" });
    await vi.advanceTimersByTimeAsync(160);

    controller.abort();
    await vi.advanceTimersByTimeAsync(80);
    await rejected;

    expectImageCleaned();
    expectSignalCleaned(["abort"]);
    expect(vi.getTimerCount()).toBe(0);
  });
});
