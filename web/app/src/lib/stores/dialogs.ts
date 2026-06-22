import { writable } from "svelte/store";

export type DialogTone = "info" | "success" | "error";

interface ConfirmRequest {
  title: string;
  message: string;
  confirmLabel: string;
  cancelLabel: string;
  resolve: (value: boolean) => void;
}

interface InputRequest {
  title: string;
  label: string;
  message?: string;
  initialValue?: string;
  placeholder?: string;
  confirmLabel: string;
  cancelLabel: string;
  requiredMessage: string;
  resolve: (value: string | null) => void;
}

interface Notification {
  id: number;
  message: string;
  tone: DialogTone;
}

let notificationID = 0;

export const confirmRequest = writable<ConfirmRequest | null>(null);
export const inputRequest = writable<InputRequest | null>(null);
export const notifications = writable<Notification[]>([]);

export function confirmDialog(
  options: Omit<ConfirmRequest, "resolve">,
): Promise<boolean> {
  return new Promise((resolve) => {
    confirmRequest.set({ ...options, resolve });
  });
}

export function inputDialog(
  options: Omit<InputRequest, "resolve">,
): Promise<string | null> {
  return new Promise((resolve) => {
    inputRequest.set({ ...options, resolve });
  });
}

export function notify(message: string, tone: DialogTone = "info"): void {
  const id = ++notificationID;
  notifications.update((items) => [...items, { id, message, tone }]);
  window.setTimeout(() => {
    notifications.update((items) => items.filter((item) => item.id !== id));
  }, 3200);
}
