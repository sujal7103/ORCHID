import { useCallback, useRef, useState, useEffect, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  ReactFlow,
  Background,
  Controls,
  // MiniMap, // Temporarily disabled due to z-index issues
  BackgroundVariant,
  Panel,
  applyNodeChanges,
} from '@xyflow/react';
import type {
  Connection,
  ReactFlowInstance,
  NodeChange,
  EdgeChange,
  Node,
  Edge,
} from '@xyflow/react';
import {
  Play,
  Save,
  Undo,
  Redo,
  Square,
  FileOutput,
  LayoutGrid,
  History,
  ChevronDown,
  KeyRound,
  AlertTriangle,
  ExternalLink,
  Bug,
  Cpu,
  Settings,
  PanelLeftOpen,
  Pencil,
} from 'lucide-react';
import { Link } from 'react-router-dom';
import { cn } from '@/lib/utils';
import { useAgentBuilderStore } from '@/store/useAgentBuilderStore';
import { useModelStore } from '@/store/useModelStore';
import { filterAgentModels } from '@/services/modelService';
import { useCredentialsStore } from '@/store/useCredentialsStore';
import { toast } from '@/store/useToastStore';
import { BlockNode } from './BlockNode';
import { DeletableEdge } from './DeletableEdge';
import { BlockPalette } from './BlockPalette';
import { ExecutionListPanel } from './ExecutionListPanel';
import { ExecutionOutputPanel } from './ExecutionOutputPanel';
import { OnboardingGuidance } from './OnboardingGuidance';
import { SetupRequiredPanel } from './SetupRequiredPanel';
import { DeployPanel } from './DeployPanel';
import { workflowExecutionService } from '@/services/workflowExecutionService';
import { validateWorkflow, type WorkflowValidationResult } from '@/utils/blockValidation';

// Tools that require credentials - maps to integration types (legacy, kept for backward compat)
const TOOLS_REQUIRING_CREDENTIALS: Record<string, string> = {
  // Messaging tools
  send_discord_message: 'discord',
  send_slack_message: 'slack',
  send_telegram_message: 'telegram',
  send_google_chat_message: 'google_chat',
  send_webhook: 'webhook',
  // Notion tools
  notion_search: 'notion',
  notion_query_database: 'notion',
  notion_create_page: 'notion',
  notion_update_page: 'notion',
  // GitHub tools
  github_create_issue: 'github',
  github_list_issues: 'github',
  github_create_pr: 'github',
};

interface WorkflowCanvasProps {
  className?: string;
}

// Custom node types - MUST be outside component to prevent re-renders
const nodeTypes = {
  blockNode: BlockNode,
};

// Custom edge types - deletable edge with hover delete button
const edgeTypes = {
  default: DeletableEdge,
};

