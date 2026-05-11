import { create } from 'zustand';
import { devtools, persist } from 'zustand/middleware';
import type { Node, Edge } from '@xyflow/react';
import type {
  Agent,
  Workflow,
  Block,
  Connection,
  BuilderMessage,
  BlockExecutionState,
  ExecutionStatus,
  WorkflowVariable,
  SyncStatus,
  AgentStatus,
  ExecutionAPIResponse,
  ForEachIterationState,
  IterationState,
} from '@/types/agent';
import { api } from '@/services/api';
import type { ExecutionRecord } from '@/services/executionService';
import { normalizeBlockName } from '@/utils/blockUtils';
import { getSmartLayout } from '@/utils/workflowLayout';
import {
  getOrCreateBuilderConversation,
  addBuilderMessage as addBuilderMessageAPI,
  listBuilderConversations,
  type BuilderConversationListItem,
} from '@/services/conversationService';
import {
  getRecentAgents,
  getAgentsPaginated,
  syncAgentToBackend as syncAgentAPI,
  getAgent as getAgentAPI,
  createAgent as createAgentAPI,
  type AgentListItem,
} from '@/services/agentService';
import {
  listWorkflowVersions,
  restoreWorkflowVersion as restoreWorkflowVersionAPI,
  type WorkflowVersionSummary,
} from '@/services/workflowVersionService';

// Helper function to get user-specific storage name
function getUserStorageName(baseName: string): string {
  try {
    const authStorage = localStorage.getItem('ca-auth');
    if (authStorage) {
      const { state } = JSON.parse(authStorage);
      if (state?.user?.id) {
        return `${baseName}-${state.user.id}`;
      }
    }
  } catch (error) {
    console.warn('Failed to get user ID for storage name:', error);
  }
  return baseName;
}

// Custom storage for UI state only (agents are fully API-based)
function createAgentStorage() {
  return {
    getItem: (_name: string) => {
      const userSpecificName = getUserStorageName('agent-builder-storage');
      const str = localStorage.getItem(userSpecificName);
      if (!str) return null;

      const { state } = JSON.parse(str);
      return { state };
    },
    setItem: (_name: string, value: unknown) => {
      const userSpecificName = getUserStorageName('agent-builder-storage');
      localStorage.setItem(userSpecificName, JSON.stringify(value));
    },
    removeItem: (_name: string) => {
      const userSpecificName = getUserStorageName('agent-builder-storage');
      localStorage.removeItem(userSpecificName);
    },
  };
}

// =============================================================================
// Types
// =============================================================================

export type AgentView = 'my-agents' | 'deployed' | 'canvas' | 'onboarding';

// Workflow version for AI-created snapshots (stored in backend)
export interface WorkflowVersion {
  id: string;
  version: number;
  description?: string;
  blockCount: number;
  createdAt: Date;
  // Full workflow data (only populated when restored)
  workflow?: Workflow;
}

// =============================================================================
// State Interface
// =============================================================================

// Pagination state
interface PaginationState {
  total: number;
  hasMore: boolean;
  isLoading: boolean;
}

interface AgentBuilderState {
  // Agents list - fully API-based (no localStorage)
  backendAgents: AgentListItem[]; // Agents from backend (lightweight list items)
  selectedAgentId: string | null;

  // Backend pagination
  pagination: PaginationState;
  isLoadingAgents: boolean;

  // Current workflow being edited
  currentAgent: Agent | null;
  workflow: Workflow | null;

  // Workflow history for undo/redo
  workflowHistory: Workflow[];
  workflowFuture: Workflow[];

  // AI-created workflow versions (for version dropdown)
  workflowVersions: WorkflowVersion[];

  // Dirty state tracking (unsaved changes detection)
  isDirty: boolean;
  lastSavedAt: Date | null;

  // Agent switch guard
  pendingAgentSwitch: { agentId: string; callback?: () => void } | null;
  showUnsavedChangesDialog: boolean;

  // React Flow state (derived from workflow)
  nodes: Node[];
  edges: Edge[];

  // Execution state
  executionId: string | null;
  executionStatus: ExecutionStatus | null;
  blockStates: Record<string, BlockExecutionState>;
  blockOutputCache: Record<string, unknown>;
  forEachStates: Record<string, ForEachIterationState>; // Per-block for-each iteration state
  lastExecutionResult: ExecutionAPIResponse | null; // Clean API response from last execution

  // UI state
  selectedBlockId: string | null;
  highlightedUpstreamIds: string[]; // Blocks this block depends on (upstream)
  highlightedDownstreamIds: string[]; // Blocks that depend on this block (downstream)
  highlightedEdgeIds: string[]; // Edges connecting to/from selected block
  isSidebarOpen: boolean;
  isSidebarCollapsed: boolean;
  activeView: AgentView;
  recentAgentIds: string[];

  // Debug mode - enables block checker for validation
  debugMode: boolean;

  // Execution viewer mode (n8n-style)
  executionViewerMode: 'editor' | 'executions';
  selectedExecutionId: string | null;
  selectedExecutionData: ExecutionRecord | null;
  inspectedBlockId: string | null;

  // Builder chat
  builderMessages: BuilderMessage[];
  isGenerating: boolean;
  showOnboardingGuide: boolean;
  showOnboardingGuidance: boolean;
  showPostExecutionGuidance: boolean;
  highlightChatInput: boolean;

  // Conversation persistence (MongoDB)
  currentConversationId: string | null;
  conversationList: BuilderConversationListItem[];
  isLoadingConversation: boolean;

  // Pending chat message (for "Fix with Agent" feature)
  pendingChatMessage: string | null;

  // ==========================================================================
  // Actions - Agent Management
  // ==========================================================================

  createAgent: (name: string, description?: string) => Promise<Agent | null>;
  selectAgent: (agentId: string | null) => void;
  updateAgent: (agentId: string, updates: Partial<Agent>) => void;
  deleteAgent: (agentId: string) => void;
  duplicateAgent: (agentId: string) => Agent;

  // ==========================================================================
  // Actions - Workflow Management
  // ==========================================================================

  setWorkflow: (workflow: Workflow, skipHistory?: boolean) => void;
  setWorkflowModelId: (modelId: string) => void;
  updateAgentStatus: (status: AgentStatus) => Promise<void>;
  addBlock: (block: Block) => void;
  updateBlock: (blockId: string, updates: Partial<Block>) => void;
  removeBlock: (blockId: string) => void;
  addConnection: (connection: Connection) => void;
  removeConnection: (connectionId: string) => void;
  addVariable: (variable: WorkflowVariable) => void;
  removeVariable: (variableName: string) => void;

  // Undo/Redo
  canUndo: () => boolean;
  canRedo: () => boolean;
  undo: () => void;
  redo: () => void;
  clearHistory: () => void;

  // Workflow Versions (AI-created snapshots, stored in backend)
  loadWorkflowVersions: (agentId: string) => Promise<void>;
  saveWorkflowVersion: (description?: string) => void; // Local tracking only - backend saves automatically
  restoreWorkflowVersion: (versionNumber: number) => Promise<void>;
  clearWorkflowVersions: () => void;

  // ==========================================================================
  // Actions - React Flow Sync
  // ==========================================================================

  syncNodesAndEdges: () => void;
  onNodesChange: (changes: unknown[]) => void;
  onEdgesChange: (changes: unknown[]) => void;
  onConnect: (connection: { source: string; target: string; sourceHandle?: string | null }) => void;
  autoLayoutWorkflow: () => void;

  // ==========================================================================
  // Actions - Block Selection & UI
  // ==========================================================================

  selectBlock: (blockId: string | null) => void;
  toggleSidebar: () => void;
  toggleSidebarCollapsed: () => void;
  setActiveView: (view: AgentView) => void;
  trackAgentAccess: (agentId: string) => void;
  toggleDebugMode: () => void;

