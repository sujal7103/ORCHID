import { api } from './api';

export interface APIKey {
  id: string;
  name: string;
  prefix: string;
  agentId: string;
  createdAt: string;
  lastUsedAt?: string;
  expiresAt?: string;
}

export interface CreateAPIKeyResponse {
  key: string;
  apiKey: APIKey;
}

export async function createAPIKey(agentId: string, name: string): Promise<CreateAPIKeyResponse> {
  return api.post<CreateAPIKeyResponse>(`/api/agents/${agentId}/api-keys`, { name });
}

export async function listAPIKeys(agentId: string): Promise<APIKey[]> {
  return api.get<APIKey[]>(`/api/agents/${agentId}/api-keys`);
}

export async function revokeAPIKey(agentId: string, keyId: string): Promise<void> {
  return api.delete(`/api/agents/${agentId}/api-keys/${keyId}`);
}

export function formatLastUsed(date?: string): string {
  if (!date) return 'Never';
  const d = new Date(date);
  const now = new Date();
  const diff = now.getTime() - d.getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

export function maskKeyPrefix(prefix: string): string {
  return `${prefix}${'•'.repeat(24)}`;
}

export async function copyToClipboard(text: string): Promise<void> {
  await navigator.clipboard.writeText(text);
}
