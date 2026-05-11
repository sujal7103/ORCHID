import { create } from 'zustand';

export type ToastType = 'success' | 'error' | 'warning' | 'info';

export interface Toast {
  id: string;
  type: ToastType;
  title?: string;
  message: string;
  duration?: number;
  action?: { label: string; onClick: () => void };
}

interface ToastState {
  toasts: Toast[];
  addToast: (toast: Omit<Toast, 'id'>) => string;
  removeToast: (id: string) => void;
  clearAllToasts: () => void;
}

let toastIdCounter = 0;

export const useToastStore = create<ToastState>((set, get) => ({
  toasts: [],

  addToast: toast => {
    const id = `toast-${++toastIdCounter}-${Date.now()}`;
    const newToast: Toast = { ...toast, id };

    set(state => ({
      toasts: [...state.toasts, newToast],
    }));

    // Auto-remove after duration (default 5 seconds)
    const duration = toast.duration ?? 5000;
    if (duration > 0) {
      setTimeout(() => {
        get().removeToast(id);
      }, duration);
    }

    return id;
  },

  removeToast: id => {
    set(state => ({
      toasts: state.toasts.filter(t => t.id !== id),
    }));
  },

  clearAllToasts: () => {
    set({ toasts: [] });
  },
}));

// Helper functions for common toast types
export const toast = {
  success: (message: string, title?: string) =>
    useToastStore.getState().addToast({ type: 'success', message, title }),
  error: (message: string, title?: string) =>
    useToastStore.getState().addToast({ type: 'error', message, title, duration: 8000 }),
  warning: (message: string, title?: string) =>
    useToastStore.getState().addToast({ type: 'warning', message, title }),
  info: (message: string, title?: string) =>
    useToastStore.getState().addToast({ type: 'info', message, title }),
};
