/**
 * Block Validation Utilities
 *
 * This module provides validation logic for workflow blocks to detect
 * missing credentials, incomplete configuration, and other issues that
 * would prevent successful execution.
 */

import type { Block, LLMInferenceConfig, VariableConfig, Workflow } from '@/types/agent';

// ============================================================================
// Tool-to-Integration Mapping
// Tools that require credentials and their corresponding integration types
// ============================================================================

export const TOOL_CREDENTIAL_REQUIREMENTS: Record<
  string,
  {
    integrationType: string;
    integrationName: string;
    icon: string;
  }
> = {
  // Messaging Tools
  send_discord_message: {
    integrationType: 'discord',
    integrationName: 'Discord',
    icon: 'discord',
  },
  send_slack_message: {
    integrationType: 'slack',
    integrationName: 'Slack',
    icon: 'slack',
  },
  send_telegram_message: {
    integrationType: 'telegram',
    integrationName: 'Telegram',
    icon: 'telegram',
  },
  send_google_chat_message: {
    integrationType: 'google_chat',
    integrationName: 'Google Chat',
    icon: 'google',
  },
  send_teams_message: {
    integrationType: 'teams',
    integrationName: 'Microsoft Teams',
    icon: 'microsoft',
  },
  send_email: {
    integrationType: 'sendgrid',
    integrationName: 'SendGrid',
    icon: 'sendgrid',
  },
  send_brevo_email: {
    integrationType: 'brevo',
    integrationName: 'Brevo',
    icon: 'brevo',
  },
  twilio_send_sms: {
    integrationType: 'twilio',
    integrationName: 'Twilio',
    icon: 'twilio',
  },
  twilio_send_whatsapp: {
    integrationType: 'twilio',
    integrationName: 'Twilio',
    icon: 'twilio',
  },

  // Video Conferencing
  zoom_meeting: {
    integrationType: 'zoom',
    integrationName: 'Zoom',
    icon: 'zoom',
  },
  calendly_events: {
    integrationType: 'calendly',
    integrationName: 'Calendly',
    icon: 'calendly',
  },
  calendly_event_types: {
    integrationType: 'calendly',
    integrationName: 'Calendly',
    icon: 'calendly',
  },
  calendly_invitees: {
    integrationType: 'calendly',
    integrationName: 'Calendly',
    icon: 'calendly',
  },

  // Project Management
  jira_issues: {
    integrationType: 'jira',
    integrationName: 'Jira',
    icon: 'jira',
  },
  jira_create_issue: {
    integrationType: 'jira',
    integrationName: 'Jira',
    icon: 'jira',
  },
  jira_update_issue: {
    integrationType: 'jira',
    integrationName: 'Jira',
    icon: 'jira',
  },
  linear_issues: {
    integrationType: 'linear',
    integrationName: 'Linear',
    icon: 'linear',
  },
  linear_create_issue: {
    integrationType: 'linear',
    integrationName: 'Linear',
    icon: 'linear',
  },
  linear_update_issue: {
    integrationType: 'linear',
    integrationName: 'Linear',
    icon: 'linear',
  },
  clickup_tasks: {
    integrationType: 'clickup',
    integrationName: 'ClickUp',
    icon: 'clickup',
  },
  clickup_create_task: {
    integrationType: 'clickup',
    integrationName: 'ClickUp',
    icon: 'clickup',
  },
  clickup_update_task: {
    integrationType: 'clickup',
    integrationName: 'ClickUp',
    icon: 'clickup',
  },
  trello_boards: {
    integrationType: 'trello',
    integrationName: 'Trello',
    icon: 'trello',
  },
  trello_lists: {
    integrationType: 'trello',
    integrationName: 'Trello',
    icon: 'trello',
  },
  trello_cards: {
    integrationType: 'trello',
    integrationName: 'Trello',
    icon: 'trello',
  },
  trello_create_card: {
    integrationType: 'trello',
    integrationName: 'Trello',
    icon: 'trello',
  },
  asana_tasks: {
    integrationType: 'asana',
    integrationName: 'Asana',
    icon: 'asana',
  },

  // CRM & Sales
  hubspot_contacts: {
    integrationType: 'hubspot',
    integrationName: 'HubSpot',
    icon: 'hubspot',
  },
  hubspot_create_contact: {
    integrationType: 'hubspot',
    integrationName: 'HubSpot',
    icon: 'hubspot',
  },
  hubspot_deals: {
    integrationType: 'hubspot',
    integrationName: 'HubSpot',
    icon: 'hubspot',
  },
  hubspot_companies: {
    integrationType: 'hubspot',
    integrationName: 'HubSpot',
    icon: 'hubspot',
  },
  leadsquared_leads: {
    integrationType: 'leadsquared',
    integrationName: 'LeadSquared',
    icon: 'leadsquared',
  },
  leadsquared_create_lead: {
    integrationType: 'leadsquared',
    integrationName: 'LeadSquared',
    icon: 'leadsquared',
  },
  mailchimp_lists: {
    integrationType: 'mailchimp',
    integrationName: 'Mailchimp',
    icon: 'mailchimp',
  },
  mailchimp_add_subscriber: {
    integrationType: 'mailchimp',
    integrationName: 'Mailchimp',
    icon: 'mailchimp',
  },

  // Analytics
  posthog_capture: {
    integrationType: 'posthog',
    integrationName: 'PostHog',
    icon: 'posthog',
  },
  posthog_identify: {
    integrationType: 'posthog',
    integrationName: 'PostHog',
    icon: 'posthog',
  },
  posthog_query: {
    integrationType: 'posthog',
    integrationName: 'PostHog',
    icon: 'posthog',
  },
  mixpanel_track: {
    integrationType: 'mixpanel',
    integrationName: 'Mixpanel',
    icon: 'mixpanel',
  },
  mixpanel_user_profile: {
    integrationType: 'mixpanel',
    integrationName: 'Mixpanel',
    icon: 'mixpanel',
  },

  // Code & DevOps
  github_create_issue: {
    integrationType: 'github',
    integrationName: 'GitHub',
    icon: 'github',
  },
  github_list_issues: {
    integrationType: 'github',
    integrationName: 'GitHub',
    icon: 'github',
  },
  github_get_repo: {
    integrationType: 'github',
    integrationName: 'GitHub',
    icon: 'github',
  },
  github_add_comment: {
    integrationType: 'github',
    integrationName: 'GitHub',
    icon: 'github',
  },
  gitlab_projects: {
    integrationType: 'gitlab',
    integrationName: 'GitLab',
    icon: 'gitlab',
  },
  gitlab_issues: {
    integrationType: 'gitlab',
    integrationName: 'GitLab',
    icon: 'gitlab',
  },
  gitlab_mrs: {
    integrationType: 'gitlab',
    integrationName: 'GitLab',
    icon: 'gitlab',
  },
  netlify_sites: {
    integrationType: 'netlify',
    integrationName: 'Netlify',
    icon: 'netlify',
  },
  netlify_deploys: {
    integrationType: 'netlify',
    integrationName: 'Netlify',
    icon: 'netlify',
  },
  netlify_trigger_build: {
    integrationType: 'netlify',
    integrationName: 'Netlify',
    icon: 'netlify',
  },

  // Productivity
  notion_search: {
    integrationType: 'notion',
    integrationName: 'Notion',
    icon: 'notion',
  },
  notion_query_database: {
    integrationType: 'notion',
    integrationName: 'Notion',
    icon: 'notion',
  },
  notion_create_page: {
    integrationType: 'notion',
    integrationName: 'Notion',
    icon: 'notion',
  },
  notion_update_page: {
    integrationType: 'notion',
    integrationName: 'Notion',
    icon: 'notion',
  },
  airtable_list: {
    integrationType: 'airtable',
    integrationName: 'Airtable',
    icon: 'airtable',
  },
  airtable_read: {
    integrationType: 'airtable',
    integrationName: 'Airtable',
    icon: 'airtable',
  },
  airtable_create: {
    integrationType: 'airtable',
    integrationName: 'Airtable',
    icon: 'airtable',
  },
  airtable_update: {
    integrationType: 'airtable',
    integrationName: 'Airtable',
    icon: 'airtable',
  },

  // E-Commerce
  shopify_products: {
    integrationType: 'shopify',
    integrationName: 'Shopify',
    icon: 'shopify',
  },
  shopify_orders: {
    integrationType: 'shopify',
    integrationName: 'Shopify',
    icon: 'shopify',
  },
  shopify_customers: {
    integrationType: 'shopify',
    integrationName: 'Shopify',
    icon: 'shopify',
  },

  // Social Media
  x_search_posts: {
    integrationType: 'x_twitter',
    integrationName: 'X (Twitter)',
    icon: 'x_twitter',
  },
  x_post_tweet: {
    integrationType: 'x_twitter',
    integrationName: 'X (Twitter)',
    icon: 'x_twitter',
  },
  x_get_user: {
    integrationType: 'x_twitter',
    integrationName: 'X (Twitter)',
    icon: 'x_twitter',
  },
  x_get_user_posts: {
    integrationType: 'x_twitter',
    integrationName: 'X (Twitter)',
    icon: 'x_twitter',
  },
};

