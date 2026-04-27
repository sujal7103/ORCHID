/**
 * Auth store for Orchid (standalone).
 *
 * Deliberately simple: email/password login → JWT stored in localStorage.
 * Exposes the same interface (`accessToken`, `isAuthenticated`, `user`, `getAccessToken`)
 * that workflowExecutionService and uploadService read from the store.
 */

import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { setAuthToken, clearAuthToken, getAuthToken } from '@/services/api';
import { getApiBaseUrl } from '@/lib/config';

export interface AuthUser {
  id: string;
  email: string;
  name: string;
  avatar?: string;
  is_admin?: boolean;
}

interface AuthState {
  user: AuthUser | null;
  accessToken: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  error: string | null;

  // Actions
  login: (email: string, password: string) => Promise<void>;
  register: (email: string, password: string, name: string) => Promise<void>;
  logout: () => void;
  initialize: () => void;
  getAccessToken: () => string | null;
  clearError: () => void;
  loginWithGoogleToken: (token: string, user: AuthUser) => void;
}

export const useAuthStore = create<AuthState>()(
  persist(
    (set, get) => ({
      user: null,
      accessToken: null,
      isAuthenticated: false,
      isLoading: false,
      error: null,

      initialize() {
        // Re-hydrate token from localStorage into the api client on page load
        const token = getAuthToken();
        if (token) {
          setAuthToken(token);
          set({ accessToken: token, isAuthenticated: true });
        }

        // Listen for session expiry events emitted by the api client on 401
        window.addEventListener('ca:session-expired', () => {
          get().logout();
        });
      },

      getAccessToken() {
        return get().accessToken;
      },

      async login(email, password) {
        set({ isLoading: true, error: null });
        try {
          const res = await fetch(`${getApiBaseUrl()}/api/auth/login`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password }),
          });

          if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            throw new Error((data as { error?: string }).error ?? 'Login failed');
          }

          const data = (await res.json()) as { access_token: string; user: AuthUser };
          setAuthToken(data.access_token);
          set({
            accessToken: data.access_token,
            user: data.user,
            isAuthenticated: true,
            isLoading: false,
          });
        } catch (err) {
          set({
            isLoading: false,
            error: err instanceof Error ? err.message : 'Login failed',
          });
          throw err;
        }
      },

      async register(email, password, name) {
        set({ isLoading: true, error: null });
        try {
          const res = await fetch(`${getApiBaseUrl()}/api/auth/register`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ email, password, name }),
          });

          if (!res.ok) {
            const data = await res.json().catch(() => ({}));
            throw new Error((data as { error?: string }).error ?? 'Registration failed');
          }

          const data = (await res.json()) as { access_token: string; user: AuthUser };
          setAuthToken(data.access_token);
          set({
            accessToken: data.access_token,
            user: data.user,
            isAuthenticated: true,
            isLoading: false,
          });
        } catch (err) {
          set({
            isLoading: false,
            error: err instanceof Error ? err.message : 'Registration failed',
          });
          throw err;
        }
      },

      logout() {
        clearAuthToken();
        set({ user: null, accessToken: null, isAuthenticated: false });
      },

      clearError() {
        set({ error: null });
      },

      loginWithGoogleToken(token: string, user: AuthUser) {
        setAuthToken(token);
        set({
          accessToken: token,
          user,
          isAuthenticated: true,
          isLoading: false,
          error: null,
        });
      },
    }),
    {
      name: 'ca-auth',
      partialize: (s) => ({ user: s.user, accessToken: s.accessToken, isAuthenticated: s.isAuthenticated }),
    },
  ),
);
