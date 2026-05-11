/**
 * Credential Manager Type Definitions
 *
 * This file contains all type definitions for the Credentials Manager feature.
 * Credentials are encrypted API keys, webhooks, and tokens for external integrations.
 * The LLM never sees raw credentials - only tools access them at runtime.
 */

// =============================================================================
// Credential Types
// =============================================================================

export interface Credential {
  id: string;
  userId: string;
  name: string;
  integrationType: string;
  metadata: CredentialMetadata;
  createdAt: string;
  updatedAt: string;
}

export interface CredentialMetadata {
  maskedPreview: string; // e.g., "https://discord...xxx"
  icon?: string;
  lastUsedAt?: string;
  usageCount: number;
  lastTestAt?: string;
  testStatus?: 'success' | 'failed' | 'pending';
}

// =============================================================================
// Integration Types
// =============================================================================

export interface Integration {
  id: string;
  name: string;
  description: string;
  icon: string;
  category: string;
  fields: IntegrationField[];
  tools: string[];
  docsUrl?: string;
  comingSoon?: boolean;
}

export interface IntegrationField {
  key: string;
  label: string;
  type: 'api_key' | 'webhook_url' | 'token' | 'text' | 'select' | 'json';
  required: boolean;
  placeholder?: string;
  helpText?: string;
  options?: string[];
  default?: string;
  sensitive: boolean;
}

export interface IntegrationCategory {
  id: string;
  name: string;
  icon: string;
  integrations: Integration[];
}

// =============================================================================
// Request/Response Types
// =============================================================================

export interface CreateCredentialRequest {
  name: string;
  integrationType: string;
  data: Record<string, unknown>;
}

export interface UpdateCredentialRequest {
  name?: string;
  data?: Record<string, unknown>;
}

export interface TestCredentialResponse {
  success: boolean;
  message: string;
  details?: string;
}

export interface GetCredentialsResponse {
  credentials: Credential[];
  total: number;
}

export interface GetIntegrationsResponse {
  categories: IntegrationCategory[];
}

export interface CredentialsByIntegration {
  integrationType: string;
  integration: Integration;
  credentials: Credential[];
}

export interface GetCredentialsByIntegrationResponse {
  integrations: CredentialsByIntegration[];
}

// Credential reference for LLM context (safe to show to LLM - names only)
export interface CredentialReference {
  id: string;
  name: string;
  integrationType: string;
}

// =============================================================================
// UI State Types
// =============================================================================

export interface CredentialsUIState {
  selectedIntegrationType: string | null;
  selectedCredentialId: string | null;
  isAddModalOpen: boolean;
  isEditModalOpen: boolean;
  isTestingCredential: boolean;
  searchQuery: string;
}

// =============================================================================
// Integration Category Icons (Lucide)
// =============================================================================

export const INTEGRATION_CATEGORY_ICONS: Record<string, string> = {
  communication: 'MessageSquare',
  productivity: 'LayoutGrid',
  development: 'Code',
  crm: 'Users',
  marketing: 'Mail',
  analytics: 'BarChart2',
  ecommerce: 'ShoppingCart',
  deployment: 'Rocket',
  ai: 'Brain',
  storage: 'HardDrive',
  database: 'Database',
  social: 'Share2',
  custom: 'Settings',
};

// =============================================================================
// Integration Icons (Custom SVGs or Lucide fallbacks)
// =============================================================================

export const INTEGRATION_ICONS: Record<string, string> = {
  discord: 'discord',
  slack: 'slack',
  telegram: 'telegram',
  teams: 'microsoft',
  google_chat: 'google',
  zoom: 'zoom',
  twilio: 'twilio',
  notion: 'notion',
  airtable: 'airtable',
  trello: 'trello',
  clickup: 'clickup',
  calendly: 'calendly',
  github: 'github',
  gitlab: 'gitlab',
  linear: 'linear',
  jira: 'jira',
  hubspot: 'hubspot',
  leadsquared: 'leadsquared',
  sendgrid: 'sendgrid',
  brevo: 'brevo',
  mailchimp: 'mailchimp',
  mixpanel: 'mixpanel',
  posthog: 'posthog',
  shopify: 'shopify',
  netlify: 'netlify',
  openai: 'openai',
  anthropic: 'anthropic',
  google_ai: 'google',
  aws_s3: 'aws',
  x_twitter: 'x_twitter',
  custom_webhook: 'Webhook',
  rest_api: 'Globe',
  mongodb: 'mongodb',
  redis: 'redis',
};

// =============================================================================
// Test Status Display
// =============================================================================

export const TEST_STATUS_INFO: Record<string, { label: string; color: string; icon: string }> = {
  success: {
    label: 'Connected',
    color: 'text-green-500',
    icon: 'CheckCircle',
  },
  failed: {
    label: 'Failed',
    color: 'text-red-500',
    icon: 'XCircle',
  },
  pending: {
    label: 'Not tested',
    color: 'text-gray-400',
    icon: 'Circle',
  },
};
