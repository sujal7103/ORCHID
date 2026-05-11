import { useState, useEffect, useCallback, useRef } from 'react';
import {
  History,
  X,
  Loader2,
  CheckCircle,
  XCircle,
  Clock,
  Play,
  Key,
  Globe,
  RefreshCw,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { useAgentBuilderStore } from '@/store/useAgentBuilderStore';
import {
  getAgentExecutions,
  getExecution,
  formatDuration,
  formatTimestamp,
  getStatusColor,
  getTriggerTypeLabel,
  type ExecutionRecord,
  type PaginatedExecutions,
} from '@/services/executionService';

export function ExecutionListPanel() {
  const {
    selectedAgentId,
    selectedExecutionId,
    executionViewerMode,
    setExecutionViewerMode,
    selectExecution,
    setSelectedExecutionData,
  } = useAgentBuilderStore();

  const [isLoading, setIsLoading] = useState(false);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [data, setData] = useState<PaginatedExecutions | null>(null);
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [loadingExecutionId, setLoadingExecutionId] = useState<string | null>(null);

  // Track previous running IDs to detect new running executions
  const prevRunningIdsRef = useRef<Set<string>>(new Set());

  const loadExecutions = useCallback(
    async (offset = 0) => {
      if (!selectedAgentId) return;

      if (offset === 0) {
        setIsLoading(true);
      } else {
        setIsLoadingMore(true);
      }

      try {
        const result = await getAgentExecutions(selectedAgentId, {
          limit: 20,
          offset,
          status: statusFilter !== 'all' ? statusFilter : undefined,
        });

        if (offset === 0) {
          setData(result);
        } else {
          setData(prev =>
            prev
              ? {
                  ...result,
                  executions: [...prev.executions, ...result.executions],
                }
              : result
          );
        }

        return result;
      } catch (err) {
        console.error('Failed to load executions:', err);
      } finally {
        setIsLoading(false);
        setIsLoadingMore(false);
      }
    },
    [selectedAgentId, statusFilter]
  );

  // Initial load
  useEffect(() => {
    if (executionViewerMode === 'executions' && selectedAgentId) {
      loadExecutions();
    }
  }, [executionViewerMode, selectedAgentId, loadExecutions]);

  // Auto-refresh every 5 seconds
  useEffect(() => {
    if (executionViewerMode !== 'executions' || !selectedAgentId) return;

    const interval = setInterval(async () => {
      const result = await getAgentExecutions(selectedAgentId, {
        limit: 20,
        status: statusFilter !== 'all' ? statusFilter : undefined,
      });

      if (result) {
        setData(result);

        // Detect new running executions and auto-select
        const currentRunningIds = new Set(
          result.executions.filter(e => e.status === 'running').map(e => e.id)
        );
        for (const id of currentRunningIds) {
          if (!prevRunningIdsRef.current.has(id)) {
            // New running execution detected — auto-select it
            handleSelectExecution(id, result.executions);
            break;
          }
        }
        prevRunningIdsRef.current = currentRunningIds;

        // If we have a selected execution that's running, refresh its data
        if (selectedExecutionId) {
          const selected = result.executions.find(e => e.id === selectedExecutionId);
          if (selected && (selected.status === 'running' || selected.status === 'pending')) {
            const fullData = await getExecution(selectedExecutionId);
            if (fullData) {
              setSelectedExecutionData(fullData);
            }
          }
        }
      }
    }, 5000);

    return () => clearInterval(interval);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [executionViewerMode, selectedAgentId, statusFilter, selectedExecutionId]);

  const handleSelectExecution = useCallback(
    async (executionId: string, executions?: ExecutionRecord[]) => {
      if (executionId === selectedExecutionId) return;

      setLoadingExecutionId(executionId);
      selectExecution(executionId);

      try {
        // First check if we already have blockStates in the list data
        const listExec = (executions || data?.executions)?.find(e => e.id === executionId);
        if (listExec?.blockStates && Object.keys(listExec.blockStates).length > 0) {
          setSelectedExecutionData(listExec);
        } else {
          // Fetch full execution with blockStates
          const fullExec = await getExecution(executionId);
          if (fullExec) {
            setSelectedExecutionData(fullExec);
          }
        }
      } catch (err) {
        console.error('Failed to load execution details:', err);
      } finally {
        setLoadingExecutionId(null);
      }
    },
    [selectedExecutionId, data?.executions, selectExecution, setSelectedExecutionData]
  );

  const handleLoadMore = () => {
    if (data) {
      loadExecutions(data.executions.length);
    }
  };

  const getStatusIcon = (status: ExecutionRecord['status']) => {
    switch (status) {
      case 'completed':
        return <CheckCircle size={14} className="text-green-400" />;
      case 'failed':
        return <XCircle size={14} className="text-red-400" />;
      case 'running':
        return <Loader2 size={14} className="text-[var(--color-accent)] animate-spin" />;
      case 'pending':
        return <Clock size={14} className="text-yellow-400" />;
      default:
        return <Clock size={14} className="text-[var(--color-text-tertiary)]" />;
    }
  };

  const getTriggerIcon = (triggerType: ExecutionRecord['triggerType']) => {
    switch (triggerType) {
      case 'manual':
        return <Play size={10} />;
      case 'scheduled':
        return <Clock size={10} />;
      case 'api':
        return <Key size={10} />;
      case 'webhook':
        return <Globe size={10} />;
      default:
        return null;
    }
  };

  return (
    <div className="flex flex-col h-full bg-[var(--color-bg-secondary)]">
      {/* Header */}
      <div className="flex-shrink-0 flex items-center justify-between px-4 py-3 border-b border-[var(--color-border)]">
        <div className="flex items-center gap-2">
          <History size={16} className="text-[var(--color-accent)]" />
          <span className="text-sm font-semibold text-[var(--color-text-primary)]">Executions</span>
          {data && data.total > 0 && (
            <span className="px-1.5 py-0.5 text-[10px] rounded-full bg-[var(--color-surface)] text-[var(--color-text-secondary)]">
              {data.total}
            </span>
          )}
        </div>
        <button
          onClick={() => setExecutionViewerMode('editor')}
          className="p-1 rounded-md hover:bg-[var(--color-surface-hover)] text-[var(--color-text-tertiary)] transition-colors"
          title="Back to Editor"
        >
          <X size={16} />
        </button>
      </div>

      {/* Filter bar */}
      <div className="flex-shrink-0 flex items-center gap-2 px-3 py-2 border-b border-[var(--color-border)]">
        <select
          value={statusFilter}
          onChange={e => setStatusFilter(e.target.value)}
          className="flex-1 px-2 py-1.5 rounded-md bg-[var(--color-bg-primary)] border border-[var(--color-border)] text-xs text-[var(--color-text-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--color-accent)] [color-scheme:dark]"
        >
          <option value="all">All Status</option>
          <option value="completed">Completed</option>
          <option value="failed">Failed</option>
          <option value="running">Running</option>
          <option value="pending">Pending</option>
        </select>
        <button
          onClick={() => loadExecutions(0)}
          disabled={isLoading}
          className="p-1.5 rounded-md hover:bg-[var(--color-surface-hover)] text-[var(--color-text-secondary)] transition-colors disabled:opacity-50"
          title="Refresh"
        >
          {isLoading ? <Loader2 size={14} className="animate-spin" /> : <RefreshCw size={14} />}
        </button>
      </div>

      {/* Execution List */}
      <div className="flex-1 overflow-y-auto">
        {isLoading ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 size={20} className="animate-spin text-[var(--color-text-tertiary)]" />
          </div>
        ) : !data || data.executions.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 px-4 text-center">
            <History size={32} className="text-[var(--color-text-tertiary)] mb-3 opacity-40" />
            <p className="text-sm text-[var(--color-text-secondary)]">No executions yet</p>
            <p className="text-xs text-[var(--color-text-tertiary)] mt-1">
              Run your workflow to see execution history
            </p>
          </div>
        ) : (
          <div className="py-1">
            {data.executions.map(execution => {
              const isSelected = selectedExecutionId === execution.id;
              const isLoadingThis = loadingExecutionId === execution.id;
              const isRunning = execution.status === 'running';

              return (
                <button
                  key={execution.id}
                  onClick={() => handleSelectExecution(execution.id)}
                  className={cn(
                    'w-full flex items-center gap-3 px-3 py-2.5 text-left transition-colors border-l-2',
                    isSelected
                      ? 'bg-[var(--color-accent)]/10 border-l-[var(--color-accent)]'
                      : 'border-l-transparent hover:bg-[var(--color-surface-hover)]'
                  )}
                >
                  {/* Status Icon */}
                  <div className="flex-shrink-0">
                    {isLoadingThis ? (
                      <Loader2 size={14} className="animate-spin text-[var(--color-accent)]" />
                    ) : (
                      getStatusIcon(execution.status)
                    )}
                  </div>

                  {/* Content */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span
                        className="text-xs font-medium capitalize"
                        style={{ color: getStatusColor(execution.status) }}
                      >
                        {execution.status}
                      </span>
                      {isRunning && (
                        <span className="flex items-center gap-1 px-1.5 py-0.5 rounded text-[9px] font-bold bg-[var(--color-accent)]/20 text-[var(--color-accent)]">
                          <span className="w-1.5 h-1.5 rounded-full bg-[var(--color-accent)] animate-pulse" />
                          LIVE
                        </span>
                      )}
                      <span className="text-[10px] text-[var(--color-text-tertiary)]">
                        {formatDuration(execution.durationMs)}
                      </span>
                    </div>
                    <div className="flex items-center gap-2 mt-0.5">
                      <span className="text-[10px] text-[var(--color-text-tertiary)]">
                        {formatTimestamp(execution.startedAt)}
                      </span>
                      <span className="flex items-center gap-0.5 text-[10px] text-[var(--color-text-tertiary)]">
                        {getTriggerIcon(execution.triggerType)}
                        {getTriggerTypeLabel(execution.triggerType)}
                      </span>
                    </div>
                  </div>
                </button>
              );
            })}

            {/* Load More */}
            {data.has_more && (
              <button
                onClick={handleLoadMore}
                disabled={isLoadingMore}
                className="w-full py-3 text-xs text-[var(--color-accent)] hover:text-[var(--color-accent-hover)] disabled:opacity-50 transition-colors"
              >
                {isLoadingMore ? (
                  <span className="flex items-center justify-center gap-2">
                    <Loader2 size={12} className="animate-spin" />
                    Loading...
                  </span>
                ) : (
                  'Load more...'
                )}
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
