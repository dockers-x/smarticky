import { get, writable } from "svelte/store";
import {
  deleteFont,
  downloadFontBlob,
  listFonts,
  uploadFont,
  type FontRecord,
  type UploadFontOptions,
} from "../api/fonts";

export const DEFAULT_FONT = "default";

export const systemFontOptions = [
  { name: DEFAULT_FONT, displayName: "System" },
  { name: "Arial", displayName: "Arial" },
  { name: "Helvetica", displayName: "Helvetica" },
  { name: "Times New Roman", displayName: "Times New Roman" },
  { name: "Georgia", displayName: "Georgia" },
  { name: "Courier New", displayName: "Courier New" },
  { name: "Verdana", displayName: "Verdana" },
];

interface FontsState {
  loading: boolean;
  error: string;
  fonts: FontRecord[];
  selected: string;
}

const loadedFontNames = new Set<string>();

function storedFont(): string {
  if (typeof localStorage === "undefined") return DEFAULT_FONT;
  return localStorage.getItem("selected-font") || DEFAULT_FONT;
}

function quoteFontFamily(name: string): string {
  return `"${name.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`;
}

export function fontFamilyValue(name: string): string {
  if (!name || name === DEFAULT_FONT) {
    return "inherit";
  }

  return `${quoteFontFamily(name)}, -apple-system, BlinkMacSystemFont, "PingFang SC", "Source Han Sans SC", "Noto Sans CJK SC", "Microsoft YaHei", "Segoe UI", sans-serif`;
}

function applySelectedFont(name: string): void {
  if (typeof localStorage !== "undefined") {
    localStorage.setItem("selected-font", name);
  }
  if (typeof document !== "undefined") {
    document.documentElement.style.setProperty(
      "--editor-font-family",
      fontFamilyValue(name),
    );
  }
}

async function loadFontFace(font: FontRecord): Promise<void> {
  if (
    typeof FontFace === "undefined" ||
    typeof document === "undefined" ||
    loadedFontNames.has(font.name)
  ) {
    return;
  }

  const blob = await downloadFontBlob(font);
  const fontURL = URL.createObjectURL(blob);

  try {
    const fontFace = new FontFace(font.name, `url(${fontURL})`);
    await fontFace.load();
    document.fonts.add(fontFace);
    loadedFontNames.add(font.name);
  } finally {
    window.setTimeout(() => URL.revokeObjectURL(fontURL), 5000);
  }
}

function createFontsStore() {
  const initial: FontsState = {
    loading: false,
    error: "",
    fonts: [],
    selected: storedFont(),
  };
  const { subscribe, update } = writable<FontsState>(initial);
  applySelectedFont(initial.selected);

  async function load(): Promise<void> {
    update((state) => ({ ...state, loading: true, error: "" }));
    try {
      const fonts = await listFonts();
      await Promise.allSettled(fonts.map(loadFontFace));
      update((state) => ({ ...state, loading: false, fonts, error: "" }));
      applySelectedFont(get({ subscribe }).selected);
    } catch (error) {
      update((state) => ({
        ...state,
        loading: false,
        error: error instanceof Error ? error.message : "Failed to load fonts",
      }));
    }
  }

  function select(name: string): void {
    applySelectedFont(name);
    update((state) => ({ ...state, selected: name }));
  }

  return {
    subscribe,
    load,
    select,
    async upload(options: UploadFontOptions): Promise<FontRecord> {
      const font = await uploadFont(options);
      await loadFontFace(font);
      await load();
      return font;
    },
    async delete(fontID: string): Promise<void> {
      const current = get({ subscribe });
      const deleted = current.fonts.find((font) => font.id === fontID);
      await deleteFont(fontID);
      if (deleted && current.selected === deleted.name) {
        select(DEFAULT_FONT);
      }
      await load();
    },
  };
}

export const fontsStore = createFontsStore();
