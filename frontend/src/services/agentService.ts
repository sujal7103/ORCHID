/**
 * Agent service
 * Handles backend API calls for agent persistence (backend-first architecture)
 */

import { api } from './api';
import type { Block, Connection, WorkflowVariable, Workflow } from '@/types/agent';

// ============================================================================
// Types
// ============================================================================

export interface AgentListItem {
  id: string;
  name: string;
  description?: string;
  status: string;
  has_workflow: boolean;
  block_count: number;
  created_at: string;
  updated_at: string;
}

export interface PaginatedAgentsResponse {
  agents: AgentListItem[];
  total: number;
  limit: number;
  offset: number;
  has_more: boolean;
}

export interface RecentAgentsResponse {
  agents: AgentListItem[];
}

export interface AgentResponse {
  id: string;
  user_id: string;
  name: string;
  description?: string;
  status: string;
  workflow?: WorkflowResponse;
  created_at: string;
  updated_at: string;
}

export interface WorkflowResponse {
  id: string;
  agent_id: string;
  blocks: Block[];
  connections: Connection[];
  variables: WorkflowVariable[];
  version: number;
  workflowModelId?: string;
  created_at: string;
  updated_at: string;
}

export interface SyncAgentRequest {
  name: string;
  description?: string;
  workflow: {
    blocks: Block[];
    connections: Connection[];
    variables?: WorkflowVariable[];
  };
  model_id?: string;
}

export interface SyncAgentResponse {
  agent: AgentResponse;
  workflow: WorkflowResponse;
  conversation_id: string;
}

export interface CreateAgentRequest {
  name: string;
  description?: string;
}

// ============================================================================
// API Functions
// ============================================================================

/**
 * Create a new agent on the backend
 */
export async function createAgent(name: string, description?: string): Promise<AgentResponse> {
  const response = await api.post<AgentResponse>('/api/agents', {
    name,
    description: description || '',
  });
  return response;
}

/**
 * Get recent agents for landing page (10 most recently updated)
 */
export async function getRecentAgents(): Promise<AgentListItem[]> {
  try {
    const response = await api.get<RecentAgentsResponse>('/api/agents/recent');
    return response.agents || [];
  } catch (error) {
    console.error('Failed to fetch recent agents:', error);
    return [];
  }
}

/**
 * Get paginated list of agents
 */
export async function getAgentsPaginated(limit = 20, offset = 0): Promise<PaginatedAgentsResponse> {
  try {
    const response = await api.get<PaginatedAgentsResponse>(
      `/api/agents?limit=${limit}&offset=${offset}`
    );
    return {
      agents: response.agents || [],
      total: response.total,
      limit: response.limit,
      offset: response.offset,
      has_more: response.has_more,
    };
  } catch (error) {
    console.error('Failed to fetch agents:', error);
    return {
      agents: [],
      total: 0,
      limit,
      offset,
      has_more: false,
    };
  }
}

/**
 * Get a single agent by ID with full workflow
 */
export async function getAgent(agentId: string): Promise<AgentResponse | null> {
  try {
    const response = await api.get<AgentResponse>(`/api/agents/${agentId}`);
    return response;
  } catch (error) {
    console.error('Failed to fetch agent:', error);
    return null;
  }
}

/**
 * Get workflow for an agent
 */
export async function getAgentWorkflow(agentId: string): Promise<Workflow | null> {
  try {
    const response = await api.get<WorkflowResponse>(`/api/agents/${agentId}/workflow`);
    return {
      blocks: response.blocks || [],
      connections: response.connections || [],
      variables: response.variables || [],
    };
  } catch (error) {
    console.error('Failed to fetch agent workflow:', error);
    return null;
  }
}

/**
 * Sync a local agent to backend on first message
 * This creates/updates the agent, workflow, and conversation atomically
 */
export async function syncAgentToBackend(
  agentId: string,
  request: SyncAgentRequest
): Promise<SyncAgentResponse | null> {
  try {
    const response = await api.post<SyncAgentResponse>(`/api/agents/${agentId}/sync`, request);
    return response;
  } catch (error) {
    console.error('Failed to sync agent to backend:', error);
    throw error; // Re-throw to let caller handle
  }
}

/**
 * Update agent name/description
 */
export async function updateAgent(
  agentId: string,
  updates: { name?: string; description?: string; status?: string }
): Promise<AgentResponse | null> {
  try {
    const response = await api.put<AgentResponse>(`/api/agents/${agentId}`, updates);
    return response;
  } catch (error) {
    console.error('Failed to update agent:', error);
    return null;
  }
}

/**
 * Delete an agent
 */
export async function deleteAgent(agentId: string): Promise<boolean> {
  try {
    await api.delete(`/api/agents/${agentId}`);
    return true;
  } catch (error) {
    console.error('Failed to delete agent:', error);
    return false;
  }
}

/**
 * Save workflow for an agent
 */
export async function saveAgentWorkflow(
  agentId: string,
  workflow: { blocks: Block[]; connections: Connection[]; variables?: WorkflowVariable[] }
): Promise<WorkflowResponse | null> {
  try {
    const response = await api.put<WorkflowResponse>(`/api/agents/${agentId}/workflow`, workflow);
    return response;
  } catch (error) {
    console.error('Failed to save agent workflow:', error);
    return null;
  }
}

// Webhook info returned by the backend
export interface AgentWebhookInfo {
  id: string;
  agentId: string;
  path: string;
  webhookUrl: string;
  method: string;
  hasSecret: boolean;
  enabled: boolean;
  createdAt: string;
}

/**
 * Get the webhook configuration for a deployed agent
 * Returns null if the agent has no webhook registered
 */
export async function getAgentWebhook(agentId: string): Promise<AgentWebhookInfo | null> {
  try {
    const response = await api.get<AgentWebhookInfo>(`/api/agents/${agentId}/webhook`);
    return response;
  } catch {
    // 404 = no webhook configured, not an error
    return null;
  }
}

// ============================================================================
// AI Auto-Fill
// ============================================================================

export interface AutoFillRequest {
  model_id: string;
  block_type: string;
  block_name: string;
  tool_name?: string;
  tool_schema?: Record<string, unknown>;
  current_config: Record<string, unknown>;
  upstream_data: Record<string, Record<string, unknown>>;
  user_context?: string; // Extra context like sheet IDs, column names, channel names
}

export interface AutoFillResponse {
  config: Record<string, unknown>;
}

/**
 * Uses AI to auto-fill a block's configuration based on upstream execution data.
 * Requires the workflow to have been run at least once so upstream outputs exist.
 */
export async function autoFillBlock(params: AutoFillRequest): Promise<AutoFillResponse> {
  return api.post<AutoFillResponse>('/api/agents/autofill', params);
}

// ============================================================================
// Single Block Test Execution
// ============================================================================

export interface TestBlockRequest {
  block: {
    id: string;
    normalizedId: string;
    type: string;
    name: string;
    description: string;
    config: Record<string, unknown>;
    timeout: number;
  };
  upstream_outputs: Record<string, Record<string, unknown>>;
  test_payload?: Record<string, unknown>; // For trigger blocks: simulated incoming data
}

export interface TestBlockResponse {
  status: 'completed' | 'failed';
  output?: Record<string, unknown>;
  error?: string;
  duration_ms: number;
  _note?: string;
}

/**
 * Executes a single block in isolation with provided upstream data.
 * Used for testing individual blocks without running the full workflow.
 */
export async function testBlock(params: TestBlockRequest): Promise<TestBlockResponse> {
  return api.post<TestBlockResponse>('/api/agents/test-block', params);
}
