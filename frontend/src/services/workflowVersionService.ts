/**
 * Workflow Version Service
 *
 * This service handles workflow version history operations:
 * - Listing all versions for an agent's workflow
 * - Getting a specific version
 * - Restoring a previous version
 */

import { api } from './api';
import type { Workflow } from '@/types/agent';

// Version summary returned when listing versions
export interface WorkflowVersionSummary {
  version: number;
  description?: string;
  blockCount: number;
  createdAt: string;
}

// Response from listing versions
export interface ListVersionsResponse {
  versions: WorkflowVersionSummary[];
  count: number;
}

/**
 * List all workflow versions for an agent
 *
 * @param agentId - The agent ID to get versions for
 * @returns List of version summaries sorted by version descending (newest first)
 */
export async function listWorkflowVersions(agentId: string): Promise<WorkflowVersionSummary[]> {
  const response = await api.get<ListVersionsResponse>(`/api/agents/${agentId}/workflow/versions`);
  return response.versions || [];
}

/**
 * Get a specific workflow version
 *
 * @param agentId - The agent ID
 * @param version - The version number to retrieve
 * @returns The full workflow at that version
 */
export async function getWorkflowVersion(agentId: string, version: number): Promise<Workflow> {
  const response = await api.get<Workflow>(`/api/agents/${agentId}/workflow/versions/${version}`);
  return response;
}

/**
 * Restore a workflow to a previous version
 *
 * This creates a new version with the content from the specified version.
 * The restored workflow becomes the latest version.
 *
 * @param agentId - The agent ID
 * @param version - The version number to restore
 * @returns The new workflow (with incremented version number)
 */
export async function restoreWorkflowVersion(agentId: string, version: number): Promise<Workflow> {
  const response = await api.post<Workflow>(
    `/api/agents/${agentId}/workflow/restore/${version}`,
    {}
  );
  return response;
}

/**
 * Format a version timestamp for display
 *
 * @param dateString - ISO date string
 * @returns Formatted date string
 */
export function formatVersionDate(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) {
    return 'Just now';
  } else if (diffMins < 60) {
    return `${diffMins}m ago`;
  } else if (diffHours < 24) {
    return `${diffHours}h ago`;
  } else if (diffDays < 7) {
    return `${diffDays}d ago`;
  } else {
    return date.toLocaleDateString();
  }
}
