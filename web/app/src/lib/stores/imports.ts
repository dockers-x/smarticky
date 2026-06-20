import { get, writable } from "svelte/store";
import {
  confirmEvernote,
  previewEvernote,
  type ImportPreview,
  type ImportResult,
} from "../api/imports";

interface ImportsState {
  loading: boolean;
  preview: ImportPreview | null;
  result: ImportResult | null;
  error: string;
  fileName: string;
}

const initialState: ImportsState = {
  loading: false,
  preview: null,
  result: null,
  error: "",
  fileName: "",
};

function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "导入失败，请稍后重试";
}

function createImportsStore() {
  const { subscribe, set, update } = writable<ImportsState>({ ...initialState });

  return {
    subscribe,
    reset() {
      set({ ...initialState });
    },
    async preview(file: File): Promise<ImportPreview | null> {
      if (!file.name.toLowerCase().endsWith(".enex")) {
        update((state) => ({
          ...state,
          preview: null,
          result: null,
          error: "请选择 .enex 文件",
          fileName: file.name,
        }));
        return null;
      }

      update((state) => ({
        ...state,
        loading: true,
        preview: null,
        result: null,
        error: "",
        fileName: file.name,
      }));

      try {
        const preview = await previewEvernote(file);
        update((state) => ({
          ...state,
          loading: false,
          preview,
          result: null,
          error: "",
          fileName: preview.filename || file.name,
        }));
        return preview;
      } catch (error) {
        update((state) => ({
          ...state,
          loading: false,
          preview: null,
          result: null,
          error: errorMessage(error),
        }));
        return null;
      }
    },
    async confirm(): Promise<ImportResult | null> {
      const state = get({ subscribe });
      if (!state.preview) {
        update((current) => ({ ...current, error: "请先选择 .enex 文件" }));
        return null;
      }

      update((current) => ({
        ...current,
        loading: true,
        result: null,
        error: "",
      }));

      try {
        const result = await confirmEvernote(state.preview.job_id);
        update((current) => ({
          ...current,
          loading: false,
          result,
          error: "",
        }));
        return result;
      } catch (error) {
        update((current) => ({
          ...current,
          loading: false,
          result: null,
          error: errorMessage(error),
        }));
        return null;
      }
    },
  };
}

export const importsStore = createImportsStore();
