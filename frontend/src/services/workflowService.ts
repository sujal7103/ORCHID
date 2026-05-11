/**
 * Workflow Generation Service
 *
 * This service handles workflow generation using the dedicated structured output endpoint.
 * It ensures 100% reliable JSON workflow output without tool call interference.
 */

import { api } from './api';
import type { Workflow, Block, Connection, WorkflowVariable } from '@/types/agent';

// Conversation message for history context
export interface ConversationMessage {
  role: 'user' | 'assistant';
  content: string;
}

// Request type for workflow generation
export interface WorkflowGenerateRequest {
  user_message: string;
  current_workflow?: {
    blocks: Block[];
    connections: Connection[];
    variables: WorkflowVariable[];
  };
  model_id?: string;
  conversation_id?: string;
  conversation_history?: ConversationMessage[]; // Recent conversation for better tool selection
}

// Response type from workflow generation
export interface WorkflowGenerateResponse {
  success: boolean;
  workflow?: Workflow;
  explanation: string;
  action: 'create' | 'modify';
  error?: string;
  version: number;
  errors?: ValidationError[];
  suggested_name?: string; // AI-generated agent name suggestion (for new workflows)
  suggested_description?: string; // AI-generated agent description (for new workflows)
}

export interface ValidationError {
  type: 'schema' | 'cycle' | 'type_mismatch' | 'missing_input';
  message: string;
  blockId?: string;
  connectionId?: string;
}

/**
 * Generate or modify a workflow using AI
 *
 * This uses a dedicated endpoint with structured output to guarantee valid JSON.
 * Unlike the chat endpoint, this will never call tools or produce non-JSON output.
 *
 * @param agentId - The agent ID to generate workflow for
 * @param userMessage - The user's natural language request
 * @param currentWorkflow - Optional existing workflow for modifications
 * @param modelId - Optional model ID override
 * @param conversationHistory - Optional recent conversation messages for context (improves tool selection)
 * @returns WorkflowGenerateResponse with the generated workflow
 */
export async function generateWorkflow(
  agentId: string,
  userMessage: string,
  currentWorkflow?: Workflow | null,
  modelId?: string,
  conversationHistory?: ConversationMessage[]
): Promise<WorkflowGenerateResponse> {
  const request: WorkflowGenerateRequest = {
    user_message: userMessage,
  };

  // Include current workflow for modification requests
  if (currentWorkflow && currentWorkflow.blocks.length > 0) {
    request.current_workflow = {
      blocks: currentWorkflow.blocks,
      connections: currentWorkflow.connections,
      variables: currentWorkflow.variables || [],
    };
  }

  if (modelId) {
    request.model_id = modelId;
  }

  // Include conversation history for better context-aware tool selection
  if (conversationHistory && conversationHistory.length > 0) {
    request.conversation_history = conversationHistory;
  }

  const response = await api.post<WorkflowGenerateResponse>(
    `/api/agents/${agentId}/generate-workflow`,
    request
  );

  return response;
}

/**
 * Check if a response indicates a successful workflow generation
 */
export function isSuccessfulGeneration(
  response: WorkflowGenerateResponse
): response is WorkflowGenerateResponse & { workflow: Workflow } {
  return response.success && response.workflow !== undefined;
}

// ============================================================================
// V2 Multi-Step Generation Types
// ============================================================================

/**
 * Tool definition from the backend registry
 */
export interface ToolDefinition {
  id: string;
  name: string;
  description: string;
  category: string;
  icon: string; // Lucide icon name
  keywords: string[];
  use_cases: string[];
  parameters?: string;
}

/**
 * Tool category definition
 */
export interface ToolCategoryDefinition {
  id: string;
  name: string;
  icon: string;
  description: string;
}

/**
 * Selected tool with reasoning
 */
export interface SelectedTool {
  tool_id: string;
  category: string;
  reason: string;
}

/**
 * Tool selection result (Step 1)
 */
export interface ToolSelectionResult {
  selected_tools: SelectedTool[];
  reasoning: string;
}

/**
 * Generation step status
 */