  // ==========================================================================
  // Actions - Builder Chat
  // ==========================================================================
  addBuilderMessage: (message: Omit<BuilderMessage, 'id' | 'timestamp'>) => void;
  clearBuilderMessages: () => void;
  setIsGenerating: (isGenerating: boolean) => void;
  setShowOnboardingGuide: (show: boolean) => void;
  setShowOnboardingGuidance: (show: boolean) => void;
  setShowPostExecutionGuidance: (show: boolean) => void;
  setHighlightChatInput: (highlight: boolean) => void;
  setPendingChatMessage: (message: string | null) => void;

  // ==========================================================================
  // Actions - Conversation Persistence
  // ==========================================================================

  loadConversation: (agentId: string, modelId?: string) => Promise<void>;
  persistMessage: (message: BuilderMessage) => Promise<void>;
  fetchConversationList: (agentId: string) => Promise<void>;
  selectConversation: (conversationId: string) => Promise<void>;
  clearConversation: () => void;

  // ==========================================================================
  // Actions - Execution
  // ==========================================================================

  startExecution: (executionId: string) => void;
  updateBlockExecution: (blockId: string, state: Partial<BlockExecutionState>) => void;
  completeExecution: (status: ExecutionStatus, apiResponse?: ExecutionAPIResponse) => void;
  cacheBlockOutput: (blockId: string, output: unknown) => void;
  clearExecution: () => void;
  updateForEachIteration: (
    blockId: string,
    iteration: number,
    totalItems: number,
    currentItem: unknown
  ) => void;
  setForEachResults: (blockId: string, iterationResults: Record<string, unknown>[]) => void;
  clearForEachStates: () => void;

  // ==========================================================================
  // Actions - Execution Viewer
  // ==========================================================================

  setExecutionViewerMode: (mode: 'editor' | 'executions') => void;
  selectExecution: (executionId: string | null) => void;
  setSelectedExecutionData: (data: ExecutionRecord | null) => void;
  setInspectedBlockId: (blockId: string | null) => void;

  // ==========================================================================
  // Actions - Persistence
  // ==========================================================================

  saveCurrentAgent: (createVersion?: boolean, versionDescription?: string) => Promise<void>;
  markClean: () => void;

  // Agent switch guard
  requestAgentSwitch: (agentId: string, callback?: () => void) => boolean;
  confirmAgentSwitch: () => void;
  saveAndSwitch: () => Promise<void>;
  cancelAgentSwitch: () => void;

  // ==========================================================================
  // Actions - Backend-First Sync
  // ==========================================================================

  fetchRecentAgents: () => Promise<void>;
  fetchAgentsPage: (offset?: number) => Promise<void>;
  syncAgentToBackend: (agentId: string) => Promise<{ conversationId: string } | null>;
  updateAgentSyncStatus: (agentId: string, status: SyncStatus, error?: string) => void;
  loadAgentFromBackend: (agentId: string) => Promise<void>;
}

// =============================================================================
// Helper Functions
// =============================================================================

function generateId(): string {
  return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
}

function createEmptyWorkflow(): Workflow {
  return {
    id: generateId(),
    blocks: [],
    connections: [],
    variables: [],
    version: 1,
  };
}

function workflowToNodesAndEdges(workflow: Workflow | null): {
  nodes: Node[];
  edges: Edge[];
} {
  if (!workflow) {
    return { nodes: [], edges: [] };
  }

  const nodes: Node[] = workflow.blocks.map(block => ({
    id: block.id,
    type: 'blockNode', // Custom node type
    position: block.position,
    data: { block },
  }));

  const edges: Edge[] = workflow.connections.map(conn => ({
    id: conn.id,
    source: conn.sourceBlockId,
    target: conn.targetBlockId,
    // Map sourceOutput to sourceHandle for multi-output blocks (e.g., if_condition true/false)
    ...(conn.sourceOutput && conn.sourceOutput !== 'output'
      ? { sourceHandle: conn.sourceOutput }
      : {}),

    animated: false,
    selectable: true,
    style: {
      strokeWidth: 2,
    },
  }));

  return { nodes, edges };
}

/**
 * Auto-connect blocks based on template references in their config.
 * If a block uses {{other-block.response}}, create a connection from other-block to this block.
 * This ensures visual edges match the actual data dependencies.
 */
