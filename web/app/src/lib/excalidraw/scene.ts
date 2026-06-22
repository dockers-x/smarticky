const sceneType = "smarticky.excalidraw.scene";

export interface StoredExcalidrawScene {
  type: typeof sceneType;
  version: 1;
  elements: readonly unknown[];
  appState: Record<string, unknown>;
  files: Record<string, unknown>;
  smarticky: {
    fontFamily: string;
  };
}

type AppStateLike = Record<string, unknown>;
type BinaryFiles = Record<string, unknown>;

const persistedAppStateKeys = [
  "viewBackgroundColor",
  "gridSize",
  "objectsSnapModeEnabled",
  "scrollX",
  "scrollY",
  "zoom",
  "currentItemStrokeColor",
  "currentItemBackgroundColor",
  "currentItemFillStyle",
  "currentItemStrokeWidth",
  "currentItemStrokeStyle",
  "currentItemRoughness",
  "currentItemOpacity",
  "currentItemFontFamily",
  "currentItemFontSize",
  "currentItemTextAlign",
  "currentItemStartArrowhead",
  "currentItemEndArrowhead",
  "currentItemRoundness",
] as const;

function safeClone<T>(value: T): T | undefined {
  try {
    return JSON.parse(JSON.stringify(value)) as T;
  } catch {
    return undefined;
  }
}

function cleanAppState(appState: AppStateLike): Record<string, unknown> {
  const next: Record<string, unknown> = {};
  for (const key of persistedAppStateKeys) {
    if (!(key in appState)) continue;
    const cloned = safeClone(appState[key]);
    if (cloned !== undefined) {
      next[key] = cloned;
    }
  }
  return next;
}

function emptyScene(fontFamily: string): StoredExcalidrawScene {
  return {
    type: sceneType,
    version: 1,
    elements: [],
    appState: {
      viewBackgroundColor: "#ffffff",
    },
    files: {},
    smarticky: {
      fontFamily,
    },
  };
}

export function parseStoredExcalidrawScene(
  sceneJSON: string,
  fontFamily: string,
): StoredExcalidrawScene {
  if (!sceneJSON.trim()) return emptyScene(fontFamily);

  try {
    const parsed = JSON.parse(sceneJSON) as Partial<StoredExcalidrawScene> & {
      elements?: readonly unknown[];
      appState?: Record<string, unknown>;
      files?: Record<string, unknown>;
    };

    return {
      type: sceneType,
      version: 1,
      elements: Array.isArray(parsed.elements) ? parsed.elements : [],
      appState:
        parsed.appState && typeof parsed.appState === "object"
          ? parsed.appState
          : emptyScene(fontFamily).appState,
      files:
        parsed.files && typeof parsed.files === "object" ? parsed.files : {},
      smarticky: {
        fontFamily:
          parsed.smarticky?.fontFamily &&
          typeof parsed.smarticky.fontFamily === "string"
            ? parsed.smarticky.fontFamily
            : fontFamily,
      },
    };
  } catch {
    return emptyScene(fontFamily);
  }
}

export function storedSceneFontFamily(
  sceneJSON: string,
  fallback: string,
): string {
  return parseStoredExcalidrawScene(sceneJSON, fallback).smarticky.fontFamily;
}

export function setStoredSceneFontFamily(
  sceneJSON: string,
  fontFamily: string,
): string {
  const scene = parseStoredExcalidrawScene(sceneJSON, fontFamily);
  scene.smarticky.fontFamily = fontFamily;
  return JSON.stringify(scene);
}

export function serializeExcalidrawScene(
  elements: readonly unknown[],
  appState: AppStateLike,
  files: BinaryFiles,
  fontFamily: string,
): string {
  const scene: StoredExcalidrawScene = {
    type: sceneType,
    version: 1,
    elements: safeClone(elements) ?? [],
    appState: cleanAppState(appState),
    files: safeClone(files) ?? {},
    smarticky: {
      fontFamily,
    },
  };
  return JSON.stringify(scene);
}
