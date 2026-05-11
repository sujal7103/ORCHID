/**
 * Credential service
 * Handles backend API calls for credential management
 *
 * SECURITY NOTE: This service never handles raw credential values after creation.
 * The frontend only sees masked previews. Actual credential values are encrypted
 * and stored on the backend, accessible only by tools at execution time.
 */

import { api } from './api';
import type {
  Credential,
  Integration,
  IntegrationCategory,
  CreateCredentialRequest,
  UpdateCredentialRequest,
  TestCredentialResponse,
  GetCredentialsResponse,
  GetIntegrationsResponse,
  CredentialsByIntegration,
  CredentialReference,
} from '@/types/credential';

// ============================================================================
// Integration API Functions (Public)
// ============================================================================

/**
 * Get all available integrations grouped by category
 */
export async function getIntegrations(): Promise<IntegrationCategory[]> {
  try {
    const response = await api.get<GetIntegrationsResponse>('/api/integrations', {
      includeAuth: false,
    });
    return response.categories || [];
  } catch (error) {
    console.error('Failed to fetch integrations:', error);
    return [];
  }
}

/**
 * Get a specific integration by ID
 */
export async function getIntegration(integrationId: string): Promise<Integration | null> {
  try {
    const response = await api.get<Integration>(`/api/integrations/${integrationId}`, {
      includeAuth: false,
    });
    return response;
  } catch (error) {
    console.error('Failed to fetch integration:', error);
    return null;
  }
}

// ============================================================================
// Credential CRUD API Functions (Authenticated)
// ============================================================================

/**
 * Create a new credential
 * SECURITY: The credential data is sent once to the backend where it's encrypted.
 * The raw values are never returned or stored on the frontend.
 */
export async function createCredential(data: CreateCredentialRequest): Promise<Credential> {
  const response = await api.post<Credential>('/api/credentials', data);
  return response;
}

/**
 * List all credentials for the current user
 * Returns metadata only - no encrypted data is ever sent to frontend
 */
export async function listCredentials(integrationType?: string): Promise<Credential[]> {
  try {
    const endpoint = integrationType
      ? `/api/credentials?type=${encodeURIComponent(integrationType)}`
      : '/api/credentials';
    const response = await api.get<GetCredentialsResponse>(endpoint);
    return response.credentials || [];
  } catch (error) {
    console.error('Failed to fetch credentials:', error);
    return [];
  }
}

/**
 * Get a specific credential by ID
 */
export async function getCredential(credentialId: string): Promise<Credential | null> {
  try {
    const response = await api.get<Credential>(`/api/credentials/${credentialId}`);
    return response;
  } catch (error) {
    console.error('Failed to fetch credential:', error);
    return null;
  }
}

/**
 * Update a credential's name and/or data
 * If data is provided, it will be re-encrypted on the backend
 */
export async function updateCredential(
  credentialId: string,
  data: UpdateCredentialRequest
): Promise<Credential> {
  const response = await api.put<Credential>(`/api/credentials/${credentialId}`, data);
  return response;
}

/**
 * Delete a credential permanently
 */
export async function deleteCredential(credentialId: string): Promise<void> {
  await api.delete(`/api/credentials/${credentialId}`);
}

/**
 * Test a credential by making a real API call to the external service
 */
export async function testCredential(credentialId: string): Promise<TestCredentialResponse> {
  const response = await api.post<TestCredentialResponse>(
    `/api/credentials/${credentialId}/test`,
    {}
  );
  return response;
}

// ============================================================================
// Specialized Queries
// ============================================================================

/**
 * Get credentials grouped by integration type
 * Useful for the credentials management page
 */
export async function getCredentialsByIntegration(): Promise<CredentialsByIntegration[]> {
  try {
    const response = await api.get<{ integrations: CredentialsByIntegration[] }>(
      '/api/credentials/by-integration'
    );
    return response.integrations || [];
  } catch (error) {
    console.error('Failed to fetch credentials by integration:', error);
    return [];
  }
}

/**
 * Get credential references for LLM context
 * Returns only names and IDs - safe to include in prompts
 */