export interface GenerationStep {
  step_number: number;
  step_name: string;
  status: 'pending' | 'running' | 'completed' | 'failed';
  description: string;
  tools?: string[]; // Tool IDs for step 1 result
}

/**
 * V2 Multi-step generation response
 */
export interface MultiStepGenerateResponse {
  success: boolean;
  current_step: number;
  total_steps: number;
  steps: GenerationStep[];
  selected_tools?: SelectedTool[];
  workflow?: Workflow;
  explanation?: string;
  error?: string;
  step_in_progress?: GenerationStep;
  action?: 'create' | 'modify';
  suggested_name?: string;
  suggested_description?: string;
}

/**
 * Tool registry response
 */
export interface ToolRegistryResponse {
  tools: ToolDefinition[];
  categories: ToolCategoryDefinition[];
}

// ============================================================================
// V2 API Functions
// ============================================================================

/**
 * Get the tool registry (all available tools and categories)
 */
export async function getToolRegistry(): Promise<ToolRegistryResponse> {
  const response = await api.get<ToolRegistryResponse>('/api/tools/registry');
  return response;
}

/**
 * Perform only tool selection (Step 1)
 * Returns the selected tools without generating the workflow
 */
export async function selectTools(
  agentId: string,
  userMessage: string,
  modelId?: string,
  conversationHistory?: ConversationMessage[]
): Promise<ToolSelectionResult> {
  const request: {
    user_message: string;
    model_id?: string;
    conversation_history?: ConversationMessage[];
  } = {
    user_message: userMessage,
  };

  if (modelId) {
    request.model_id = modelId;
  }

  // Include conversation history for better context-aware tool selection
  if (conversationHistory && conversationHistory.length > 0) {
    request.conversation_history = conversationHistory;
  }

  const response = await api.post<ToolSelectionResult>(
    `/api/agents/${agentId}/select-tools`,
    request
  );

  return response;
}

/**
 * Progress callback for multi-step generation
 */
export type GenerationProgressCallback = (
  steps: GenerationStep[],
  selectedTools?: SelectedTool[]
) => void;

/**
 * Request body for generating workflow with pre-selected tools
 */
interface GenerateWithToolsRequest {
  user_message: string;
  model_id?: string;
  selected_tools: SelectedTool[];
  current_workflow?: {
    blocks: Block[];
    connections: Connection[];
    variables: WorkflowVariable[];
  };
  conversation_history?: ConversationMessage[];
}

/**
 * Generate workflow with pre-selected tools (Step 2 only)
 */
export async function generateWithTools(
  agentId: string,
  userMessage: string,
  selectedTools: SelectedTool[],
  currentWorkflow?: Workflow | null,
  modelId?: string,
  conversationHistory?: ConversationMessage[]
): Promise<WorkflowGenerateResponse> {
  const request: GenerateWithToolsRequest = {
    user_message: userMessage,
    selected_tools: selectedTools,
  };

  if (currentWorkflow && currentWorkflow.blocks.length > 0) {
    request.current_workflow = {
      blocks: currentWorkflow.blocks,
      connections: currentWorkflow.connections,
      variables: currentWorkflow.variables || [],
    };
  }

  if (modelId) {
    request.model_id = modelId;
  }

  // Include conversation history for better context-aware workflow generation
  if (conversationHistory && conversationHistory.length > 0) {
    request.conversation_history = conversationHistory;
  }

  const response = await api.post<WorkflowGenerateResponse>(
    `/api/agents/${agentId}/generate-with-tools`,
    request
  );

  return response;
}

/**
 * Generate workflow using V2 multi-step process with REAL two-step API calls
 * Step 1: Tool Selection (separate API call)
 * Step 2: Workflow Generation with selected tools (separate API call)
 *
 * The frontend makes two sequential API calls so the user can see the progress
 * of each step in real-time with the floating tool icons.
 *
 * @param agentId - The agent ID to generate workflow for
 * @param userMessage - The user's natural language request
 * @param currentWorkflow - Optional existing workflow for modifications
 * @param modelId - Optional model ID override
 * @param onProgress - Callback for step progress updates
 * @param conversationHistory - Optional recent conversation messages for context (improves tool selection)
 * @returns MultiStepGenerateResponse with selected tools and generated workflow
 */
