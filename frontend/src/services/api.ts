/**
 * Orchid API client
 *
 * Clean standalone version — no subscription store, no OAuth, no legacy
 * data-migration shims.  JWT is stored in localStorage under 'ca-token'
 * and auto-attached to every request.
 */

import { getApiBaseUrl } from '@/lib/config';

// ─── Token helpers ────────────────────────────────────────────────────────────

const TOKEN_KEY = 'ca-token';

export function setAuthToken(token: string) {
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearAuthToken() {
  localStorage.removeItem(TOKEN_KEY);
}

export function getAuthToken(): string | null {
  return localStorage.getItem(TOKEN_KEY);
}

// ─── Error class ─────────────────────────────────────────────────────────────

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    message: string,
    public readonly data?: unknown,
  ) {
    super(message);
    this.name = 'ApiError';
  }
}

// ─── Request options ─────────────────────────────────────────────────────────

interface RequestOptions {
  includeAuth?: boolean; // default: true
  signal?: AbortSignal;
  timeout?: number; // ms, default: 30 000
}

// ─── Core fetch ──────────────────────────────────────────────────────────────

async function request<T>(
  method: string,
  endpoint: string,
  body?: unknown,
  options: RequestOptions = {},
): Promise<T> {
  const { includeAuth = true, signal, timeout = 30_000 } = options;

  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };

  if (includeAuth) {
    const token = getAuthToken();
    if (token) headers['Authorization'] = `Bearer ${token}`;
  }

  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeout);
  const mergedSignal = signal ?? controller.signal;

  let response: Response;
  try {
    response = await fetch(`${getApiBaseUrl()}${endpoint}`, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
      signal: mergedSignal,
    });
  } finally {
    clearTimeout(timer);
  }

  // Auto-retry on gateway errors (5xx)
  if (response.status >= 500 && response.status < 600) {
    throw new ApiError(response.status, `Server error: ${response.status}`);
  }

  // 401 — token expired or invalid
  if (response.status === 401) {
    clearAuthToken();
    // Emit a custom event so the auth store can react
    window.dispatchEvent(new CustomEvent('ca:session-expired'));
    throw new ApiError(401, 'Session expired — please log in again.');
  }

  if (!response.ok) {
    let errBody: unknown;
    try {
      errBody = await response.json();
    } catch {
      errBody = await response.text();
    }
    const message =
      typeof errBody === 'object' && errBody !== null && 'error' in errBody
        ? String((errBody as Record<string, unknown>)['error'])
        : `Request failed: ${response.status}`;
    throw new ApiError(response.status, message, errBody);
  }

  // 204 No Content
  if (response.status === 204) return undefined as T;

  return response.json() as Promise<T>;
}

// ─── Download helper ──────────────────────────────────────────────────────────

async function downloadFile(url: string, filename?: string): Promise<void> {
  const token = getAuthToken();
  const response = await fetch(url, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });

  if (!response.ok) throw new ApiError(response.status, 'Download failed');

  const blob = await response.blob();
  const objectUrl = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = objectUrl;
  a.download = filename ?? 'download';
  a.click();
  URL.revokeObjectURL(objectUrl);
}

// ─── Public API ───────────────────────────────────────────────────────────────

export const api = {
  get<T>(endpoint: string, options?: RequestOptions): Promise<T> {
    return request<T>('GET', endpoint, undefined, options);
  },
  post<T>(endpoint: string, data?: unknown, options?: RequestOptions): Promise<T> {
    return request<T>('POST', endpoint, data, options);
  },
  put<T>(endpoint: string, data?: unknown, options?: RequestOptions): Promise<T> {
    return request<T>('PUT', endpoint, data, options);
  },
  delete<T>(endpoint: string, options?: RequestOptions): Promise<T> {
    return request<T>('DELETE', endpoint, undefined, options);
  },
  deleteWithBody<T>(endpoint: string, data?: unknown, options?: RequestOptions): Promise<T> {
    return request<T>('DELETE', endpoint, data, options);
  },
  downloadFile,
};