export async function getCredentialReferences(
  integrationTypes?: string[]
): Promise<CredentialReference[]> {
  try {
    let endpoint = '/api/credentials/references';
    if (integrationTypes && integrationTypes.length > 0) {
      endpoint += `?types=${integrationTypes.join(',')}`;
    }
    const response = await api.get<{ credentials: CredentialReference[] }>(endpoint);
    return response.credentials || [];
  } catch (error) {
    console.error('Failed to fetch credential references:', error);
    return [];
  }
}

// ============================================================================
// Helper Functions
// ============================================================================

/**
 * Format last used time for display
 */
export function formatLastUsed(lastUsedAt?: string): string {
  if (!lastUsedAt) return 'Never used';

  const date = new Date(lastUsedAt);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / (1000 * 60));
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffMins < 1) return 'Just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays === 1) return 'Yesterday';
  if (diffDays < 7) return `${diffDays} days ago`;

  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
  });
}

/**
 * Format usage count for display
 */
export function formatUsageCount(count: number): string {
  if (count === 0) return 'Never used';
  if (count === 1) return 'Used once';
  if (count < 100) return `Used ${count} times`;
  if (count < 1000) return `Used ${count}+ times`;
  return `Used ${Math.floor(count / 1000)}k+ times`;
}

/**
 * Get test status display info
 */
export function getTestStatusDisplay(testStatus?: string): {
  label: string;
  color: string;
  bgColor: string;
} {
  switch (testStatus) {
    case 'success':
      return {
        label: 'Connected',
        color: 'text-green-400',
        bgColor: 'bg-green-500/10',
      };
    case 'failed':
      return {
        label: 'Failed',
        color: 'text-red-400',
        bgColor: 'bg-red-500/10',
      };
    default:
      return {
        label: 'Not tested',
        color: 'text-gray-400',
        bgColor: 'bg-gray-500/10',
      };
  }
}

/**
 * Copy text to clipboard
 */
export async function copyToClipboard(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    // Fallback for older browsers
    const textArea = document.createElement('textarea');
    textArea.value = text;
    textArea.style.position = 'fixed';
    textArea.style.left = '-999999px';
    document.body.appendChild(textArea);
    textArea.select();
    try {
      document.execCommand('copy');
      document.body.removeChild(textArea);
      return true;
    } catch {
      document.body.removeChild(textArea);
      return false;
    }
  }
}

/**
 * Validate credential data against integration schema
 */
export function validateCredentialData(
  integration: Integration,
  data: Record<string, unknown>
): { valid: boolean; errors: string[] } {
  const errors: string[] = [];

  for (const field of integration.fields) {
    const value = data[field.key];

    if (field.required) {
      if (value === undefined || value === null || value === '') {
        errors.push(`${field.label} is required`);
      }
    }

    // Type-specific validation
    if (value !== undefined && value !== null && value !== '') {
      if (field.type === 'webhook_url' && typeof value === 'string') {
        try {
          const url = new URL(value);
          if (!['http:', 'https:'].includes(url.protocol)) {
            errors.push(`${field.label} must be a valid HTTP/HTTPS URL`);
          }
        } catch {
          errors.push(`${field.label} must be a valid URL`);
        }
      }

      if (field.type === 'json' && typeof value === 'string') {
        try {
          JSON.parse(value);
        } catch {
          errors.push(`${field.label} must be valid JSON`);
        }
      }
    }
  }

  return {
    valid: errors.length === 0,
    errors,
  };
}

/**
 * Get integration icon URL or Lucide icon name
 * Returns the icon identifier for the integration
 */
export function getIntegrationIcon(integration: Integration): string {
  return integration.icon || 'Settings';
}

/**
 * Check if an integration has custom branded icon
 * Some integrations (Discord, Slack, etc.) have custom SVG icons
 */
export function hasCustomIcon(integrationId: string): boolean {
  const customIcons = [
    'discord',
    'slack',
    'telegram',
    'notion',
    'github',
    'gitlab',
    'openai',
    'anthropic',
  ];
  return customIcons.includes(integrationId);
}