export function WorkflowCanvas({ className }: WorkflowCanvasProps) {
  const navigate = useNavigate();
  const {
    currentAgent,
    workflow,
    nodes: storeNodes,
    edges: storeEdges,
    executionStatus,
    blockStates,
    workflowHistory,
    workflowFuture,
    workflowVersions,
    highlightedEdgeIds,
    debugMode,
    onNodesChange: storeOnNodesChange,
    onEdgesChange: storeOnEdgesChange,
    onConnect: storeOnConnect,
    saveCurrentAgent,
    clearExecution,
    autoLayoutWorkflow,
    undo,
    redo,
    restoreWorkflowVersion,
    showOnboardingGuidance,
    selectBlock,
    toggleDebugMode,
    setWorkflowModelId,
    updateAgentStatus,
    executionViewerMode,
    selectedExecutionData,
    setExecutionViewerMode,
    setInspectedBlockId,
    isDirty,
    lastSavedAt,
  } = useAgentBuilderStore();

  // State for execution errors
  const [, setExecutionError] = useState<string | null>(null);

  // State for output panel visibility
  const [showOutputPanel, setShowOutputPanel] = useState(false);

  // State for version dropdown
  const [showVersionDropdown, setShowVersionDropdown] = useState(false);

  // State for saving indicator
  const [isSaving, setIsSaving] = useState(false);

  // State for credentials dropdown
  const [showCredentialsDropdown, setShowCredentialsDropdown] = useState(false);

  // State for setup required panel
  const [showSetupPanel, setShowSetupPanel] = useState(false);

  // State for deploy panel
  const [showDeployPanel, setShowDeployPanel] = useState(false);

  // State for inline test input dialog
  const [showTestInputDialog, setShowTestInputDialog] = useState(false);
  const [testInput, setTestInput] = useState('');

  // State for block palette (open by default for easy access)
  const [showBlockPalette, setShowBlockPalette] = useState(true);

  // Model store for workflow-level model selector
  const { models, fetchModels, isLoading: modelsLoading } = useModelStore();
  const agentModels = useMemo(() => filterAgentModels(models), [models]);

  useEffect(() => {
    if (models.length === 0) {
      fetchModels();
    }
  }, [models.length, fetchModels]);

  // Credentials store
  const { credentialReferences, fetchCredentialReferences } = useCredentialsStore();

  // Fetch credentials on mount
  useEffect(() => {
    fetchCredentialReferences();
  }, [fetchCredentialReferences]);

  // Load saved test input from localStorage when agent changes
  useEffect(() => {
    if (currentAgent?.id) {
      const saved = localStorage.getItem(`clara_test_input_${currentAgent.id}`);
      if (saved) setTestInput(saved);
      else setTestInput('');
    }
  }, [currentAgent?.id]);

  // Get all tools used in the workflow that require credentials
  const workflowCredentialInfo = useMemo(() => {
    if (!workflow || workflow.blocks.length === 0) {
      return { toolsNeedingCredentials: [], unconfiguredTools: [], configuredTools: [] };
    }

    // Collect all tools from llm_inference blocks
    const allToolsInWorkflow: string[] = [];
    const blockCredentials: Record<string, string[]> = {};

    workflow.blocks.forEach(block => {
      // Check block.type instead of config.type (AI-generated blocks might not set config.type)
      if (block.type === 'llm_inference') {
        // Check both 'tools' and 'enabledTools' keys (config uses both depending on source)
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const config = block.config as any;
        const enabledTools: string[] = config.tools || config.enabledTools || [];
        const credentials: string[] = config.credentials || [];

        enabledTools.forEach((tool: string) => {
          if (!allToolsInWorkflow.includes(tool)) {
            allToolsInWorkflow.push(tool);
          }
        });

        // Track credentials per block
        blockCredentials[block.id] = credentials;
      }
    });

    // Filter to only tools that require credentials
    const toolsNeedingCredentials = allToolsInWorkflow.filter(
      tool => tool in TOOLS_REQUIRING_CREDENTIALS
    );

    // Check which integration types are needed
    const neededIntegrationTypes = new Set<string>();
    toolsNeedingCredentials.forEach(tool => {
      const integrationType = TOOLS_REQUIRING_CREDENTIALS[tool];
      if (integrationType) {
        neededIntegrationTypes.add(integrationType);
      }
    });

    // Check which integration types have credentials available
    const availableIntegrationTypes = new Set<string>();
    credentialReferences.forEach(cred => {
      availableIntegrationTypes.add(cred.integrationType);
    });

    // Determine configured vs unconfigured
    const unconfiguredTools: Array<{ tool: string; integrationType: string; displayName: string }> =
      [];
    const configuredTools: Array<{ tool: string; integrationType: string; displayName: string }> =
      [];

    toolsNeedingCredentials.forEach(tool => {
      const integrationType = TOOLS_REQUIRING_CREDENTIALS[tool];
      const displayName = tool
        .replace(/_/g, ' ')
        .replace(/^send /, '')
        .replace(/\b\w/g, l => l.toUpperCase());

      if (availableIntegrationTypes.has(integrationType)) {
        configuredTools.push({ tool, integrationType, displayName });
      } else {
        unconfiguredTools.push({ tool, integrationType, displayName });
      }
    });

    return { toolsNeedingCredentials, unconfiguredTools, configuredTools };
  }, [workflow, credentialReferences]);

  // Keyboard shortcuts for undo/redo (save is registered after handleSave below)
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Don't trigger if user is typing in an input/textarea
      if (
        e.target instanceof HTMLInputElement ||
        e.target instanceof HTMLTextAreaElement ||
        (e.target as HTMLElement).isContentEditable
      ) {
        return;
      }

      if ((e.ctrlKey || e.metaKey) && e.key === 'z' && !e.shiftKey) {
        e.preventDefault();
        undo();
      } else if (
        ((e.ctrlKey || e.metaKey) && e.key === 'y') ||
        ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'z')
      ) {
        e.preventDefault();
        redo();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [undo, redo]);

  // Auto-show output panel when execution completes
  useEffect(() => {
    if (executionStatus === 'failed' || executionStatus === 'partial_failure') {
      setShowOutputPanel(true);
    }
  }, [executionStatus]);

  // Check if workflow is valid and ready to run (same validation as "Save & Run" button)
  // Returns { isValid, missingItems } to help show specific error messages
  const workflowValidation = useMemo(() => {
    const result = { isValid: false, missingItems: [] as string[] };

    if (!workflow || workflow.blocks.length === 0) {
      result.missingItems.push('workflow blocks');
      return result;
    }

    // Trigger-based workflows are always valid (they don't need a model)
    const hasTriggerBlock = workflow.blocks.some(
      block => block.type === 'webhook_trigger' || block.type === 'schedule_trigger'
    );
    if (hasTriggerBlock) {
      result.isValid = true;
      return result;
    }

    // Check if workflow has LLM blocks that need a model
    const hasLLMBlocks = workflow.blocks.some(block => block.type === 'llm_inference');
    if (hasLLMBlocks) {
      // Check workflow-level model first, then Start block fallback
      const hasWorkflowModel = !!workflow.workflowModelId;
      const startBlock = workflow.blocks.find(
        block =>
          block.type === 'variable' &&
          block.config.type === 'variable' &&
          (block.config as unknown as Record<string, unknown>)?.operation === 'read' &&
          (block.config as unknown as Record<string, unknown>)?.variableName === 'input'
      );
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const hasStartBlockModel = !!(startBlock?.config as any)?.workflowModelId;

      if (!hasWorkflowModel && !hasStartBlockModel) {
        result.missingItems.push('model selection');
        return result;
      }
    }

    // Any non-empty workflow with blocks is valid (test input handled at run time)
    result.isValid = true;
    return result;
  }, [workflow]);

  const isWorkflowValid = workflowValidation.isValid;

  // Comprehensive workflow validation including credentials
  const comprehensiveValidation: WorkflowValidationResult = useMemo(() => {
    if (!workflow || workflow.blocks.length === 0) {
      return {
        isValid: false,
        issues: [],
        missingCredentials: new Map(),
        blocksNeedingAttention: [],
      };
    }

    // Get integration types that have credentials configured
    const configuredIntegrationTypes = credentialReferences.map(cred => cred.integrationType);

    return validateWorkflow(workflow, configuredIntegrationTypes);
  }, [workflow, credentialReferences]);

  // Handle running the workflow
  const handleRunWorkflow = useCallback(async () => {
    if (!currentAgent?.id || !workflow || workflow.blocks.length === 0) return;

    setExecutionError(null);

    try {
      // Auto-save before running to ensure agent exists on backend
      console.log('💾 [WORKFLOW] Auto-saving agent before execution...');
      await saveCurrentAgent();

      // Get the updated agent ID (may have changed after save)
      const updatedAgent = useAgentBuilderStore.getState().currentAgent;
      if (!updatedAgent?.id) {
        throw new Error('Failed to save agent');
      }

      // Build workflow input depending on entry point type
      const workflowInput: Record<string, unknown> = {};

      // Check for trigger blocks (n8n-style deterministic workflows)
      const triggerBlock = workflow.blocks.find(
        block => block.type === 'webhook_trigger' || block.type === 'schedule_trigger'
      );

      if (triggerBlock) {
        // Trigger-based workflow: use test data from block config if available
        workflowInput.triggerType =
          triggerBlock.type === 'webhook_trigger' ? 'webhook' : 'schedule';
        workflowInput.headers = {};
        workflowInput.method = (triggerBlock.config as Record<string, unknown>).method || 'POST';
        workflowInput.path = '/test';
        workflowInput.query = {};

        // Parse test data from webhook trigger config (use default if not explicitly set)
        const testDataStr =
          ((triggerBlock.config as Record<string, unknown>).testData as string) ||
          '{"message": "Hello from webhook"}';
        try {
          workflowInput.body = JSON.parse(testDataStr);
        } catch {
          workflowInput.body = { message: 'Hello from webhook' };
        }
        console.log(
          '🔗 [WORKFLOW] Running trigger-based workflow with test data:',
          workflowInput.body
        );
      } else {
        // Traditional Start block workflow: extract input from variable/read block
        const startBlock = workflow.blocks.find(
          block =>
            block.type === 'variable' &&
            block.config.type === 'variable' &&
            block.config.operation === 'read' &&
            block.config.variableName === 'input'
        );

        if (startBlock && startBlock.config.type === 'variable') {
          const config = startBlock.config;
          const inputType = config.inputType || 'text';

          if (inputType === 'file' && config.fileValue) {
            // Pass file reference as input
            workflowInput.input = {
              file_id: config.fileValue.fileId,
              filename: config.fileValue.filename,
              mime_type: config.fileValue.mimeType,
              size: config.fileValue.size,
              type: config.fileValue.type,
            };
            console.log('📁 [WORKFLOW] Using file input:', config.fileValue.filename);
          } else if (inputType === 'json' && config.jsonValue) {
            // Pass JSON object as input
            workflowInput.input = config.jsonValue;
            console.log('📋 [WORKFLOW] Using JSON input:', config.jsonValue);
          } else if (config.defaultValue) {
            // Pass text as input
            workflowInput.input = config.defaultValue;
            console.log('📝 [WORKFLOW] Using text input:', workflowInput.input);
          }
        }
      }

      console.log('▶️ [WORKFLOW] Running workflow for agent:', updatedAgent.id);
      await workflowExecutionService.executeWorkflow(updatedAgent.id, workflowInput);
    } catch (error) {
      console.error('Failed to execute workflow:', error);
      setExecutionError(error instanceof Error ? error.message : 'Execution failed');
    }
  }, [currentAgent?.id, workflow, saveCurrentAgent]);

  // Run workflow with custom test input (from inline dialog)
  const handleRunWithTestInput = useCallback(
    async (input: string) => {
      if (!currentAgent?.id || !workflow || workflow.blocks.length === 0) return;

      // Save input to localStorage
      localStorage.setItem(`clara_test_input_${currentAgent.id}`, input);
      setShowTestInputDialog(false);
      setExecutionError(null);

      try {
        await saveCurrentAgent();
        const updatedAgent = useAgentBuilderStore.getState().currentAgent;
        if (!updatedAgent?.id) throw new Error('Failed to save agent');

        // Try to parse as JSON, fall back to plain text
        let parsedInput: unknown = input;
        try {
          parsedInput = JSON.parse(input);
        } catch {
          // Not JSON — use as plain string
        }

        await workflowExecutionService.executeWorkflow(updatedAgent.id, { input: parsedInput });
      } catch (error) {
        console.error('Failed to execute workflow:', error);
        setExecutionError(error instanceof Error ? error.message : 'Execution failed');
      }
    },
    [currentAgent?.id, workflow, saveCurrentAgent]
  );

  // Handle showing validation error message when user clicks Play
  const handleRunButtonClick = useCallback(() => {
    // Don't do anything if workflow is already running
    if (executionStatus === 'running') {
      return;
    }

    if (!isWorkflowValid) {
      const missing = workflowValidation.missingItems[0];
      let message = 'Add blocks to your workflow to get started.';

      if (missing === 'model selection') {
        message = 'Select a model in the toolbar to run LLM blocks.';
      }

      toast.warning(message, 'Cannot Run Workflow');
      return;
    }

    // Check for credential issues - show setup panel if any
    if (!comprehensiveValidation.isValid && comprehensiveValidation.issues.length > 0) {
      setShowSetupPanel(true);
      return;
    }

    // Check if we need test input (no trigger block and no Start block with data)
    if (workflow) {
      const hasTrigger = workflow.blocks.some(
        b => b.type === 'webhook_trigger' || b.type === 'schedule_trigger'
      );
      const startBlock = workflow.blocks.find(
        b =>
          b.type === 'variable' &&
          b.config.type === 'variable' &&
          (b.config as unknown as Record<string, unknown>)?.operation === 'read' &&
          (b.config as unknown as Record<string, unknown>)?.variableName === 'input'
      );
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const startHasData = startBlock && !!(startBlock.config as any)?.defaultValue;

      if (!hasTrigger && !startHasData) {
        // Check if we have saved test input
        const savedInput = localStorage.getItem(`clara_test_input_${currentAgent?.id}`);
        if (savedInput) {
          handleRunWithTestInput(savedInput);
          return;
        }
        // Show inline test input dialog
        setShowTestInputDialog(true);
        return;
      }
    }

    handleRunWorkflow();
  }, [
    executionStatus,
    isWorkflowValid,
    workflowValidation.missingItems,
    comprehensiveValidation,
    handleRunWorkflow,
    handleRunWithTestInput,
    workflow,
    currentAgent?.id,
  ]);

  // Handle stopping the execution
  const handleStopExecution = useCallback(() => {
    workflowExecutionService.disconnect();
    clearExecution();
  }, [clearExecution]);

  // Handle auto-layout with fitView
  const handleAutoLayout = useCallback(() => {
    autoLayoutWorkflow();
    // Fit view after layout to center the workflow
    setTimeout(() => {
      reactFlowInstance.current?.fitView({ padding: 0.2 });
    }, 150);
  }, [autoLayoutWorkflow]);

  // Handle deploying the agent - opens deploy panel
  const [isTogglingDeploy, setIsTogglingDeploy] = useState(false);

  const handleDeployToggle = useCallback(async () => {
    if (!currentAgent || isTogglingDeploy) return;

    setIsTogglingDeploy(true);
    try {
      if (currentAgent.status === 'deployed') {
        // Undeploy → draft
        await updateAgentStatus('draft');
        toast.success('Agent deactivated');
      } else {
        // Save first, then deploy
        await saveCurrentAgent();
        await updateAgentStatus('deployed');
        toast.success('Agent deployed and active');
      }
    } catch (error) {
      console.error('Failed to toggle deploy:', error);
      toast.error('Failed to update deploy status');
    } finally {
      setIsTogglingDeploy(false);
    }
  }, [currentAgent, isTogglingDeploy, updateAgentStatus, saveCurrentAgent]);

  // Handle save — every manual save creates a version snapshot
  const handleSave = useCallback(async () => {
    setIsSaving(true);
    try {
      await saveCurrentAgent(true);
    } catch (error) {
      console.error('Failed to save:', error);
    } finally {
      setIsSaving(false);
    }
  }, [saveCurrentAgent]);

  // Ctrl+S / Cmd+S → Save (must be after handleSave declaration)
  useEffect(() => {
    const handleCtrlS = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault();
        handleSave();
      }
    };

    window.addEventListener('keydown', handleCtrlS);
    return () => window.removeEventListener('keydown', handleCtrlS);
  }, [handleSave]);

  // Warn on tab close/refresh if there are unsaved changes
  useEffect(() => {
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      if (useAgentBuilderStore.getState().isDirty) {
        e.preventDefault();
        e.returnValue = 'You have unsaved changes. Are you sure you want to leave?';
        return e.returnValue;
      }
    };

    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, []);

  // Local state for smooth dragging - synced from store
  const [localNodes, setLocalNodes] = useState<Node[]>([]);

  // Track if we're currently dragging to prevent store sync during drag
  const isDraggingRef = useRef(false);

  // Determine which block states to show on canvas:
  // - In execution viewer mode with a selected execution, use historical block states
  // - Otherwise, use live block states from the current execution
  const isExecutionViewer = executionViewerMode === 'executions';
  const effectiveBlockStates = useMemo(() => {
    if (isExecutionViewer && selectedExecutionData?.blockStates) {
      return Object.fromEntries(
        Object.entries(selectedExecutionData.blockStates).map(([id, state]) => [
          id,
          {
            status: state.status as 'pending' | 'running' | 'completed' | 'failed' | 'skipped',
            error: state.error,
            startedAt: state.startedAt,
            completedAt: state.completedAt,
          },
        ])
      );
    }
    return blockStates;
  }, [isExecutionViewer, selectedExecutionData?.blockStates, blockStates]);

  // Sync store nodes to local state (when store changes externally)
  useEffect(() => {
    // Don't sync if we're in the middle of dragging
    if (isDraggingRef.current) return;

    // Add execution state to nodes from store
    const nodesWithState = storeNodes.map(node => ({
      ...node,
      data: {
        ...node.data,
        executionState: effectiveBlockStates[node.id],
      },
    }));
    setLocalNodes(nodesWithState);
  }, [storeNodes, effectiveBlockStates]);

  // React Flow instance for programmatic control
  const reactFlowInstance = useRef<ReactFlowInstance | null>(null);

  // Store ReactFlow instance on init and fit view
  const handleInit = useCallback((instance: ReactFlowInstance) => {
    reactFlowInstance.current = instance;
    // Fit view after a short delay to ensure nodes are rendered
    setTimeout(() => {
      instance.fitView({ padding: 0.2 });
    }, 100);
  }, []);

  // Handle node changes - apply to local state for smooth dragging
  const handleNodesChange = useCallback(
    (changes: NodeChange<Node>[]) => {
      // Handle removals immediately — forward to store
      const removeChanges = changes.filter(change => change.type === 'remove');
      if (removeChanges.length > 0) {
        storeOnNodesChange(removeChanges);
        return; // Store will rebuild nodes, no need to apply locally
      }

      // Check if any change is a drag start/in-progress
      const isDragging = changes.some(
        change => change.type === 'position' && change.dragging === true
      );

      // Check if drag just ended
      const dragEnded = changes.some(
        change => change.type === 'position' && change.dragging === false
      );

      if (isDragging) {
        isDraggingRef.current = true;
      }

      // Apply ALL position changes to local state for smooth visual movement
      setLocalNodes(nodes => applyNodeChanges(changes, nodes));

      // When drag ends, persist final position to store
      if (dragEnded) {
        isDraggingRef.current = false;
        const positionChanges = changes.filter(
          change => change.type === 'position' && change.dragging === false
        );
        if (positionChanges.length > 0) {
          storeOnNodesChange(positionChanges);
        }
      }
    },
    [storeOnNodesChange]
  );

  // Handle edge changes — forward removals to store
  const handleEdgesChange = useCallback(
    (changes: EdgeChange<Edge>[]) => {
      const removeChanges = changes.filter(change => change.type === 'remove');
      if (removeChanges.length > 0) {
        storeOnEdgesChange(removeChanges);
      }
    },
    [storeOnEdgesChange]
  );

  // Handle new connections
  const handleConnect = useCallback(
    (connection: Connection) => {
      if (connection.source && connection.target) {
        storeOnConnect(connection);
      }
    },
    [storeOnConnect]
  );

  // Transform edges to add flow animation classes and highlighting
  const styledEdges: Edge[] = useMemo(() => {
    return storeEdges.map(edge => {
      const sourceState = blockStates[edge.source];
      const targetState = blockStates[edge.target];
      const isHighlighted = highlightedEdgeIds.includes(edge.id);

      let edgeClassName = '';

      // Source completed and target running = flowing data
      if (sourceState?.status === 'completed' && targetState?.status === 'running') {
        edgeClassName = 'edge-flowing';
      }
      // Both completed = completed edge
      else if (sourceState?.status === 'completed' && targetState?.status === 'completed') {
        edgeClassName = 'edge-completed';
      }
      // Highlighted edge (when block is selected)
      else if (isHighlighted) {
        edgeClassName = 'edge-highlighted';
      }

      return {
        ...edge,
        className: edgeClassName,
        style: {
          ...edge.style,
          strokeWidth: isHighlighted ? 3 : 2,
          stroke: isHighlighted ? 'var(--color-accent)' : undefined,
        },
      };
    });
  }, [storeEdges, blockStates, highlightedEdgeIds]);

  // Empty state - no agent selected
  if (!currentAgent) {
    return (
      <div
        className={cn(
          'flex flex-col items-center justify-center h-full bg-[var(--color-bg-primary)]',
          className
        )}
      >
        <div className="text-center p-8">
          <div className="w-20 h-20 mx-auto mb-5 rounded-2xl bg-[var(--color-bg-tertiary)] flex items-center justify-center border border-[var(--color-border)]">
            <svg
              width="36"
              height="36"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              className="text-white"
            >
              <rect x="3" y="3" width="7" height="7" rx="1" />
              <rect x="14" y="3" width="7" height="7" rx="1" />
              <rect x="3" y="14" width="7" height="7" rx="1" />
              <rect x="14" y="14" width="7" height="7" rx="1" />
              <path d="M10 6.5h4M6.5 10v4M17.5 10v4M10 17.5h4" />
            </svg>
          </div>
          <h3 className="text-lg font-semibold text-[var(--color-text-primary)] mb-2">
            No Workflow
          </h3>
          <p className="text-sm text-[var(--color-text-secondary)] max-w-[300px] leading-relaxed">
            Select an agent to view and edit its workflow, or create a new agent to get started.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className={cn('relative h-full flex', className)}>
      {/* Left sidebar: Mode Toggle + Execution List OR Block Palette */}
      {(isExecutionViewer || showBlockPalette) && (
        <div
          className={cn(
            'h-full border-r border-[var(--color-border)] flex-shrink-0 z-10 flex flex-col',
            isExecutionViewer ? 'w-72' : 'w-64'
          )}
        >
          {/* Mode Toggle: Editor / Executions */}
          <div className="px-3 py-2.5 border-b border-[var(--color-border)] flex items-center justify-center">
            <div className="flex items-center w-full rounded-lg bg-[var(--color-bg-tertiary)] p-0.5">
              <button
                onClick={() => setExecutionViewerMode('editor')}
                className={cn(
                  'flex-1 flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-all',
                  !isExecutionViewer
                    ? 'bg-[var(--color-bg-primary)] text-[var(--color-text-primary)] shadow-sm'
                    : 'text-[var(--color-text-tertiary)] hover:text-[var(--color-text-secondary)]'
                )}
              >
                <Pencil size={12} />
                Editor
              </button>
              <button
                onClick={() => setExecutionViewerMode('executions')}
                className={cn(
                  'flex-1 flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-medium transition-all',
                  isExecutionViewer
                    ? 'bg-[var(--color-bg-primary)] text-[var(--color-text-primary)] shadow-sm'
                    : 'text-[var(--color-text-tertiary)] hover:text-[var(--color-text-secondary)]'
                )}
              >
                <History size={12} />
                Executions
              </button>
            </div>
          </div>

          {/* Panel content */}
          <div className="flex-1 min-h-0">
            {isExecutionViewer ? (
              <ExecutionListPanel />
            ) : (
              <BlockPalette onClose={() => setShowBlockPalette(false)} />
            )}
          </div>
        </div>
      )}

      {/* Canvas area */}
      <div className="relative flex-1 h-full">
        {/* SVG Gradient Definition for flowing edges */}
        <svg width="0" height="0" style={{ position: 'absolute' }}>
          <defs>
            <linearGradient id="flowGradient" x1="0%" y1="0%" x2="100%" y2="0%">
              <stop offset="0%" stopColor="#06b6d4" />
              <stop offset="50%" stopColor="#8b5cf6" />
              <stop offset="100%" stopColor="#ec4899" />
            </linearGradient>
          </defs>
        </svg>

        <ReactFlow
          nodes={localNodes}
          edges={styledEdges}
          onNodesChange={isExecutionViewer ? undefined : handleNodesChange}
          onEdgesChange={isExecutionViewer ? undefined : handleEdgesChange}
          onConnect={isExecutionViewer ? undefined : handleConnect}
          onInit={handleInit}
          onPaneClick={() => (isExecutionViewer ? setInspectedBlockId(null) : selectBlock(null))}
          onNodeClick={isExecutionViewer ? (_e, node) => setInspectedBlockId(node.id) : undefined}
          nodeTypes={nodeTypes}
          edgeTypes={edgeTypes}
          deleteKeyCode={isExecutionViewer ? null : ['Backspace', 'Delete']}
          nodesDraggable={!isExecutionViewer}
          nodesConnectable={!isExecutionViewer}
          edgesFocusable={!isExecutionViewer}
          fitView
          fitViewOptions={{ padding: 0.2 }}
          proOptions={{ hideAttribution: true }}
          className="flex-1 min-h-0 bg-[var(--color-bg-primary)]"
        >
          {/* Background Grid */}
          <Background
            variant={BackgroundVariant.Dots}
            gap={20}
            size={1}
            color="var(--color-border)"
          />

          {/* Controls */}
          <Controls
            showInteractive={false}
            className="!bg-[var(--color-bg-secondary)] !border !border-[var(--color-border)] !rounded-xl !z-[5] [&_button]:!bg-[var(--color-bg-primary)] [&_button]:!text-[var(--color-text-primary)] [&_button]:!border-[var(--color-border)] [&_button]:hover:!bg-[var(--color-bg-tertiary)]"
          />

          {/* Mini Map - hidden to avoid z-index issues with nodes */}
          {/* <MiniMap
          nodeColor={node => {
            const state = blockStates[node.id];
            if (state?.status === 'completed') return 'var(--color-success)';
            if (state?.status === 'failed') return 'var(--color-error)';
            if (state?.status === 'running') return 'var(--color-accent)';
            return 'var(--color-bg-tertiary)';
          }}
          maskColor="rgba(0, 0, 0, 0.2)"
          className="!bg-[var(--color-bg-secondary)] !border !border-[var(--color-border)] !rounded-xl"
        /> */}

          {/* Show Blocks button when sidebar is fully hidden */}
          {!isExecutionViewer && !showBlockPalette && (
            <Panel position="top-left">
              <ToolbarButton
                icon={<PanelLeftOpen size={16} />}
                tooltip="Show Block Palette"
                label="Blocks"
                onClick={() => setShowBlockPalette(true)}
              />
            </Panel>
          )}

          {/* Toolbar Panel — grouped into logical sections */}
          <Panel position="top-right" className="flex items-center gap-1.5">
            {/* ── Group 1: History ── */}
            <div className="flex items-center gap-1 rounded-xl bg-black/20 backdrop-blur-sm px-1.5 py-1 border border-white/[0.06]">
              <ToolbarButton
                icon={<Undo size={16} />}
                tooltip="Undo (Ctrl+Z)"
                onClick={undo}
                disabled={workflowHistory.length === 0}
              />
              <ToolbarButton
                icon={<Redo size={16} />}
                tooltip="Redo (Ctrl+Y)"
                onClick={redo}
                disabled={workflowFuture.length === 0}
              />
              {workflowVersions.length > 0 && (
                <div className="relative">
                  <button
                    onClick={() => setShowVersionDropdown(!showVersionDropdown)}
                    className={cn(
                      'flex items-center gap-1.5 px-2 py-1.5 rounded-lg transition-all text-xs font-medium',
                      showVersionDropdown
                        ? 'bg-[var(--color-accent)]/15 text-[var(--color-accent)]'
                        : 'text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-white/5'
                    )}
                    title="Workflow versions"
                  >
                    <History size={13} />
                    <span>{workflowVersions.length}</span>
                    <ChevronDown
                      size={10}
                      className={cn('transition-transform', showVersionDropdown && 'rotate-180')}
                    />
                  </button>
                  {showVersionDropdown && (
                    <>
                      <div
                        className="fixed inset-0 z-[100]"
                        onClick={() => setShowVersionDropdown(false)}
                      />
                      <div
                        className="absolute top-full right-0 mt-2 z-[101] w-72 max-h-[300px] overflow-y-auto rounded-xl bg-[#1a1a1a] border border-[var(--color-border)] shadow-xl"
                        onWheelCapture={e => e.stopPropagation()}
                      >
                        <div className="px-3 py-2 border-b border-[var(--color-border)]">
                          <span className="text-xs font-medium text-[var(--color-text-primary)]">
                            Version History
                          </span>
                          <p className="text-[10px] text-[var(--color-text-tertiary)] mt-0.5">
                            Click to restore a previous version
                          </p>
                        </div>
                        <div className="py-1">
                          {[...workflowVersions].reverse().map(version => (
                            <button
                              key={version.id}
                              onClick={() => {
                                restoreWorkflowVersion(version.version);
                                setShowVersionDropdown(false);
                              }}
                              className="w-full px-3 py-2 text-left hover:bg-[var(--color-bg-tertiary)] transition-colors"
                            >
                              <div className="flex items-center justify-between">
                                <span className="text-xs font-medium text-[var(--color-accent)]">
                                  v{version.version}
                                </span>
                                <span className="text-[10px] text-[var(--color-text-tertiary)]">
                                  {new Date(version.createdAt).toLocaleTimeString([], {
                                    hour: '2-digit',
                                    minute: '2-digit',
                                  })}
                                </span>
                              </div>
                              <p className="text-[10px] text-[var(--color-text-secondary)] truncate mt-0.5">
                                {version.description || `${version.blockCount} blocks`}
                              </p>
                            </button>
                          ))}
                        </div>
                      </div>
                    </>
                  )}
                </div>
              )}
            </div>

            {/* ── Group 2: Canvas Actions ── */}
            <div className="flex items-center gap-1 rounded-xl bg-black/20 backdrop-blur-sm px-1.5 py-1 border border-white/[0.06]">
              <ToolbarButton
                icon={<LayoutGrid size={16} />}
                tooltip="Auto-arrange blocks"
                onClick={handleAutoLayout}
                disabled={!workflow || workflow.blocks.length === 0}
              />
              <div className="relative">
                <ToolbarButton
                  icon={<Save size={16} />}
                  tooltip={isDirty ? 'Save (Ctrl+S) — Unsaved changes' : 'Save (Ctrl+S)'}
                  onClick={handleSave}
                  disabled={isSaving}
                  className={isDirty ? 'text-amber-400' : ''}
                />
                {isDirty && (
                  <span className="absolute -top-0.5 -right-0.5 w-2 h-2 rounded-full bg-amber-400 animate-pulse pointer-events-none" />
                )}
              </div>
              {lastSavedAt && (
                <span
                  className="text-[10px] text-[var(--color-text-tertiary)] whitespace-nowrap px-1"
                  title={lastSavedAt.toLocaleString()}
                >
                  {getRelativeTime(lastSavedAt)}
                </span>
              )}
              <ToolbarButton
                icon={<Bug size={16} />}
                tooltip={debugMode ? 'Debug Mode ON - Block validation enabled' : 'Debug Mode OFF'}
                onClick={toggleDebugMode}
                className={debugMode ? 'text-yellow-500 bg-yellow-500/10' : ''}
              />
            </div>

            {/* ── Group 3: Model & Credentials ── */}
            <div className="flex items-center gap-1.5 rounded-xl bg-black/20 backdrop-blur-sm px-1.5 py-1 border border-white/[0.06]">
              {/* Model Selector */}
              <div className="relative flex items-center">
                <Cpu
                  size={13}
                  className="absolute left-2 text-[var(--color-text-tertiary)] pointer-events-none z-10"
                />
                <select
                  value={workflow?.workflowModelId || ''}
                  onChange={e => setWorkflowModelId(e.target.value)}
                  disabled={modelsLoading || !workflow}
                  className="pl-7 pr-5 py-1.5 rounded-lg text-xs bg-white/5 text-[var(--color-text-primary)] border-none hover:bg-white/10 focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)]/50 appearance-none cursor-pointer max-w-[160px] truncate"
                  title="Workflow model — used by all LLM blocks"
                >
                  <option value="">Select model</option>
                  {agentModels.map(model => (
                    <option key={model.id} value={model.id}>
                      {model.name || model.id}
                    </option>
                  ))}
                </select>
                <ChevronDown
                  size={10}
                  className="absolute right-1.5 text-[var(--color-text-tertiary)] pointer-events-none"
                />
              </div>

              {/* Credentials */}
              {workflowCredentialInfo.toolsNeedingCredentials.length > 0 && (
                <div className="relative">
                  <button
                    onClick={() => setShowCredentialsDropdown(!showCredentialsDropdown)}
                    title={
                      workflowCredentialInfo.unconfiguredTools.length > 0
                        ? 'Some integrations need credentials'
                        : 'Manage credentials'
                    }
                    className={cn(
                      'rounded-lg transition-all flex items-center gap-1.5 px-2 py-1.5',
                      workflowCredentialInfo.unconfiguredTools.length > 0
                        ? 'bg-amber-500/20 text-amber-400 hover:bg-amber-500/30'
                        : 'text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-white/5'
                    )}
                  >
                    {workflowCredentialInfo.unconfiguredTools.length > 0 ? (
                      <AlertTriangle size={14} />
                    ) : (
                      <KeyRound size={14} />
                    )}
                    {workflowCredentialInfo.unconfiguredTools.length > 0 && (
                      <span className="text-[11px] font-medium">Setup</span>
                    )}
                  </button>

                  {showCredentialsDropdown && (
                    <>
                      <div
                        className="fixed inset-0 z-[100]"
                        onClick={() => setShowCredentialsDropdown(false)}
                      />
                      <div
                        className="absolute top-full right-0 mt-2 z-[101] w-80 max-h-[400px] overflow-y-auto rounded-xl bg-[#1a1a1a] border border-[var(--color-border)] shadow-xl"
                        onWheelCapture={e => e.stopPropagation()}
                      >
                        <div className="px-3 py-2 border-b border-[var(--color-border)]">
                          <span className="text-xs font-medium text-[var(--color-text-primary)]">
                            Workflow Integrations
                          </span>
                          <p className="text-[10px] text-[var(--color-text-tertiary)] mt-0.5">
                            {workflowCredentialInfo.unconfiguredTools.length > 0
                              ? 'Some integrations need credentials to work'
                              : 'All integrations are configured'}
                          </p>
                        </div>
                        {workflowCredentialInfo.unconfiguredTools.length > 0 && (
                          <div className="px-3 py-2 bg-amber-500/5 border-b border-[var(--color-border)]">
                            <div className="flex items-center gap-2 text-amber-400 mb-2">
                              <AlertTriangle size={12} />
                              <span className="text-xs font-medium">Missing Credentials</span>
                            </div>
                            <div className="space-y-1">
                              {workflowCredentialInfo.unconfiguredTools.map(
                                ({ tool, displayName }) => (
                                  <div
                                    key={tool}
                                    className="flex items-center gap-2 text-xs text-[var(--color-text-secondary)]"
                                  >
                                    <div className="w-1.5 h-1.5 rounded-full bg-amber-400" />
                                    <span>{displayName}</span>
                                  </div>
                                )
                              )}
                            </div>
                          </div>
                        )}
                        {workflowCredentialInfo.configuredTools.length > 0 && (
                          <div className="px-3 py-2 border-b border-[var(--color-border)]">
                            <div className="flex items-center gap-2 text-green-400 mb-2">
                              <KeyRound size={12} />
                              <span className="text-xs font-medium">Configured</span>
                            </div>
                            <div className="space-y-1">
                              {workflowCredentialInfo.configuredTools.map(
                                ({ tool, displayName }) => (
                                  <div
                                    key={tool}
                                    className="flex items-center gap-2 text-xs text-[var(--color-text-secondary)]"
                                  >
                                    <div className="w-1.5 h-1.5 rounded-full bg-green-400" />
                                    <span>{displayName}</span>
                                  </div>
                                )
                              )}
                            </div>
                          </div>
                        )}
                        <div className="p-2">
                          <Link
                            to="/credentials"
                            onClick={() => setShowCredentialsDropdown(false)}
                            className="flex items-center justify-center gap-2 w-full px-3 py-2 text-xs font-medium text-[var(--color-accent)] bg-[var(--color-accent)]/10 hover:bg-[var(--color-accent)]/20 rounded-lg transition-colors"
                          >
                            <ExternalLink size={12} />
                            <span>Manage All Credentials</span>
                          </Link>
                        </div>
                      </div>
                    </>
                  )}
                </div>
              )}
            </div>

            {/* ── Group 4: Run & Deploy ── */}
            <div className="relative flex items-center gap-1 rounded-xl bg-black/20 backdrop-blur-sm px-1.5 py-1 border border-white/[0.06]">
              {executionStatus === 'running' ? (
                <button
                  onClick={handleStopExecution}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-red-500/90 text-white text-xs font-medium hover:bg-red-500 transition-all"
                  title="Stop Execution"
                >
                  <Square size={13} />
                  <span>Stop</span>
                </button>
              ) : (
                <button
                  onClick={handleRunButtonClick}
                  disabled={executionStatus === 'running'}
                  className={cn(
                    'flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-all',
                    isWorkflowValid
                      ? 'bg-[var(--color-accent)] text-white hover:opacity-90'
                      : 'bg-[var(--color-accent)]/40 text-white/60 cursor-not-allowed'
                  )}
                  title={!isWorkflowValid ? 'Add blocks and select a model to run' : 'Run Workflow'}
                >
                  <Play size={13} />
                  <span>Run</span>
                </button>
              )}

              <div className="w-px h-5 bg-white/10" />

              {/* Deploy Toggle */}
              <button
                onClick={handleDeployToggle}
                disabled={!workflow || workflow.blocks.length === 0 || isTogglingDeploy}
                title={
                  currentAgent?.status === 'deployed' ? 'Click to deactivate' : 'Click to deploy'
                }
                className={cn(
                  'flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs font-medium transition-all',
                  currentAgent?.status === 'deployed'
                    ? 'bg-emerald-500/15 text-emerald-400 hover:bg-emerald-500/25'
                    : currentAgent?.status === 'paused'
                      ? 'bg-amber-500/15 text-amber-400 hover:bg-amber-500/25'
                      : 'text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] hover:bg-white/5',
                  isTogglingDeploy && 'opacity-50 pointer-events-none'
                )}
              >
                <span
                  className={cn(
                    'w-2 h-2 rounded-full',
                    currentAgent?.status === 'deployed' && 'bg-emerald-400 animate-pulse',
                    currentAgent?.status === 'paused' && 'bg-amber-400',
                    (!currentAgent || currentAgent.status === 'draft') && 'bg-gray-400'
                  )}
                />
                {currentAgent?.status === 'deployed'
                  ? 'Active'
                  : currentAgent?.status === 'paused'
                    ? 'Paused'
                    : 'Deploy'}
              </button>

              {/* Deploy settings gear — only when deployed */}
              {currentAgent?.status === 'deployed' && (
                <ToolbarButton
                  icon={<Settings size={14} />}
                  tooltip="Deploy Settings (API, webhooks, schedules)"
                  onClick={() => setShowDeployPanel(true)}
                />
              )}

              {/* Inline Test Input Dialog — positioned below this group */}
              {showTestInputDialog && (
                <div className="absolute top-full right-0 mt-2 w-80 bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-xl shadow-2xl p-4 z-50">
                  <div className="flex items-center justify-between mb-2">
                    <span className="text-xs font-semibold text-[var(--color-text-primary)]">
                      Test Input
                    </span>
                    <button
                      onClick={() => setShowTestInputDialog(false)}
                      className="text-[var(--color-text-tertiary)] hover:text-[var(--color-text-primary)] p-0.5"
                    >
                      <Square size={10} />
                    </button>
                  </div>
                  <textarea
                    value={testInput}
                    onChange={e => setTestInput(e.target.value)}
                    placeholder="Enter test input (text or JSON)..."
                    className="w-full h-24 px-3 py-2 rounded-lg bg-white/5 text-sm text-[var(--color-text-primary)] border border-[var(--color-border)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)]/50 resize-none font-mono"
                    autoFocus
                    onKeyDown={e => {
                      if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
                        handleRunWithTestInput(testInput);
                      }
                    }}
                  />
                  <div className="flex items-center justify-between mt-2">
                    <span className="text-[10px] text-[var(--color-text-tertiary)]">
                      Ctrl+Enter to run
                    </span>
                    <button
                      onClick={() => handleRunWithTestInput(testInput)}
                      disabled={!testInput.trim()}
                      className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-[var(--color-accent)] text-white text-xs font-medium hover:opacity-90 disabled:opacity-50 transition-all"
                    >
                      <Play size={12} />
                      Run
                    </button>
                  </div>
                </div>
              )}
            </div>

            {/* ── Group 5: Output ── */}
            <ToolbarButton
              icon={<FileOutput size={16} />}
              tooltip="View Output"
              onClick={() => setShowOutputPanel(!showOutputPanel)}
              disabled={!executionStatus || executionStatus === 'running'}
            />
          </Panel>

          {/* Status Panel with Glass Effect */}
          {executionStatus && (
            <Panel position="bottom-center">
              <div
                className={cn(
                  'px-5 py-2.5 rounded-xl shadow-xl backdrop-blur-md flex items-center gap-2.5 border font-medium text-sm',
                  executionStatus === 'running' &&
                    'bg-[var(--color-accent)] bg-opacity-90 text-white border-[var(--color-accent)]',
                  executionStatus === 'completed' &&
                    'bg-[var(--color-surface-elevated)] text-[var(--color-text-primary)] border-[var(--color-border)]',
                  executionStatus === 'failed' &&
                    'bg-red-500 bg-opacity-90 text-white border-red-400',
                  executionStatus === 'partial_failure' &&
                    'bg-yellow-500 bg-opacity-90 text-white border-yellow-400'
                )}
              >
                {executionStatus === 'running' && (
                  <>
                    <div className="w-2 h-2 rounded-full bg-white animate-pulse shadow-md" />
                    <span>Running...</span>
                  </>
                )}
                {executionStatus === 'completed' && <span>✓ Execution completed</span>}
                {executionStatus === 'failed' && <span>✗ Execution failed</span>}
                {executionStatus === 'partial_failure' && <span>⚠ Partial failure</span>}
              </div>
            </Panel>
          )}

          {/* Empty workflow message */}
          {(!workflow || workflow.blocks.length === 0) && (
            <Panel position="top-center" className="mt-20">
              <div className="text-center p-6 rounded-xl bg-[var(--color-bg-secondary)] border border-[var(--color-border)] shadow-lg">
                <p className="text-sm text-[var(--color-text-secondary)] mb-2">
                  Describe your agent in the chat to generate a workflow
                </p>
                <p className="text-xs text-[var(--color-text-tertiary)]">
                  Or manually add blocks using the sidebar
                </p>
              </div>
            </Panel>
          )}

          {/* Onboarding Guidance */}
          {showOnboardingGuidance && (
            <Panel position="bottom-center" className="mb-8">
              <OnboardingGuidance onRun={handleRunWorkflow} onDeploy={handleDeploy} />
            </Panel>
          )}
        </ReactFlow>

        {/* Execution Output Panel */}
        {showOutputPanel && <ExecutionOutputPanel onClose={() => setShowOutputPanel(false)} />}

        {/* Setup Required Panel - shows when clicking Run with missing credentials */}
        <SetupRequiredPanel
          validation={comprehensiveValidation}
          isOpen={showSetupPanel}
          onClose={() => setShowSetupPanel(false)}
          onSelectBlock={blockId => {
            // Select the block and close the panel
            useAgentBuilderStore.getState().selectBlock(blockId);
          }}
          onOpenCredentials={integrationTypes => {
            // Navigate to credentials page with required integrations as query params
            setShowSetupPanel(false);
            const queryParams =
              integrationTypes.length > 0 ? `?required=${integrationTypes.join(',')}` : '';
            navigate(`/credentials${queryParams}`);
          }}
        />

        {/* Deploy Panel - shows when clicking Deploy button */}
        <DeployPanel isOpen={showDeployPanel} onClose={() => setShowDeployPanel(false)} />
      </div>
      {/* end canvas area */}
    </div>
  );
}

