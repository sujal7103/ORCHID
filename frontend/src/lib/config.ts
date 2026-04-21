/**
 * Runtime config helpers for Orchid.
 * VITE_API_BASE_URL is the only required env var.
 */
export function getApiBaseUrl(): string {
  return import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:3001';
}

export function getWsBaseUrl(): string {
  return import.meta.env.VITE_WS_URL ?? 'ws://localhost:3001';
}