export async function generateWorkflowV2(
  agentId: string,
  userMessage: string,
  currentWorkflow?: Workflow | null,
  modelId?: string,
  onProgress?: GenerationProgressCallback,
  conversationHistory?: ConversationMessage[]
): Promise<MultiStepGenerateResponse> {
  // Initialize steps for progress tracking
  const steps: GenerationStep[] = [
    {
      step_number: 1,
      step_name: 'Analyzing Request',
      status: 'running',
      description: 'Identifying the best tools for your workflow...',
    },
    {
      step_number: 2,
      step_name: 'Building Workflow',
      status: 'pending',
      description: 'Generating workflow structure with selected tools...',
    },
  ];

  // Notify initial state - Step 1 running
  onProgress?.(steps);

  try {
    // ========== STEP 1: Tool Selection ==========
    const toolSelectionResult = await selectTools(
      agentId,
      userMessage,
      modelId,
      conversationHistory
    );

    // Step 1 completed - show selected tools
    steps[0] = {
      ...steps[0],
      status: 'completed',
      tools: toolSelectionResult.selected_tools.map(t => t.tool_id),
    };
    steps[1] = {
      ...steps[1],
      status: 'running',
    };
    onProgress?.(steps, toolSelectionResult.selected_tools);

    // ========== STEP 2: Workflow Generation with selected tools ==========
    const workflowResult = await generateWithTools(
      agentId,
      userMessage,
      toolSelectionResult.selected_tools,
      currentWorkflow,
      modelId,
      conversationHistory
    );

    if (!workflowResult.success) {
      // Step 2 failed
      steps[1] = {
        ...steps[1],
        status: 'failed',
      };
      onProgress?.(steps, toolSelectionResult.selected_tools);

      return {
        success: false,
        current_step: 2,
        total_steps: 2,
        steps,
        selected_tools: toolSelectionResult.selected_tools,
        error: workflowResult.error || 'Workflow generation failed',
      };
    }

    // Step 2 completed
    steps[1] = {
      ...steps[1],
      status: 'completed',
    };
    onProgress?.(steps, toolSelectionResult.selected_tools);

    return {
      success: true,
      current_step: 2,
      total_steps: 2,
      steps,
      selected_tools: toolSelectionResult.selected_tools,
      workflow: workflowResult.workflow,
      explanation: workflowResult.explanation,
      action: workflowResult.action,
      suggested_name: workflowResult.suggested_name,
      suggested_description: workflowResult.suggested_description,
    };
  } catch (error) {
    // Determine which step failed
    const failedStepIndex = steps.findIndex(s => s.status === 'running');
    if (failedStepIndex >= 0) {
      steps[failedStepIndex] = {
        ...steps[failedStepIndex],
        status: 'failed',
      };
      onProgress?.(steps);
    }

    return {
      success: false,
      current_step: failedStepIndex + 1,
      total_steps: 2,
      steps,
      error: error instanceof Error ? error.message : 'Generation failed',
    };
  }
}

/**
 * Check if a v2 response indicates successful generation
 */
export function isSuccessfulV2Generation(
  response: MultiStepGenerateResponse
): response is MultiStepGenerateResponse & { workflow: Workflow } {
  return response.success && response.workflow !== undefined;
}

// ============================================================================
// Sample Input Generation
// ============================================================================

/**
 * Response from sample input generation
 */
export interface SampleInputResponse {
  success: boolean;
  sample_input?: Record<string, unknown>;
  error?: string;
}

/**
 * Generate sample JSON input for a workflow using AI
 * Analyzes the workflow blocks and generates realistic sample data for testing
 *
 * @param agentId - The agent ID whose workflow to analyze
 * @param modelId - The model ID to use for generation
 * @returns SampleInputResponse with generated sample JSON
 */
export async function generateSampleInput(
  agentId: string,
  modelId: string
): Promise<SampleInputResponse> {
  const response = await api.post<SampleInputResponse>(
    `/api/agents/${agentId}/generate-sample-input`,
    { model_id: modelId }
  );

  return response;
}