// Relative time helper for "Saved Xm ago" display
function getRelativeTime(date: Date): string {
  const diffSec = Math.floor((Date.now() - date.getTime()) / 1000);
  if (diffSec < 10) return 'Just saved';
  if (diffSec < 60) return `Saved ${diffSec}s ago`;
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `Saved ${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  return `Saved ${diffHr}h ago`;
}

// Toolbar Button Component
interface ToolbarButtonProps {
  icon: React.ReactNode;
  tooltip: string;
  label?: string;
  onClick?: () => void;
  disabled?: boolean;
  primary?: boolean;
  className?: string;
}

function ToolbarButton({
  icon,
  tooltip,
  label,
  onClick,
  disabled,
  primary,
  className,
}: ToolbarButtonProps) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      title={tooltip}
      className={cn(
        'rounded-xl transition-all flex items-center gap-1.5',
        label ? 'px-3 py-2' : 'p-2.5',
        primary
          ? 'bg-[var(--color-accent)] text-white hover:bg-[var(--color-accent-hover)]'
          : 'bg-[var(--color-bg-secondary)] text-[var(--color-text-secondary)] hover:bg-[var(--color-bg-tertiary)] hover:text-[var(--color-text-primary)] border border-[var(--color-border)]',
        disabled && 'opacity-50 cursor-not-allowed',
        className
      )}
    >
      {icon}
      {label && <span className="text-xs font-medium">{label}</span>}
    </button>
  );
}
