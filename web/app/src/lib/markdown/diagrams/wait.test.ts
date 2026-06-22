import { describe, expect, it } from "vitest";
import { waitForDiagramSettle } from "./wait";
import type { DiagramRuntimeState } from "./types";

describe("waitForDiagramSettle", () => {
  it("resolves immediately when diagrams are settled", async () => {
    const state: DiagramRuntimeState = { pending: 0, total: 1, settled: true };

    await expect(waitForDiagramSettle(() => state, 20)).resolves.toBeUndefined();
  });

  it("waits until pending diagrams settle", async () => {
    const state: DiagramRuntimeState = { pending: 1, total: 1, settled: false };
    setTimeout(() => {
      state.pending = 0;
      state.settled = true;
    }, 5);

    await expect(waitForDiagramSettle(() => state, 50)).resolves.toBeUndefined();
  });

  it("rejects after timeout", async () => {
    const state: DiagramRuntimeState = { pending: 1, total: 1, settled: false };

    await expect(waitForDiagramSettle(() => state, 5)).rejects.toThrow(
      "Diagram rendering timed out",
    );
  });
});
