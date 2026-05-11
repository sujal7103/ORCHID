/**
 * Execution service
 * Handles backend API calls for execution history
 */

import { api } from './api';

// ============================================================================
// Types
// ============================================================================

export interface BlockState {
  blockId: string;
  status: string;
  inputs?: Record<string, unknown>;
  outputs?: Record<string, unknown>;
  error?: string;
  startedAt?: string;
  completedAt?: string;
  durationMs?: number;
}

export interface ExecutionRecord {
  id: string;
  agentId: string;
  userId: string;
  workflowVersion: number;

  // Trigger info
  triggerType: 'manual' | 'scheduled' | 'webhook' | 'api';
  scheduleId?: string;
  apiKeyId?: string;

  // Execution state
  status: 'pending' | 'running' | 'completed' | 'failed' | 'partial';
  input?: Record<string, unknown>;
  output?: Record<string, unknown>; // Legacy - use result/artifacts/files instead
  blockStates?: Record<string, BlockState>;
  error?: string;

  // Clean API response fields (new format)
  result?: string; // Primary text result
  artifacts?: Array<{
    // Generated charts/images
    type: string;
    format: string;
    data: string;
    title?: string;
    source_block?: string;
  }>;
  files?: Array<{
    // Generated files
    file_id?: string;
    filename: string;
    download_url: string;
    mime_type?: string;
    size?: number;
    source_block?: string;
  }>;

  // Timing
  startedAt: string;
  completedAt?: string;
  durationMs?: number;

  // TTL
  expiresAt: string;
  createdAt: string;
}

export interface PaginatedExecutions {
  executions: ExecutionRecord[];
  total: number;
  limit: number;
  offset: number;
  has_more: boolean;
}

// ============================================================================
// API Functions
// ============================================================================

/**
 * Get executions for a specific agent
 */
export async function getAgentExecutions(
  agentId: string,
  options?: {
    limit?: number;
    offset?: number;
    status?: string;
  }
): Promise<PaginatedExecutions> {
  try {
    const params = new URLSearchParams();
    if (options?.limit) params.set('limit', options.limit.toString());
    if (options?.offset) params.set('offset', options.offset.toString());
    if (options?.status) params.set('status', options.status);

    const queryString = params.toString();
    const url = `/api/agents/${agentId}/executions${queryString ? `?${queryString}` : ''}`;

    const response = await api.get<PaginatedExecutions>(url);
    return {
      executions: response.executions || [],
      total: response.total || 0,
      limit: response.limit || 20,
      offset: response.offset || 0,
      has_more: response.has_more || false,
    };
  } catch (error) {
    console.error('Failed to fetch agent executions:', error);
    return {
      executions: [],
      total: 0,
      limit: 20,
      offset: 0,
      has_more: false,
    };
  }
}

/**
 * Get a single execution by ID
 */
export async function getExecution(executionId: string): Promise<ExecutionRecord | null> {
  try {
    const response = await api.get<ExecutionRecord>(`/api/executions/${executionId}`);
    return response;
  } catch (error) {
    console.error('Failed to fetch execution:', error);
    return null;
  }
}

/**
 * Get all executions for the current user (across all agents)
 */
export async function getAllExecutions(options?: {
  limit?: number;
  offset?: number;
  status?: string;
}): Promise<PaginatedExecutions> {
  try {
    const params = new URLSearchParams();
    if (options?.limit) params.set('limit', options.limit.toString());
    if (options?.offset) params.set('offset', options.offset.toString());
    if (options?.status) params.set('status', options.status);

    const queryString = params.toString();
    const url = `/api/executions${queryString ? `?${queryString}` : ''}`;

    const response = await api.get<PaginatedExecutions>(url);
    return {
      executions: response.executions || [],
      total: response.total || 0,
      limit: response.limit || 20,
      offset: response.offset || 0,
      has_more: response.has_more || false,
    };
  } catch (error) {
    console.error('Failed to fetch executions:', error);
    return {
      executions: [],
      total: 0,
      limit: 20,
      offset: 0,
      has_more: false,
    };
  }
}

// ============================================================================
// Helper Functions
// ============================================================================

/**
 * Format duration for display
 */
export function formatDuration(durationMs?: number): string {
  if (!durationMs) return '-';

  if (durationMs < 1000) {
    return `${durationMs}ms`;
  }

  const seconds = durationMs / 1000;
  if (seconds < 60) {
    return `${seconds.toFixed(1)}s`;
  }

  const minutes = Math.floor(seconds / 60);
  const remainingSeconds = Math.floor(seconds % 60);
  return `${minutes}m ${remainingSeconds}s`;
}

/**
 * Format timestamp for display
 */
export function formatTimestamp(timestamp: string): string {
  const date = new Date(timestamp);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / (1000 * 60));
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60));
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));

  if (diffMins < 1) return 'Just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays === 1) return 'Yesterday';
  if (diffDays < 7) return `${diffDays}d ago`;

  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });
}

/**
 * Get status badge color
 */
export function getStatusColor(status: ExecutionRecord['status']): string {
  switch (status) {
    case 'completed':
      return 'var(--color-success)';
    case 'failed':
      return 'var(--color-error)';
    case 'running':
      return 'var(--color-accent)';
    case 'pending':
      return 'var(--color-warning)';
    case 'partial':
      return 'var(--color-warning)';
    default:
      return 'var(--color-text-secondary)';
  }
}

/**
 * Get trigger type display name
 */
export function getTriggerTypeLabel(triggerType: ExecutionRecord['triggerType']): string {
  switch (triggerType) {
    case 'manual':
      return 'Manual';
    case 'scheduled':
      return 'Scheduled';
    case 'api':
      return 'API';
    case 'webhook':
      return 'Webhook';
    default:
      return triggerType;
  }
}

/**
 * Get trigger type icon name (for lucide-react)
 */
export function getTriggerTypeIcon(triggerType: ExecutionRecord['triggerType']): string {
  switch (triggerType) {
    case 'manual':
      return 'Play';
    case 'scheduled':
      return 'Clock';
    case 'api':
      return 'Key';
    case 'webhook':
      return 'Globe';
    default:
      return 'Circle';
  }
}
