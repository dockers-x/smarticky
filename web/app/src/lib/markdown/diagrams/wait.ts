import type { DiagramRuntimeState } from "./types";

export async function waitForDiagramSettle(
  getState: () => DiagramRuntimeState,
  timeoutMs = 5000,
): Promise<void> {
  const start = performance.now();

  while (!getState().settled) {
    if (performance.now() - start > timeoutMs) {
      throw new Error("Diagram rendering timed out");
    }
    await new Promise((resolve) => setTimeout(resolve, 16));
  }
}