// ============================================================================
// Validation Types
// ============================================================================

export interface BlockValidationIssue {
  type: 'missing_credential' | 'missing_config' | 'missing_input' | 'expired_file';
  severity: 'error' | 'warning';
  message: string;
  blockId: string;
  blockName: string;
  // For credential issues
  integrationType?: string;
  integrationName?: string;
  toolId?: string;
}

export interface WorkflowValidationResult {
  isValid: boolean;
  issues: BlockValidationIssue[];
  missingCredentials: Map<string, { integrationName: string; blocks: string[] }>;
  blocksNeedingAttention: string[];
}

// ============================================================================
// Validation Functions
// ============================================================================

/**
 * Check if a tool requires credentials
 */
export function toolRequiresCredential(toolId: string): boolean {
  return toolId in TOOL_CREDENTIAL_REQUIREMENTS;
}

/**
 * Get credential requirement for a tool
 */
export function getToolCredentialRequirement(toolId: string) {
  return TOOL_CREDENTIAL_REQUIREMENTS[toolId];
}

/**
 * Get all tools in a block that require credentials
 */
export function getBlockToolsRequiringCredentials(block: Block): string[] {
  if (block.type !== 'llm_inference') return [];

  const config = block.config as LLMInferenceConfig;
  const tools = config.enabledTools || [];

  return tools.filter(toolId => toolRequiresCredential(toolId));
}

