export async function waitForShareImages(
  root: HTMLElement,
  signal: AbortSignal,
  timeout = 5000,
): Promise<void> {
  const deadline = Date.now() + timeout;
  const cancelled = () => new DOMException("Image export cancelled", "AbortError");

  while (root.querySelector("img[data-auth-image-loading='true']")) {
    if (signal.aborted) throw cancelled();
    if (Date.now() >= deadline) throw new Error("Image loading timed out");
    await new Promise((resolve) => setTimeout(resolve, Math.min(80, deadline - Date.now())));
  }

  if (signal.aborted) throw cancelled();
  await Promise.all(Array.from(root.querySelectorAll<HTMLImageElement>("img")).map((image) => {
    if (image.complete) return Promise.resolve();
    return new Promise<void>((resolve, reject) => {
      const timer = setTimeout(() => {
        cleanup();
        reject(new Error("Image loading timed out"));
      }, Math.max(0, deadline - Date.now()));
      const done = () => {
        cleanup();
        resolve();
      };
      const abort = () => {
        cleanup();
        reject(cancelled());
      };
      function cleanup(): void {
        clearTimeout(timer);
        image.removeEventListener("load", done);
        image.removeEventListener("error", done);
        signal.removeEventListener("abort", abort);
      }
      image.addEventListener("load", done, { once: true });
      image.addEventListener("error", done, { once: true });
      signal.addEventListener("abort", abort, { once: true });
    });
  }));
}
