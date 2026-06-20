import { writable } from "svelte/store";
import { apiFetch } from "../api/client";
import type { SetupCheckResponse, User } from "../api/types";
import { t } from "./preferences";

interface AuthState {
  loading: boolean;
  setupNeeded: boolean;
  user: User | null;
  error: string;
}

function createAuthStore() {
  const { subscribe, set, update } = writable<AuthState>({
    loading: true,
    setupNeeded: false,
    user: null,
    error: "",
  });

  return {
    subscribe,
    async hydrate() {
      update((state) => ({ ...state, loading: true, error: "" }));

      const setup = await apiFetch<SetupCheckResponse>("/setup/check");
      if (setup.setup_needed) {
        set({ loading: false, setupNeeded: true, user: null, error: "" });
        window.location.href = "/setup";
        return;
      }

      const token = localStorage.getItem("jwt_token");
      if (!token) {
        set({ loading: false, setupNeeded: false, user: null, error: "" });
        window.location.href = "/login";
        return;
      }

      try {
        const user = await apiFetch<User>("/auth/me");
        set({ loading: false, setupNeeded: false, user, error: "" });
      } catch {
        localStorage.removeItem("jwt_token");
        localStorage.removeItem("user");
        set({
          loading: false,
          setupNeeded: false,
          user: null,
          error: t("sessionExpired"),
        });
        window.location.href = "/login";
      }
    },
    logout() {
      localStorage.removeItem("jwt_token");
      localStorage.removeItem("user");
      set({ loading: false, setupNeeded: false, user: null, error: "" });
      window.location.href = "/login";
    },
  };
}

export const authStore = createAuthStore();
