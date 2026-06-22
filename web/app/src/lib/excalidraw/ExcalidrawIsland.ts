import React from "react";
import { createRoot, type Root } from "react-dom/client";
import {
  Excalidraw,
  THEME,
} from "@excalidraw/excalidraw";
import "@excalidraw/excalidraw/index.css";
import type { Theme } from "../stores/preferences";
import {
  parseExcalidrawLibraryJSON,
  serializeExcalidrawLibrary,
} from "./library";
import {
  parseStoredExcalidrawScene,
  serializeExcalidrawScene,
} from "./scene";

export interface ExcalidrawMountOptions {
  sceneJSON: string;
  title: string;
  theme: Theme;
  fontFamily: string;
  libraryJSON: string;
  libraryReturnUrl: string;
  onChange: (sceneJSON: string) => void;
  onLibraryChange: (libraryJSON: string) => void;
}

export interface ExcalidrawMountHandle {
  update(options: ExcalidrawMountOptions): void;
  updateLibrary(libraryJSON: string): void;
  destroy(): void;
}

type AppStateLike = Record<string, unknown>;
type BinaryFiles = Record<string, unknown>;

interface ExcalidrawAPI {
  refresh?: () => void;
  updateLibrary?: (options: {
    libraryItems: readonly unknown[];
    merge?: boolean;
    openLibraryMenu?: boolean;
  }) => Promise<unknown>;
}

export function mountExcalidrawWhiteboard(
  node: HTMLElement,
  initialOptions: ExcalidrawMountOptions,
): ExcalidrawMountHandle {
  const root = createRoot(node);
  let options = initialOptions;
  let excalidrawAPI: ExcalidrawAPI | null = null;

  function render(): void {
    const ExcalidrawComponent = Excalidraw as React.ComponentType<
      Record<string, unknown>
    >;
    const scene = parseStoredExcalidrawScene(
      options.sceneJSON,
      options.fontFamily,
    );
    const initialData = {
      elements: scene.elements,
      appState: {
        ...scene.appState,
        name: options.title,
      },
      files: scene.files,
      libraryItems: parseExcalidrawLibraryJSON(options.libraryJSON),
    };

    root.render(
      React.createElement(ExcalidrawComponent, {
        initialData,
        name: options.title,
        libraryReturnUrl: options.libraryReturnUrl,
        theme: options.theme === "dark" ? THEME.DARK : THEME.LIGHT,
        UIOptions: {
          canvasActions: {
            saveToActiveFile: false,
            toggleTheme: null,
          },
          tools: {
            image: true,
          },
        },
        excalidrawAPI: (api: ExcalidrawAPI) => {
          excalidrawAPI = api;
          api.refresh?.();
        },
        onLibraryChange: (libraryItems: readonly unknown[]) => {
          options.onLibraryChange(serializeExcalidrawLibrary(libraryItems));
        },
        onChange: (
          elements: readonly unknown[],
          appState: AppStateLike,
          files: BinaryFiles,
        ) => {
          options.onChange(
            serializeExcalidrawScene(
              elements,
              appState,
              files,
              options.fontFamily,
            ),
          );
        },
      }),
    );
  }

  render();

  return {
    update(nextOptions: ExcalidrawMountOptions): void {
      options = nextOptions;
      render();
    },
    updateLibrary(libraryJSON: string): void {
      options = { ...options, libraryJSON };
      void excalidrawAPI
        ?.updateLibrary?.({
          libraryItems: parseExcalidrawLibraryJSON(libraryJSON),
          merge: true,
          openLibraryMenu: true,
        })
        .then(() => excalidrawAPI?.refresh?.());
    },
    destroy(): void {
      root.unmount();
    },
  };
}
