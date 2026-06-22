import { writable } from "svelte/store";

interface WhiteboardState {
  openID: string;
  libraryRefreshID: string;
  libraryRefreshToken: number;
}

export interface WhiteboardReferenceRemovalResult {
  handled: boolean;
  removedCount: number;
}

type WhiteboardReferenceRemovalHandler = (
  whiteboardID: string,
  noteID: string,
) => WhiteboardReferenceRemovalResult | Promise<WhiteboardReferenceRemovalResult>;

function createWhiteboardStore() {
  const { subscribe, update } = writable<WhiteboardState>({
    openID: "",
    libraryRefreshID: "",
    libraryRefreshToken: 0,
  });
  const referenceRemovalHandlers = new Set<WhiteboardReferenceRemovalHandler>();

  return {
    subscribe,
    open(whiteboardID: string): void {
      if (!whiteboardID) return;
      update((state) => ({ ...state, openID: whiteboardID }));
    },
    close(): void {
      update((state) => ({ ...state, openID: "" }));
    },
    refreshLibrary(whiteboardID: string): void {
      if (!whiteboardID) return;
      update((state) => ({
        ...state,
        libraryRefreshID: whiteboardID,
        libraryRefreshToken: state.libraryRefreshToken + 1,
      }));
    },
    registerReferenceRemoval(
      handler: WhiteboardReferenceRemovalHandler,
    ): () => void {
      referenceRemovalHandlers.add(handler);
      return () => {
        referenceRemovalHandlers.delete(handler);
      };
    },
    async removeReference(
      whiteboardID: string,
      noteID: string,
    ): Promise<WhiteboardReferenceRemovalResult> {
      for (const handler of Array.from(referenceRemovalHandlers).reverse()) {
        const result = await handler(whiteboardID, noteID);
        if (result.handled) return result;
      }
      return { handled: false, removedCount: 0 };
    },
  };
}

export const whiteboardStore = createWhiteboardStore();