/**
 * Validate a single block for issues
 */
export function validateBlock(
  block: Block,
  configuredCredentials: string[] = [] // Integration types that have credentials
): BlockValidationIssue[] {
  const issues: BlockValidationIssue[] = [];

  // Check LLM blocks for missing credentials
  if (block.type === 'llm_inference') {
    const config = block.config as LLMInferenceConfig;
    const tools = config.enabledTools || [];
    const blockCredentials = config.credentials || [];

    for (const toolId of tools) {
      const requirement = TOOL_CREDENTIAL_REQUIREMENTS[toolId];
      if (requirement) {
        // Check if this integration has any configured credential
        const hasCredential =
          configuredCredentials.includes(requirement.integrationType) ||
          blockCredentials.some(credId => credId.includes(requirement.integrationType));

        if (!hasCredential) {
          issues.push({
            type: 'missing_credential',
            severity: 'error',
            message: `${requirement.integrationName} credentials required for "${toolId}" tool`,
            blockId: block.id,
            blockName: block.name,
            integrationType: requirement.integrationType,
            integrationName: requirement.integrationName,
            toolId,
          });
        }
      }
    }
  }

  // Check Start block for missing input
  if (block.type === 'variable') {
    const config = block.config as VariableConfig;
    if (config.operation === 'read' && config.variableName === 'input') {
      const requiresInput = config.requiresInput !== false;
      const inputType = config.inputType || 'text';

      if (requiresInput) {
        if (inputType === 'text') {
          const hasInput =
            typeof config.defaultValue === 'string' && config.defaultValue.trim() !== '';
          if (!hasInput) {
            issues.push({
              type: 'missing_input',
              severity: 'warning',
              message: 'Test input is required to run the workflow',
              blockId: block.id,
              blockName: block.name,
            });
          }
        } else if (inputType === 'file') {
          const hasFile = config.fileValue?.fileId;
          if (!hasFile) {
            issues.push({
              type: 'missing_input',
              severity: 'warning',
              message: 'File upload is required to run the workflow',
              blockId: block.id,
              blockName: block.name,
            });
          }
        } else if (inputType === 'json') {
          const hasJson = config.jsonValue !== null && config.jsonValue !== undefined;
          if (!hasJson) {
            issues.push({
              type: 'missing_input',
              severity: 'warning',
              message: 'JSON input is required to run the workflow',
              blockId: block.id,
              blockName: block.name,
            });
          }
        }
      }

      // Check for workflow model selection
      const workflowModelId = (config as VariableConfig & { workflowModelId?: string })
        .workflowModelId;
      if (!workflowModelId) {
        issues.push({
          type: 'missing_config',
          severity: 'error',
          message: 'Workflow model must be selected',
          blockId: block.id,
          blockName: block.name,
        });
      }
    }
  }

  return issues;
}