function autoConnectByTemplateReferences(workflow: Workflow): Workflow {
  // Build a map of normalizedId -> blockId for template matching
  const normalizedIdToBlockId = new Map<string, string>();
  for (const block of workflow.blocks) {
    if (block.normalizedId) {
      normalizedIdToBlockId.set(block.normalizedId, block.id);
    }
    // Also map by block name (lowercase, with spaces as dashes)
    const nameKey = block.name.toLowerCase().replace(/\s+/g, '-');
    normalizedIdToBlockId.set(nameKey, block.id);
  }

  // Find the start block
  const startBlock = workflow.blocks.find(b => {
    if (b.type !== 'variable') return false;
    const cfg = b.config as { operation?: string; variableName?: string };
    return cfg?.operation === 'read' && cfg?.variableName === 'input';
  });

  // Build a set of existing connections for quick lookup
  const existingConnections = new Set<string>();
  for (const conn of workflow.connections) {
    existingConnections.add(`${conn.sourceBlockId}->${conn.targetBlockId}`);
  }

  // Template regex to find {{block-name.field}} references
  const templateRegex = /\{\{([a-zA-Z0-9_-]+)\./g;
  const newConnections: Connection[] = [];

  for (const block of workflow.blocks) {
    // Skip the start block
    if (startBlock && block.id === startBlock.id) continue;

    // Stringify the block config to find all template references
    const configStr = JSON.stringify(block.config || {});
    let match;
    const referencedBlockIds = new Set<string>();

    while ((match = templateRegex.exec(configStr)) !== null) {
      const refName = match[1];

      // Handle 'start' or 'input' references
      if (refName === 'start' || refName === 'input') {
        if (startBlock) {
          referencedBlockIds.add(startBlock.id);
        }
        continue;
      }

      // Look up by normalizedId
      const foundBlockId = normalizedIdToBlockId.get(refName);
      if (foundBlockId && foundBlockId !== block.id) {
        referencedBlockIds.add(foundBlockId);
      }
    }

    // Create connections for any referenced blocks that don't have edges yet
    for (const sourceBlockId of referencedBlockIds) {
      const connectionKey = `${sourceBlockId}->${block.id}`;
      if (!existingConnections.has(connectionKey)) {
        newConnections.push({
          id: generateId(),
          sourceBlockId: sourceBlockId,
          sourceOutput: 'output',
          targetBlockId: block.id,
          targetInput: `from_${sourceBlockId}`,
        });
        existingConnections.add(connectionKey); // Prevent duplicates
        console.log(
          `🔗 [AUTO-CONNECT] Created edge: ${sourceBlockId} → ${block.id} (template reference)`
        );
      }
    }
  }

  // If we found new connections, return updated workflow
  if (newConnections.length > 0) {
    console.log(`🔗 [AUTO-CONNECT] Added ${newConnections.length} missing connections`);
    return {
      ...workflow,
      connections: [...workflow.connections, ...newConnections],
    };
  }

  return workflow;
}

/**
 * Create a stable JSON snapshot of workflow for dirty comparison.
 * Excludes position (drag moves are cosmetic, not structural changes).
 */
function workflowSnapshot(workflow: Workflow | null): string | null {
  if (!workflow) return null;
  const normalized = {
    blocks: workflow.blocks.map(b => ({
      id: b.id,
      type: b.type,
      name: b.name,
      normalizedId: b.normalizedId,
      config: b.config,
    })),
    connections: workflow.connections.map(c => ({
      id: c.id,
      sourceBlockId: c.sourceBlockId,
      targetBlockId: c.targetBlockId,
      sourceOutput: c.sourceOutput,
      targetInput: c.targetInput,
    })),
    variables: workflow.variables,
    workflowModelId: workflow.workflowModelId,
  };
  return JSON.stringify(normalized);
}

// =============================================================================
// Store Implementation
// =============================================================================

export const useAgentBuilderStore = create<AgentBuilderState>()(
  devtools(
    persist(
      (set, get) => ({
        // Initial state - fully API-based (no localStorage for agents)
        backendAgents: [],
        selectedAgentId: null,
        pagination: { total: 0, hasMore: false, isLoading: false },
        isLoadingAgents: false,
        currentAgent: null,
        workflow: null,
        workflowHistory: [],
        workflowFuture: [],
        workflowVersions: [],
        isDirty: false,
        lastSavedAt: null,
        pendingAgentSwitch: null,
        showUnsavedChangesDialog: false,
        nodes: [],
        edges: [],
        executionId: null,
        executionStatus: null,
        blockStates: {},
        blockOutputCache: {},
        forEachStates: {},
        lastExecutionResult: null,
        selectedBlockId: null,
        highlightedUpstreamIds: [],
        highlightedDownstreamIds: [],
        highlightedEdgeIds: [],
        isSidebarOpen: true,
        isSidebarCollapsed: false,
        activeView: 'my-agents',
        recentAgentIds: [],
        debugMode: false, // Debug mode enables block checker validation
        executionViewerMode: 'editor',
        selectedExecutionId: null,
        selectedExecutionData: null,
        inspectedBlockId: null,
        builderMessages: [],
        isGenerating: false,
        showOnboardingGuide: false,
        showOnboardingGuidance: false,
        showPostExecutionGuidance: false,
        highlightChatInput: false,
        currentConversationId: null,
        conversationList: [],
        isLoadingConversation: false,
        pendingChatMessage: null,

        // =====================================================================
        // Agent Management
        // =====================================================================

        createAgent: async (name, description = '') => {
          set({ isLoadingAgents: true });

          try {
            // Create agent on backend immediately
            const response = await createAgentAPI(name, description);

            // Create local agent object from backend response
            const workflow = createEmptyWorkflow();
            const newAgent: Agent = {
              id: response.id,
              userId: response.user_id,
              name: response.name,
              description: response.description || '',
              workflow,
              status: (response.status as 'draft' | 'deployed' | 'paused') || 'draft',
              createdAt: new Date(response.created_at),
              updatedAt: new Date(response.updated_at),
              syncStatus: 'synced', // Already on backend
            };

            const { nodes, edges } = workflowToNodesAndEdges(workflow);

            // Create list item for backendAgents
            const newAgentListItem: AgentListItem = {
              id: response.id,
              name: response.name,
              description: response.description,
              status: response.status,
              has_workflow: false,
              block_count: 1,
              created_at: response.created_at,
              updated_at: response.updated_at,
            };

            set(state => ({
              selectedAgentId: newAgent.id,
              currentAgent: newAgent,
              workflow: newAgent.workflow,
              nodes,
              edges,
              builderMessages: [],
              currentConversationId: null,
              isLoadingAgents: false,
              // Clear version history for new agent
              workflowVersions: [],
              workflowHistory: [],
              workflowFuture: [],
              // Add new agent to the front of backendAgents list
              backendAgents: [newAgentListItem, ...state.backendAgents],
            }));

            // New agent starts clean
            get().markClean();

            console.log('✅ [STORE] Created agent on backend:', newAgent.id);
            return newAgent;
          } catch (error) {
            console.error('❌ [STORE] Failed to create agent on backend:', error);
            set({ isLoadingAgents: false });
            return null;
          }
        },

        selectAgent: agentId => {
          // For API-based agents, just set the ID and let loadAgentFromBackend handle it
          // If currentAgent already matches, keep it
          const { currentAgent } = get();
          if (currentAgent?.id === agentId) {
            return; // Already selected
          }

          set({
            selectedAgentId: agentId,
            currentAgent: null,
            workflow: null,
            nodes: [],
            edges: [],
            selectedBlockId: null,
            builderMessages: [],
            // Clear execution state when switching agents
            executionId: null,
            executionStatus: null,
            blockStates: {},
            forEachStates: {},
            blockOutputCache: {},
            lastExecutionResult: null,
            // Clear version history when switching agents
            workflowHistory: [],
            workflowFuture: [],
            workflowVersions: [],
          });
        },

        updateAgent: (agentId, updates) => {
          // Update currentAgent AND backendAgents list
          set(state => {
            const updatedCurrentAgent =
              state.currentAgent?.id === agentId
                ? { ...state.currentAgent, ...updates, updatedAt: new Date() }
                : state.currentAgent;

            // Check if agent exists in backendAgents
            const existsInBackend = state.backendAgents.some(a => a.id === agentId);

            let updatedBackendAgents;
            if (existsInBackend) {
              // Update existing agent
              updatedBackendAgents = state.backendAgents.map(agent => {
                if (agent.id === agentId) {
                  // Extract only common fields that might change
                  const { name, description, status } = updates;
                  return {
                    ...agent,
                    ...(name !== undefined ? { name } : {}),
                    ...(description !== undefined ? { description } : {}),
                    ...(status !== undefined ? { status } : {}),
                    updated_at: new Date().toISOString(),
                  };
                }
                return agent;
              });
            } else if (updatedCurrentAgent && updatedCurrentAgent.id === agentId) {
              // Add new agent to list (at the beginning for recency)
              const newItem: AgentListItem = {
                id: updatedCurrentAgent.id,
                name: updatedCurrentAgent.name,
                description: updatedCurrentAgent.description,
                status: updatedCurrentAgent.status,
                has_workflow: !!updatedCurrentAgent.workflow,
                block_count: updatedCurrentAgent.workflow?.blocks.length || 0,
                created_at: updatedCurrentAgent.createdAt.toISOString(),
                updated_at: updatedCurrentAgent.updatedAt.toISOString(),
              };
              updatedBackendAgents = [newItem, ...state.backendAgents];
            } else {
              updatedBackendAgents = state.backendAgents;
            }

            return {
              currentAgent: updatedCurrentAgent,
              backendAgents: updatedBackendAgents,
            };
          });

          // Persist to backend (fire and forget for name/description updates)
          // Skip API call if backend already saved (during workflow generation)
          api
            .put(`/api/agents/${agentId}`, {
              name: updates.name,
              description: updates.description,
              status: updates.status,
            })
            .catch(err => {
              console.warn('⚠️ [STORE] Failed to persist agent update:', err);
            });
        },

        deleteAgent: agentId => {
          // Remove from backendAgents list and clear if selected
          set(state => {
            const wasSelected = state.selectedAgentId === agentId;
            return {
              backendAgents: state.backendAgents.filter(a => a.id !== agentId),
              selectedAgentId: wasSelected ? null : state.selectedAgentId,
              currentAgent: wasSelected ? null : state.currentAgent,
              workflow: wasSelected ? null : state.workflow,
              nodes: wasSelected ? [] : state.nodes,
              edges: wasSelected ? [] : state.edges,
            };
          });
        },

        duplicateAgent: _agentId => {
          // Duplicating requires creating a new agent on backend
          // For now, just use createAgent with a copy name
          const { currentAgent } = get();
          if (!currentAgent) {
            throw new Error('No agent selected to duplicate');
          }

          // Return a placeholder - the actual duplication should be done via createAgent
          // This is a simplified version - full implementation would copy workflow too
          console.warn('⚠️ [STORE] duplicateAgent should use createAgent for API-based agents');
          return currentAgent; // Return current as placeholder
        },

        // =====================================================================
        // Workflow Management
        // =====================================================================

        setWorkflow: (workflow, skipHistory = false) => {
          // Auto-connect blocks based on template references (e.g., {{block-name.response}})
          // This ensures visual edges match actual data dependencies
          const connectedWorkflow = autoConnectByTemplateReferences(workflow);

          set(state => {
            const { nodes, edges } = workflowToNodesAndEdges(connectedWorkflow);

            // Push current workflow to history (unless skipping or no previous workflow)
            const newHistory =
              !skipHistory && state.workflow
                ? [...state.workflowHistory.slice(-49), state.workflow] // Keep last 50 states
                : state.workflowHistory;

            return {
              workflow: connectedWorkflow,
              nodes,
              edges,
              isDirty: true,
              workflowHistory: newHistory,
              workflowFuture: skipHistory ? state.workflowFuture : [], // Clear future on new changes
              currentAgent: state.currentAgent
                ? { ...state.currentAgent, workflow: connectedWorkflow, updatedAt: new Date() }
                : null,
            };
          });

          // Auto-layout after workflow is set (with small delay to ensure state is updated)
          setTimeout(() => {
            get().autoLayoutWorkflow();
          }, 100);
        },

        setWorkflowModelId: (modelId: string) => {
          set(state => {
            if (!state.workflow) return state;
            const updated = { ...state.workflow, workflowModelId: modelId };
            return {
              workflow: updated,
              isDirty: true,
              currentAgent: state.currentAgent
                ? { ...state.currentAgent, workflow: updated, updatedAt: new Date() }
                : null,
            };
          });
        },

        updateAgentStatus: async (status: AgentStatus) => {
          const { currentAgent } = get();
          if (!currentAgent) return;
          try {
            await api.put(`/api/agents/${currentAgent.id}`, { status });
            set({ currentAgent: { ...currentAgent, status, updatedAt: new Date() } });
          } catch (error) {
            console.error('Failed to update agent status:', error);
          }
        },

        addBlock: block => {
          set(state => {
            if (!state.workflow) return state;

            const newWorkflow = {
              ...state.workflow,
              blocks: [...state.workflow.blocks, block],
            };

            const { nodes, edges } = workflowToNodesAndEdges(newWorkflow);

            return {
              workflow: newWorkflow,
              nodes,
              edges,
              isDirty: true,
              // Push current workflow to history
              workflowHistory: [...state.workflowHistory.slice(-49), state.workflow],
              workflowFuture: [], // Clear future on new changes
            };
          });
        },

        updateBlock: (blockId, updates) => {
          set(state => {
            if (!state.workflow) return state;

            const newWorkflow = {
              ...state.workflow,
              blocks: state.workflow.blocks.map(block => {
                if (block.id !== blockId) return block;

                // Regenerate normalizedId if name changed
                const updatedBlock = { ...block, ...updates };
                if (updates.name !== undefined) {
                  updatedBlock.normalizedId = normalizeBlockName(updates.name);
                }
                return updatedBlock;
              }),
            };

            const { nodes, edges } = workflowToNodesAndEdges(newWorkflow);

            return {
              workflow: newWorkflow,
              nodes,
              edges,
              isDirty: true,
              // Push current workflow to history
              workflowHistory: [...state.workflowHistory.slice(-49), state.workflow],
              workflowFuture: [], // Clear future on new changes
            };
          });
        },

        removeBlock: blockId => {
          set(state => {
            if (!state.workflow) return state;

            const newWorkflow = {
              ...state.workflow,
              blocks: state.workflow.blocks.filter(b => b.id !== blockId),
              // Also remove connections involving this block
              connections: state.workflow.connections.filter(
                c => c.sourceBlockId !== blockId && c.targetBlockId !== blockId
              ),
            };

            const { nodes, edges } = workflowToNodesAndEdges(newWorkflow);

            return {
              workflow: newWorkflow,
              nodes,
              edges,
              isDirty: true,
              selectedBlockId: state.selectedBlockId === blockId ? null : state.selectedBlockId,
              // Push current workflow to history
              workflowHistory: [...state.workflowHistory.slice(-49), state.workflow],
              workflowFuture: [], // Clear future on new changes
            };
          });
        },

        addConnection: connection => {
          set(state => {
            if (!state.workflow) return state;

            const newWorkflow = {
              ...state.workflow,
              connections: [...state.workflow.connections, connection],
            };

            const { nodes, edges } = workflowToNodesAndEdges(newWorkflow);

            return {
              workflow: newWorkflow,
              nodes,
              edges,
              isDirty: true,
              // Push current workflow to history
              workflowHistory: [...state.workflowHistory.slice(-49), state.workflow],
              workflowFuture: [], // Clear future on new changes
            };
          });
        },

        removeConnection: connectionId => {
          set(state => {
            if (!state.workflow) return state;

            const newWorkflow = {
              ...state.workflow,
              connections: state.workflow.connections.filter(c => c.id !== connectionId),
            };

            const { nodes, edges } = workflowToNodesAndEdges(newWorkflow);

            return {
              workflow: newWorkflow,
              nodes,
              edges,
              isDirty: true,
              // Push current workflow to history
              workflowHistory: [...state.workflowHistory.slice(-49), state.workflow],
              workflowFuture: [], // Clear future on new changes
            };
          });
        },

        addVariable: variable => {
          set(state => {
            if (!state.workflow) return state;

            return {
              workflow: {
                ...state.workflow,
                variables: [...state.workflow.variables, variable],
              },
              isDirty: true,
              // Push current workflow to history
              workflowHistory: [...state.workflowHistory.slice(-49), state.workflow],
              workflowFuture: [], // Clear future on new changes
            };
          });
        },

        removeVariable: variableName => {
          set(state => {
            if (!state.workflow) return state;

            return {
              workflow: {
                ...state.workflow,
                variables: state.workflow.variables.filter(v => v.name !== variableName),
              },
              isDirty: true,
              // Push current workflow to history
              workflowHistory: [...state.workflowHistory.slice(-49), state.workflow],
              workflowFuture: [], // Clear future on new changes
            };
          });
        },

        // =====================================================================
        // Undo/Redo
        // =====================================================================

        canUndo: () => {
          return get().workflowHistory.length > 0;
        },

        canRedo: () => {
          return get().workflowFuture.length > 0;
        },

        undo: () => {
          const { workflow, workflowHistory, workflowFuture } = get();
          if (workflowHistory.length === 0 || !workflow) return;

          // Get the previous state
          const previousWorkflow = workflowHistory[workflowHistory.length - 1];
          const newHistory = workflowHistory.slice(0, -1);

          // Convert to nodes/edges
          const { nodes, edges } = workflowToNodesAndEdges(previousWorkflow);

          set(state => ({
            workflow: previousWorkflow,
            workflowHistory: newHistory,
            workflowFuture: [workflow, ...workflowFuture.slice(0, 49)], // Keep last 50 future states
            nodes,
            edges,
            currentAgent: state.currentAgent
              ? { ...state.currentAgent, workflow: previousWorkflow, updatedAt: new Date() }
              : null,
          }));
        },

        redo: () => {
          const { workflow, workflowHistory, workflowFuture } = get();
          if (workflowFuture.length === 0) return;

          // Get the next state
          const nextWorkflow = workflowFuture[0];
          const newFuture = workflowFuture.slice(1);

          // Convert to nodes/edges
          const { nodes, edges } = workflowToNodesAndEdges(nextWorkflow);

          set(state => ({
            workflow: nextWorkflow,
            workflowHistory: workflow ? [...workflowHistory.slice(-49), workflow] : workflowHistory,
            workflowFuture: newFuture,
            nodes,
            edges,
            currentAgent: state.currentAgent
              ? { ...state.currentAgent, workflow: nextWorkflow, updatedAt: new Date() }
              : null,
          }));
        },

        clearHistory: () => {
          set({ workflowHistory: [], workflowFuture: [] });
        },

        // =====================================================================
        // Workflow Versions (AI-created snapshots, stored in backend)
        // =====================================================================

        loadWorkflowVersions: async (agentId: string) => {
          try {
            const versions = await listWorkflowVersions(agentId);
            // Convert to WorkflowVersion format
            const workflowVersions: WorkflowVersion[] = versions.map(
              (v: WorkflowVersionSummary) => ({
                id: `v${v.version}`,
                version: v.version,
                description: v.description,
                blockCount: v.blockCount,
                createdAt: new Date(v.createdAt),
              })
            );
            set({ workflowVersions });
            console.log(
              '📜 [STORE] Loaded',
              workflowVersions.length,
              'workflow versions from backend'
            );
          } catch (error) {
            console.warn('⚠️ [STORE] Failed to load workflow versions:', error);
            // Keep existing local versions on error
          }
        },

        saveWorkflowVersion: (description?: string) => {
          // This is now just for local UI tracking
          // Backend automatically saves versions when saveCurrentAgent is called
          const { workflow, workflowVersions } = get();
          if (!workflow) return;

          // Calculate next version number (from local tracking)
          const nextVersionNumber =
            workflowVersions.length > 0
              ? Math.max(...workflowVersions.map(v => v.version)) + 1
              : workflow.version;

          const newVersion: WorkflowVersion = {
            id: generateId(),
            version: nextVersionNumber,
            description: description || `Version ${nextVersionNumber}`,
            blockCount: workflow.blocks.length,
            createdAt: new Date(),
          };

          set({
            workflowVersions: [...workflowVersions, newVersion],
          });

          console.log(
            '📸 [STORE] Tracked workflow version locally:',
            nextVersionNumber,
            description
          );
        },

        restoreWorkflowVersion: async (versionNumber: number) => {
          const { selectedAgentId, workflow } = get();
          if (!selectedAgentId) {
            console.warn('⚠️ [STORE] No agent selected for version restore');
            return;
          }

          try {
            console.log('⏪ [STORE] Restoring version', versionNumber, 'from backend...');
            const restoredWorkflow = await restoreWorkflowVersionAPI(
              selectedAgentId,
              versionNumber
            );

            const { nodes, edges } = workflowToNodesAndEdges(restoredWorkflow);

            set(state => ({
              workflow: restoredWorkflow,
              nodes,
              edges,
              // Push current to history for undo
              workflowHistory: workflow
                ? [...state.workflowHistory.slice(-49), workflow]
                : state.workflowHistory,
              workflowFuture: [], // Clear future
              currentAgent: state.currentAgent
                ? { ...state.currentAgent, workflow: restoredWorkflow, updatedAt: new Date() }
                : null,
            }));

            // Reload versions from backend (new version was created)
            get().loadWorkflowVersions(selectedAgentId);

            // Mark as clean — restore saves to backend automatically
            get().markClean();

            console.log(
              '✅ [STORE] Restored workflow version:',
              versionNumber,
              '→ new version:',
              restoredWorkflow.version
            );
          } catch (error) {
            console.error('❌ [STORE] Failed to restore workflow version:', error);
          }
        },

        clearWorkflowVersions: () => {
          set({ workflowVersions: [] });
        },

        // =====================================================================
        // React Flow Sync
        // =====================================================================

        syncNodesAndEdges: () => {
          const { workflow } = get();
          const { nodes, edges } = workflowToNodesAndEdges(workflow);
          set({ nodes, edges });
        },

        onNodesChange: changes => {
          // Handle node changes from React Flow (position + removal)
          const typedChanges = changes as Array<{
            type: string;
            id: string;
            position?: { x: number; y: number };
          }>;

          // Handle removals first (backspace/delete key)
          const removedIds = typedChanges.filter(c => c.type === 'remove').map(c => c.id);

          if (removedIds.length > 0) {
            for (const id of removedIds) {
              get().removeBlock(id);
            }
            return; // removeBlock already rebuilds nodes/edges
          }

          // Handle position changes
          set(state => {
            if (!state.workflow) return state;

            let updatedBlocks = [...state.workflow.blocks];

            for (const change of typedChanges) {
              if (change.type === 'position' && change.position) {
                updatedBlocks = updatedBlocks.map(block =>
                  block.id === change.id ? { ...block, position: change.position! } : block
                );
              }
            }

            const newWorkflow = {
              ...state.workflow,
              blocks: updatedBlocks,
            };

            const { nodes, edges } = workflowToNodesAndEdges(newWorkflow);

            return {
              workflow: newWorkflow,
              nodes,
              edges,
            };
          });
        },

        onEdgesChange: changes => {
          // Handle edge changes from React Flow (deletion via backspace/delete)
          const typedChanges = changes as Array<{ type: string; id: string }>;
          const removedIds = typedChanges.filter(c => c.type === 'remove').map(c => c.id);

          if (removedIds.length > 0) {
            for (const id of removedIds) {
              get().removeConnection(id);
            }
          }
        },

        onConnect: connection => {
          // Map sourceHandle from React Flow to sourceOutput on the Connection
          // For multi-output blocks (e.g., if_condition), sourceHandle is "true" or "false"
          // For for_each, sourceHandle is "loop_body" or "done"
          const newConnection: Connection = {
            id: generateId(),
            sourceBlockId: connection.source,
            sourceOutput: connection.sourceHandle || 'output', // Use handle ID or default "output"
            targetBlockId: connection.target,
            targetInput: `from_${connection.source}`, // Named input
          };

          get().addConnection(newConnection);
        },

        autoLayoutWorkflow: () => {
          const { nodes, edges, currentAgent, workflow } = get();
          if (!nodes.length || !workflow) return;

          // Apply smart layout algorithm (auto-selects best layout for workflow size)
          // - Hierarchical DAG for small/medium workflows (up to ~75 blocks)
          // - Compact grid for very large workflows (75+ blocks)
          // - Barycenter heuristic to minimize edge crossings
          // - Adaptive spacing based on node count and depth
          const { nodes: layoutedNodes } = getSmartLayout(nodes, edges);

          // Update nodes with new positions
          set({ nodes: layoutedNodes });

          // Sync positions back to workflow blocks
          if (currentAgent) {
            const updatedBlocks = workflow.blocks.map(block => {
              const node = layoutedNodes.find(n => n.id === block.id);
              return node ? { ...block, position: node.position } : block;
            });

            const updatedWorkflow = { ...workflow, blocks: updatedBlocks };

            set({
              workflow: updatedWorkflow,
              currentAgent: {
                ...currentAgent,
                workflow: updatedWorkflow,
              },
            });
          }

          console.log('✅ [STORE] Smart auto-layout applied to', nodes.length, 'blocks');
        },

        // =====================================================================
        // Block Selection & UI
        // =====================================================================

        selectBlock: blockId => {
          const { workflow } = get();

          if (!blockId || !workflow) {
            set({
              selectedBlockId: blockId,
              highlightedUpstreamIds: [],
              highlightedDownstreamIds: [],
              highlightedEdgeIds: [],
            });
            return;
          }

          // Find the selected block
          const selectedBlock = workflow.blocks.find(b => b.id === blockId);
          if (!selectedBlock) {
            set({
              selectedBlockId: blockId,
              highlightedUpstreamIds: [],
              highlightedDownstreamIds: [],
              highlightedEdgeIds: [],
            });
            return;
          }

          // Build a map of normalizedId -> blockId for template matching
          const normalizedIdToBlockId = new Map<string, string>();
          for (const block of workflow.blocks) {
            if (block.normalizedId) {
              normalizedIdToBlockId.set(block.normalizedId, block.id);
            }
            // Also map by block name (lowercase, with spaces as dashes)
            const nameKey = block.name.toLowerCase().replace(/\s+/g, '-');
            normalizedIdToBlockId.set(nameKey, block.id);
          }

          // Parse template references from block config (userPrompt, systemPrompt, etc.)
          // Templates look like: {{block-name.response}}, {{block-name.data.field}}
          const templateRegex = /\{\{([a-zA-Z0-9_-]+)\./g;
          const referencedBlockIds = new Set<string>();

          // Stringify the entire config to find all template references
          const configStr = JSON.stringify(selectedBlock.config || {});
          let match;
          while ((match = templateRegex.exec(configStr)) !== null) {
            const refName = match[1];
            // Skip special variables like 'start', 'input'
            if (refName === 'start' || refName === 'input') {
              // Find the start block
              const startBlock = workflow.blocks.find(b => {
                if (b.type !== 'variable') return false;
                const cfg = b.config as { operation?: string; variableName?: string };
                return cfg?.operation === 'read' && cfg?.variableName === 'input';
              });
              if (startBlock) {
                referencedBlockIds.add(startBlock.id);
              }
              continue;
            }
            // Look up by normalizedId
            const foundBlockId = normalizedIdToBlockId.get(refName);
            if (foundBlockId && foundBlockId !== blockId) {
              referencedBlockIds.add(foundBlockId);
            }
          }

          // Also include direct edge connections (for blocks that don't use templates)
          for (const conn of workflow.connections) {
            if (conn.targetBlockId === blockId) {
              referencedBlockIds.add(conn.sourceBlockId);
            }
          }

          // Find downstream: blocks that reference THIS block in their templates
          // OR have direct edge connections from this block
          const downstreamBlockIds = new Set<string>();
          const selectedNormalizedId = selectedBlock.normalizedId;
          const selectedNameKey = selectedBlock.name.toLowerCase().replace(/\s+/g, '-');

          for (const block of workflow.blocks) {
            if (block.id === blockId) continue;

            const blockConfigStr = JSON.stringify(block.config || {});
            // Check if this block references the selected block
            if (
              (selectedNormalizedId && blockConfigStr.includes(`{{${selectedNormalizedId}.`)) ||
              blockConfigStr.includes(`{{${selectedNameKey}.`)
            ) {
              downstreamBlockIds.add(block.id);
            }
          }

          // Also add direct edge connections going out
          for (const conn of workflow.connections) {
            if (conn.sourceBlockId === blockId) {
              downstreamBlockIds.add(conn.targetBlockId);
            }
          }

          // Find edges connected to the selected block (both incoming and outgoing)
          const highlightedEdges = workflow.connections
            .filter(conn => conn.sourceBlockId === blockId || conn.targetBlockId === blockId)
            .map(conn => conn.id);

          set({
            selectedBlockId: blockId,
            highlightedUpstreamIds: Array.from(referencedBlockIds),
            highlightedDownstreamIds: Array.from(downstreamBlockIds),
            highlightedEdgeIds: highlightedEdges,
          });
        },

        toggleSidebar: () => {
          set(state => ({ isSidebarOpen: !state.isSidebarOpen }));
        },

        toggleSidebarCollapsed: () => {
          set(state => ({ isSidebarCollapsed: !state.isSidebarCollapsed }));
        },

        toggleDebugMode: () => {
          set(state => ({ debugMode: !state.debugMode }));
        },

        setActiveView: view => {
          set({ activeView: view });
        },

        trackAgentAccess: agentId => {
          set(state => {
            // Remove agentId if it exists, then add to front
            const filtered = state.recentAgentIds.filter(id => id !== agentId);
            const updated = [agentId, ...filtered].slice(0, 10); // Keep max 10
            return { recentAgentIds: updated };
          });
        },

        // =====================================================================
        // Builder Chat
        // =====================================================================

        addBuilderMessage: message => {
          const newMessage: BuilderMessage = {
            ...message,
            id: generateId(),
            timestamp: new Date(),
          };

          set(state => ({
            builderMessages: [...state.builderMessages, newMessage],
          }));
        },

        clearBuilderMessages: () => {
          set({ builderMessages: [] });
        },

        setIsGenerating: isGenerating => {
          set({ isGenerating });
        },

        setShowOnboardingGuide: show => {
          set({ showOnboardingGuide: show });
        },

        setShowOnboardingGuidance: show => {
          set({ showOnboardingGuidance: show });
        },

        setShowPostExecutionGuidance: show => {
          set({ showPostExecutionGuidance: show });
        },

        setHighlightChatInput: highlight => {
          set({ highlightChatInput: highlight });
        },

        setPendingChatMessage: message => {
          set({ pendingChatMessage: message });
        },

        // =====================================================================
        // Conversation Persistence
        // =====================================================================

        loadConversation: async (agentId, modelId) => {
          set({ isLoadingConversation: true });
          try {
            const conversation = await getOrCreateBuilderConversation(agentId, modelId);
            if (conversation) {
              // Convert message timestamps to Date objects (handle null/undefined messages)
              const messages = (conversation.messages || []).map(msg => ({
                ...msg,
                timestamp: new Date(msg.timestamp),
              }));
              set({
                currentConversationId: conversation.id,
                builderMessages: messages,
                isLoadingConversation: false,
              });
              console.log(
                '✅ [STORE] Loaded conversation:',
                conversation.id,
                'with',
                messages.length,
                'messages'
              );
            } else {
              set({
                currentConversationId: null,
                builderMessages: [],
                isLoadingConversation: false,
              });
            }
          } catch (error) {
            console.warn(
              '⚠️ [STORE] Failed to load conversation (MongoDB may be disabled):',
              error
            );
            set({
              currentConversationId: null,
              isLoadingConversation: false,
            });
          }
        },

        persistMessage: async message => {
          let { currentConversationId } = get();
          const { selectedAgentId, workflow } = get();

          if (!selectedAgentId) {
            console.log('⏭️ [STORE] Skipping message persistence (no agent selected)');
            return;
          }

          // If no conversation exists, create one first
          if (!currentConversationId) {
            console.log('📝 [STORE] No conversation ID, creating one...');
            try {
              const conversation = await getOrCreateBuilderConversation(selectedAgentId);
              if (conversation) {
                currentConversationId = conversation.id;
                set({ currentConversationId: conversation.id });
                console.log('✅ [STORE] Created conversation:', conversation.id);
              } else {
                console.warn('⚠️ [STORE] Failed to create conversation');
                return;
              }
            } catch (error) {
              console.warn('⚠️ [STORE] Failed to create conversation:', error);
              return;
            }
          }

          try {
            // Use workflow update from message if present, otherwise create from current workflow
            let workflowSnapshot:
              | { version: number; action?: 'create' | 'modify'; explanation?: string }
              | undefined;

            if ('workflowUpdate' in message && message.workflowUpdate) {
              // Assistant message with workflow update - use it
              workflowSnapshot = {
                version: message.workflowUpdate.workflow.version,
                action: message.workflowUpdate.action,
                explanation: message.workflowUpdate.explanation,
              };
            } else if (workflow && message.role === 'user') {
              // User message - capture current workflow state
              workflowSnapshot = {
                version: workflow.version,
                action: workflow.blocks.length > 1 ? 'modify' : 'create',
              };
            }

            await addBuilderMessageAPI(selectedAgentId, currentConversationId, {
              role: message.role,
              content: message.content,
              workflow_snapshot: workflowSnapshot,
            });
            console.log('✅ [STORE] Persisted message to conversation:', currentConversationId);
          } catch (error) {
            console.warn('⚠️ [STORE] Failed to persist message:', error);
          }
        },

        fetchConversationList: async agentId => {
          try {
            const conversations = await listBuilderConversations(agentId);
            set({ conversationList: conversations });
            console.log(
              '✅ [STORE] Fetched',
              conversations.length,
              'conversations for agent:',
              agentId
            );
          } catch (error) {
            console.warn('⚠️ [STORE] Failed to fetch conversation list:', error);
            set({ conversationList: [] });
          }
        },

        selectConversation: async conversationId => {
          const { selectedAgentId } = get();
          if (!selectedAgentId) return;

          set({ isLoadingConversation: true });
          try {
            // Import dynamically to avoid circular dependency
            const { getBuilderConversation } = await import('@/services/conversationService');
            const conversation = await getBuilderConversation(selectedAgentId, conversationId);
            if (conversation) {
              const messages = (conversation.messages || []).map(msg => ({
                ...msg,
                timestamp: new Date(msg.timestamp),
              }));
              set({
                currentConversationId: conversation.id,
                builderMessages: messages,
                isLoadingConversation: false,
              });
            }
          } catch (error) {
            console.warn('⚠️ [STORE] Failed to load conversation:', error);
            set({ isLoadingConversation: false });
          }
        },

        clearConversation: () => {
          set({
            currentConversationId: null,
            builderMessages: [],
          });
        },

        // =====================================================================
        // Execution
        // =====================================================================

        startExecution: executionId => {
          const { workflow } = get();

          // Initialize block states
          const blockStates: Record<string, BlockExecutionState> = {};
          if (workflow) {
            for (const block of workflow.blocks) {
              blockStates[block.id] = {
                blockId: block.id,
                status: 'pending',
                inputs: {},
                outputs: {},
              };
            }
          }

          set({
            executionId,
            executionStatus: 'running',
            blockStates,
          });
        },

        updateBlockExecution: (blockId, state) => {
          set(prev => ({
            blockStates: {
              ...prev.blockStates,
              [blockId]: {
                ...prev.blockStates[blockId],
                ...state,
              },
            },
          }));
        },

        completeExecution: (status, apiResponse) => {
          set({
            executionStatus: status,
            lastExecutionResult: apiResponse || null,
          });

          // Add execution result as a chat message
          const blocksExecuted = apiResponse?.metadata?.blocks_executed || 0;
          const blocksFailed = apiResponse?.metadata?.blocks_failed || 0;

          let content = '';
          if (status === 'completed') {
            content = apiResponse?.result
              ? `Execution completed successfully!\n\n${apiResponse.result}`
              : 'Execution completed successfully!';
          } else if (status === 'failed') {
            content = `Execution failed. ${apiResponse?.error || 'Please check the workflow and try again.'}`;
          } else if (status === 'partial_failure') {
            content = `Execution completed with some failures. ${blocksExecuted} blocks succeeded, ${blocksFailed} blocks failed.`;
          }

          if (content) {
            const executionMessage = {
              id: `exec-${Date.now()}`,
              role: 'system' as const,
              content,
              timestamp: new Date(),
              executionResult: {
                status: status as 'completed' | 'failed' | 'partial_failure',
                result: apiResponse?.result,
                error: apiResponse?.error,
                blocksExecuted,
                blocksFailed,
              },
            };

            set(state => ({
              builderMessages: [...state.builderMessages, executionMessage],
            }));
          }
        },

        cacheBlockOutput: (blockId, output) => {
          set(state => ({
            blockOutputCache: {
              ...state.blockOutputCache,
              [blockId]: output,
            },
          }));
        },

        clearExecution: () => {
          set({
            executionId: null,
            executionStatus: null,
            blockStates: {},
            forEachStates: {},
            lastExecutionResult: null,
          });
        },

        updateForEachIteration: (blockId, iteration, totalItems, currentItem) => {
          set(state => {
            const existing = state.forEachStates[blockId];
            const iterationIndex = iteration - 1; // Convert 1-based to 0-based

            // Build new iteration entry
            const newIteration: IterationState = {
              index: iterationIndex,
              status: 'running',
              item: currentItem,
            };

            let iterations: IterationState[];
            if (existing) {
              iterations = [...existing.iterations];
              // Mark previous iteration as completed if it was running
              if (iterationIndex > 0 && iterations[iterationIndex - 1]?.status === 'running') {
                iterations[iterationIndex - 1] = {
                  ...iterations[iterationIndex - 1],
                  status: 'completed',
                };
              }
              iterations[iterationIndex] = newIteration;
            } else {
              iterations = [];
              iterations[iterationIndex] = newIteration;
            }

            return {
              forEachStates: {
                ...state.forEachStates,
                [blockId]: {
                  blockId,
                  currentIteration: iteration,
                  totalItems,
                  iterations,
                },
              },
            };
          });
        },

        setForEachResults: (blockId, iterationResults) => {
          set(state => {
            const existing = state.forEachStates[blockId];
            const iterations: IterationState[] = iterationResults.map((result, idx) => ({
              index: idx,
              status: (result._error ? 'failed' : 'completed') as 'completed' | 'failed',
              item: result._currentItem ?? existing?.iterations[idx]?.item,
              output: result,
              error: result._error as string | undefined,
            }));

            return {
              forEachStates: {
                ...state.forEachStates,
                [blockId]: {
                  blockId,
                  currentIteration: iterationResults.length,
                  totalItems: iterationResults.length,
                  iterations,
                },
              },
            };
          });
        },

        clearForEachStates: () => {
          set({ forEachStates: {} });
        },

        // =====================================================================
        // Execution Viewer
        // =====================================================================

        setExecutionViewerMode: mode => {
          if (mode === 'editor') {
            set({
              executionViewerMode: 'editor',
              selectedExecutionId: null,
              selectedExecutionData: null,
              inspectedBlockId: null,
            });
          } else {
            set({ executionViewerMode: mode });
          }
        },

        selectExecution: executionId => {
          set({
            selectedExecutionId: executionId,
            selectedExecutionData: executionId ? get().selectedExecutionData : null,
            inspectedBlockId: null,
          });
        },

        setSelectedExecutionData: data => {
          set({ selectedExecutionData: data });
        },

        setInspectedBlockId: blockId => {
          set({ inspectedBlockId: blockId });
        },

        // =====================================================================
        // Persistence
        // =====================================================================

        saveCurrentAgent: async (createVersion = true, versionDescription = '') => {
          const { currentAgent, workflow } = get();

          if (!currentAgent || !workflow) return;

          // Auto-generate version description if creating version without one
          if (createVersion && !versionDescription) {
            versionDescription = 'Manual save';
          }

          try {
            // Agent should already exist on backend (created via createAgent)
            console.log(
              '💾 [AGENT] Saving workflow to backend for agent:',
              currentAgent.id,
              createVersion ? `(creating version: ${versionDescription})` : '(no version)'
            );
            await api.put(`/api/agents/${currentAgent.id}/workflow`, {
              blocks: workflow.blocks,
              connections: workflow.connections,
              variables: workflow.variables,
              workflowModelId: workflow.workflowModelId,
              createVersion,
              versionDescription,
            });
            console.log('✅ [AGENT] Workflow saved successfully');

            // Update local state
            const updatedAgent: Agent = {
              ...currentAgent,
              workflow,
              updatedAt: new Date(),
              syncStatus: 'synced',
            };

            set({
              currentAgent: updatedAgent,
            });

            // Mark as clean after successful save
            get().markClean();

            // Always reload versions (every save creates a version now)
            get().loadWorkflowVersions(currentAgent.id);
          } catch (error) {
            console.error('❌ [AGENT] Failed to save agent:', error);
            // Still update local state even if backend fails
            const updatedAgent: Agent = {
              ...currentAgent,
              workflow,
              updatedAt: new Date(),
            };

            set({
              currentAgent: updatedAgent,
            });
          }
        },

        markClean: () => {
          const { workflow } = get();
          set({
            isDirty: false,
            lastSavedAt: new Date(),
          });
          // Store snapshot for future comparison (unused for now but available)
          void workflowSnapshot(workflow);
        },

        // Agent switch guard
        requestAgentSwitch: (agentId, callback) => {
          const { isDirty, selectedAgentId } = get();
          if (agentId === selectedAgentId) return true;
          if (!isDirty) return true;
          // Show confirmation dialog
          set({
            pendingAgentSwitch: { agentId, callback },
            showUnsavedChangesDialog: true,
          });
          return false;
        },

        confirmAgentSwitch: () => {
          const { pendingAgentSwitch } = get();
          const callback = pendingAgentSwitch?.callback;
          set({
            showUnsavedChangesDialog: false,
            isDirty: false,
            lastSavedAt: null,
            pendingAgentSwitch: null,
          });
          // Execute the pending switch callback
          if (callback) {
            callback();
          }
        },

        saveAndSwitch: async () => {
          const { pendingAgentSwitch } = get();
          const callback = pendingAgentSwitch?.callback;
          // Save first, then switch
          await get().saveCurrentAgent(true, 'Auto-save before switching');
          set({
            showUnsavedChangesDialog: false,
            pendingAgentSwitch: null,
          });
          if (callback) {
            callback();
          }
        },

        cancelAgentSwitch: () => {
          set({
            pendingAgentSwitch: null,
            showUnsavedChangesDialog: false,
          });
        },

        // =====================================================================
        // Backend-First Sync
        // =====================================================================

        fetchRecentAgents: async () => {
          set({ isLoadingAgents: true });
          try {
            const recentAgents = await getRecentAgents();
            set({
              backendAgents: recentAgents,
              isLoadingAgents: false,
            });
            console.log('✅ [STORE] Fetched', recentAgents.length, 'recent agents from backend');
          } catch (error) {
            console.warn('⚠️ [STORE] Failed to fetch recent agents:', error);
            set({ isLoadingAgents: false });
          }
        },

        fetchAgentsPage: async (offset = 0) => {
          set(state => ({
            pagination: { ...state.pagination, isLoading: true },
          }));

          try {
            const response = await getAgentsPaginated(20, offset);
            set(state => ({
              backendAgents:
                offset === 0 ? response.agents : [...state.backendAgents, ...response.agents],
              pagination: {
                total: response.total,
                hasMore: response.has_more,
                isLoading: false,
              },
            }));
            console.log(
              '✅ [STORE] Fetched agents page:',
              response.agents.length,
              'total:',
              response.total
            );
          } catch (error) {
            console.warn('⚠️ [STORE] Failed to fetch agents page:', error);
            set(state => ({
              pagination: { ...state.pagination, isLoading: false },
            }));
          }
        },

        syncAgentToBackend: async agentId => {
          // With API-based agents, agents are already on backend when created
          // This function is kept for backward compatibility but simplified
          const { workflow, currentAgent } = get();

          if (!currentAgent || currentAgent.id !== agentId) {
            console.warn('⚠️ [STORE] Agent not found for sync:', agentId);
            return null;
          }

          // Mark as syncing
          get().updateAgentSyncStatus(agentId, 'syncing');

          try {
            const syncResult = await syncAgentAPI(agentId, {
              name: currentAgent.name,
              description: currentAgent.description,
              workflow: {
                blocks: (workflow || currentAgent.workflow).blocks,
                connections: (workflow || currentAgent.workflow).connections,
                variables: (workflow || currentAgent.workflow).variables,
              },
            });

            if (syncResult) {
              // Mark as synced
              get().updateAgentSyncStatus(agentId, 'synced');

              // Update currentConversationId
              if (syncResult.conversation_id) {
                set({ currentConversationId: syncResult.conversation_id });
              }

              console.log(
                '✅ [STORE] Agent synced to backend:',
                agentId,
                'conversation:',
                syncResult.conversation_id
              );
              return { conversationId: syncResult.conversation_id };
            }

            // No result, mark as error
            get().updateAgentSyncStatus(agentId, 'error', 'Sync returned no result');
            return null;
          } catch (error) {
            const errorMessage = error instanceof Error ? error.message : 'Unknown error';
            get().updateAgentSyncStatus(agentId, 'error', errorMessage);
            console.error('❌ [STORE] Failed to sync agent:', error);
            return null;
          }
        },

        updateAgentSyncStatus: (agentId, status, error) => {
          // Only update currentAgent (no local agents array)
          set(state => ({
            currentAgent:
              state.currentAgent?.id === agentId
                ? { ...state.currentAgent, syncStatus: status, lastSyncError: error }
                : state.currentAgent,
          }));
        },

        loadAgentFromBackend: async agentId => {
          set({ isLoadingAgents: true });
          try {
            // Fetch full agent with workflow from backend
            const agentResponse = await getAgentAPI(agentId);
            if (agentResponse) {
              // Create workflow from response or empty workflow
              const workflowData = agentResponse.workflow;
              const workflow: Workflow = {
                id: workflowData?.id || generateId(),
                blocks: workflowData?.blocks || [],
                connections: workflowData?.connections || [],
                variables: workflowData?.variables || [],
                version: workflowData?.version || 1,
                workflowModelId: workflowData?.workflowModelId,
              };

              // Migration: if no workflow-level model, check Start block config
              if (!workflow.workflowModelId) {
                const startBlock = workflow.blocks.find(b => {
                  if (b.type !== 'variable') return false;
                  const cfg = b.config as unknown as Record<string, unknown>;
                  return cfg?.operation === 'read' && cfg?.variableName === 'input';
                });
                if (startBlock) {
                  const cfg = startBlock.config as unknown as Record<string, unknown>;
                  const startModel = cfg?.workflowModelId as string | undefined;
                  if (startModel) {
                    workflow.workflowModelId = startModel;
                  }
                }
              }

              // Auto-connect blocks based on template references
              const connectedWorkflow = autoConnectByTemplateReferences(workflow);

              // Create a full Agent from backend data
              const agent: Agent = {
                id: agentResponse.id,
                userId: agentResponse.user_id,
                name: agentResponse.name,
                description: agentResponse.description || '',
                workflow: connectedWorkflow,
                status: (agentResponse.status as 'draft' | 'deployed' | 'paused') || 'draft',
                syncStatus: 'synced',
                createdAt: new Date(agentResponse.created_at),
                updatedAt: new Date(agentResponse.updated_at),
              };

              const { nodes, edges } = workflowToNodesAndEdges(agent.workflow);

              set({
                currentAgent: agent,
                selectedAgentId: agentId,
                workflow: agent.workflow,
                nodes,
                edges,
                isLoadingAgents: false,
                builderMessages: [],
                // Clear execution state when loading different agent
                executionId: null,
                executionStatus: null,
                blockStates: {},
                forEachStates: {},
                blockOutputCache: {},
                lastExecutionResult: null,
                // Clear version history
                workflowVersions: [],
                workflowHistory: [],
                workflowFuture: [],
              });

              // Load workflow versions from backend
              get().loadWorkflowVersions(agentId);

              // Freshly loaded from backend — mark as clean
              get().markClean();

              console.log('✅ [STORE] Loaded agent from backend:', agentId, 'name:', agent.name);
            } else {
              console.warn('⚠️ [STORE] Agent not found on backend:', agentId);
              set({ isLoadingAgents: false, selectedAgentId: null });
            }
          } catch (error) {
            console.warn('⚠️ [STORE] Failed to load agent from backend:', error);
            set({ isLoadingAgents: false });
          }
        },
      }),
      {
        name: 'agent-builder-storage',
        storage: createAgentStorage(),
        partialize: state => ({
          // Only persist UI state - agents are fully API-based
          selectedAgentId: state.selectedAgentId,
          isSidebarOpen: state.isSidebarOpen,
          isSidebarCollapsed: state.isSidebarCollapsed,
          activeView: state.activeView,
          recentAgentIds: state.recentAgentIds,
          debugMode: state.debugMode,
        }),
        onRehydrateStorage: () => _state => {
          // No local agent rehydration needed - agents are loaded from backend
          // The Agents.tsx useEffect will handle loading the agent from backend
          // if selectedAgentId is set but currentAgent is null
        },
      }
    ),
    { name: 'AgentBuilderStore' }
  )
);