/**
 * Validate entire workflow
 */
export function validateWorkflow(
  workflow: Workflow,
  configuredCredentials: string[] = []
): WorkflowValidationResult {
  const issues: BlockValidationIssue[] = [];
  const missingCredentials = new Map<string, { integrationName: string; blocks: string[] }>();
  const blocksNeedingAttention: string[] = [];

  for (const block of workflow.blocks) {
    const blockIssues = validateBlock(block, configuredCredentials);
    issues.push(...blockIssues);

    if (blockIssues.length > 0) {
      blocksNeedingAttention.push(block.id);
    }

    // Group credential issues by integration
    for (const issue of blockIssues) {
      if (issue.type === 'missing_credential' && issue.integrationType && issue.integrationName) {
        if (!missingCredentials.has(issue.integrationType)) {
          missingCredentials.set(issue.integrationType, {
            integrationName: issue.integrationName,
            blocks: [],
          });
        }
        const entry = missingCredentials.get(issue.integrationType)!;
        if (!entry.blocks.includes(issue.blockName)) {
          entry.blocks.push(issue.blockName);
        }
      }
    }
  }

  // Workflow is valid if there are no errors (warnings are OK)
  const hasErrors = issues.some(i => i.severity === 'error');

  return {
    isValid: !hasErrors,
    issues,
    missingCredentials,
    blocksNeedingAttention,
  };
}

/**
 * Get summary of missing integrations for display
 */
export function getMissingIntegrationsSummary(
  validation: WorkflowValidationResult
): { name: string; type: string; blockCount: number; icon: string }[] {
  const summary: { name: string; type: string; blockCount: number; icon: string }[] = [];

  for (const [integrationType, info] of validation.missingCredentials) {
    const firstTool = Object.entries(TOOL_CREDENTIAL_REQUIREMENTS).find(
      ([, req]) => req.integrationType === integrationType
    );
    summary.push({
      name: info.integrationName,
      type: integrationType,
      blockCount: info.blocks.length,
      icon: firstTool?.[1].icon || 'key',
    });
  }

  return summary.sort((a, b) => b.blockCount - a.blockCount);
}
