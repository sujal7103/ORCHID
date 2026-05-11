import { useState, useEffect, useMemo, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, Search, Cloud, Loader2, Trash2, X, CheckSquare, Square } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useAgentBuilderStore } from '@/store/useAgentBuilderStore';
import { deleteAgent as deleteAgentAPI } from '@/services/agentService';
import { formatDistanceToNow } from '@/utils/dateUtils';

interface AgentListViewProps {
  className?: string;
}

export function AgentListView({ className }: AgentListViewProps) {
  const navigate = useNavigate();
  const {
    backendAgents,
    pagination,
    isLoadingAgents,
    createAgent,
    setActiveView,
    trackAgentAccess,
    fetchAgentsPage,
    loadAgentFromBackend,
    deleteAgent,
    requestAgentSwitch,
  } = useAgentBuilderStore();

  const [searchQuery, setSearchQuery] = useState('');
  const [isCreating, setIsCreating] = useState(false);
  const [isSelectMode, setIsSelectMode] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [isDeleting, setIsDeleting] = useState(false);

  // Fetch backend agents on mount with pagination
  useEffect(() => {
    fetchAgentsPage(0);
  }, [fetchAgentsPage]);

  // Map backend agents to display format
  const allAgents = useMemo(() => {
    return backendAgents
      .map(a => ({
        id: a.id,
        name: a.name,
        description: a.description || '',
        status: a.status,
        updatedAt: new Date(a.updated_at),
        hasWorkflow: a.has_workflow,
        blockCount: a.block_count,
      }))
      .sort((a, b) => b.updatedAt.getTime() - a.updatedAt.getTime());
  }, [backendAgents]);

  // Filter by search
  const filteredAgents = useMemo(() => {
    if (!searchQuery) return allAgents;
    const query = searchQuery.toLowerCase();
    return allAgents.filter(a => a.name.toLowerCase().includes(query));
  }, [allAgents, searchQuery]);

  const handleNewAgent = async () => {
    if (isCreating) return;
    setIsCreating(true);
    try {
      const agent = await createAgent('New Agent', 'Describe what this agent does...');
      if (agent) {
        trackAgentAccess(agent.id);
        setActiveView('canvas');
        navigate(`/agents/builder/${agent.id}`);
      }
    } finally {
      setIsCreating(false);
    }
  };

  const handleSelectAgent = useCallback(
    async (agentId: string) => {
      const doSwitch = async () => {
        trackAgentAccess(agentId);
        await loadAgentFromBackend(agentId);
        setActiveView('canvas');
        navigate(`/agents/builder/${agentId}`);
      };

      // Guard: check for unsaved changes before switching
      const canProceed = requestAgentSwitch(agentId, () => {
        doSwitch();
      });
      if (!canProceed) return;
      await doSwitch();
    },
    [trackAgentAccess, setActiveView, loadAgentFromBackend, navigate, requestAgentSwitch]
  );

  const handleLoadMore = () => {
    if (!pagination.isLoading && pagination.hasMore) {
      fetchAgentsPage(backendAgents.length);
    }
  };

  const toggleSelectMode = () => {
    setIsSelectMode(prev => !prev);
    setSelectedIds(new Set());
  };

  const toggleAgentSelection = (agentId: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev);
      if (next.has(agentId)) {
        next.delete(agentId);
      } else {
        next.add(agentId);
      }
      return next;
    });
  };

  const selectAllAgents = () => {
    if (selectedIds.size === filteredAgents.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(filteredAgents.map(a => a.id)));
    }
  };

  const handleBulkDelete = async () => {
    if (selectedIds.size === 0 || isDeleting) return;
    setIsDeleting(true);
    try {
      // Delete all selected agents
      const deletePromises = Array.from(selectedIds).map(async id => {
        await deleteAgentAPI(id);
        deleteAgent(id);
      });
      await Promise.all(deletePromises);
      setSelectedIds(new Set());
      setIsSelectMode(false);
    } catch (error) {
      console.error('Failed to delete agents:', error);
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <div className={`flex-1 flex flex-col bg-[var(--color-bg-primary)] ${className || ''}`}>
      {/* Header */}
      <header className="flex items-center justify-between px-8 py-6 border-b border-[var(--color-border)]">
        <h1 className="text-2xl font-semibold text-[var(--color-text-primary)]">My Agents</h1>
        <button
          onClick={handleNewAgent}
          disabled={isCreating}
          className="flex items-center gap-2 px-4 py-2 rounded-lg bg-[var(--color-surface)] border border-[var(--color-border)] text-[var(--color-text-primary)] hover:bg-[var(--color-surface-hover)] transition-colors disabled:opacity-50"
        >
          {isCreating ? <Loader2 size={18} className="animate-spin" /> : <Plus size={18} />}
          <span className="text-sm font-medium">{isCreating ? 'Creating...' : 'New agent'}</span>
        </button>
      </header>

      {/* Search */}
      <div className="px-8 py-4">
        <div className="relative max-w-xl">
          <Search
            size={18}
            className="absolute left-4 top-1/2 -translate-y-1/2 text-[var(--color-text-tertiary)]"
          />
          <input
            type="text"
            placeholder="Search your agents..."
            value={searchQuery}
            onChange={e => setSearchQuery(e.target.value)}
            className="w-full pl-11 pr-4 py-3 rounded-xl bg-[var(--color-surface)] border border-[var(--color-border)] text-[var(--color-text-primary)] placeholder:text-[var(--color-text-tertiary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:ring-opacity-50 focus:border-transparent text-sm transition-all"
          />
        </div>
      </div>

      {/* Agent count and Select / Selection Actions */}
      <div className="flex items-center justify-between px-8 py-2">
        {isSelectMode ? (
          <>
            <div className="flex items-center gap-3">
              <button
                onClick={selectAllAgents}
                className="flex items-center gap-1.5 text-sm text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors"
              >
                {selectedIds.size === filteredAgents.length && filteredAgents.length > 0 ? (
                  <CheckSquare size={16} className="text-[var(--color-accent)]" />
                ) : (
                  <Square size={16} />
                )}
                <span>Select all</span>
              </button>
              <span className="text-sm text-[var(--color-accent)]">
                {selectedIds.size} selected
              </span>
            </div>
            <div className="flex items-center gap-2">
              {selectedIds.size > 0 && (
                <button
                  onClick={handleBulkDelete}
                  disabled={isDeleting}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm text-red-400 bg-red-500/10 hover:bg-red-500/20 transition-colors disabled:opacity-50"
                >
                  {isDeleting ? (
                    <Loader2 size={14} className="animate-spin" />
                  ) : (
                    <Trash2 size={14} />
                  )}
                  <span>{isDeleting ? 'Deleting...' : 'Delete'}</span>
                </button>
              )}
              <button
                onClick={toggleSelectMode}
                className="flex items-center gap-1.5 text-sm text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors"
              >
                <X size={16} />
                <span>Cancel</span>
              </button>
            </div>
          </>
        ) : (
          <>
            <span className="text-sm text-[var(--color-accent)]">
              {searchQuery ? (
                // When searching, show filtered count
                <>
                  {filteredAgents.length} {filteredAgents.length === 1 ? 'result' : 'results'}
                </>
              ) : (
                // When not searching, show loaded count / total
                <>
                  {pagination.total > 0 ? (
                    <>
                      {backendAgents.length} of {pagination.total} agents
                    </>
                  ) : (
                    <>
                      {filteredAgents.length} {filteredAgents.length === 1 ? 'agent' : 'agents'}
                    </>
                  )}
                </>
              )}
              {(isLoadingAgents || pagination.isLoading) && (
                <Loader2
                  size={14}
                  className="inline-block ml-2 animate-spin text-[var(--color-text-tertiary)]"
                />
              )}
            </span>
            <button
              onClick={toggleSelectMode}
              className="text-sm text-[var(--color-accent)] hover:underline"
              disabled={filteredAgents.length === 0}
            >
              Select
            </button>
          </>
        )}
      </div>

      {/* Agent List */}
      <div className="flex-1 overflow-y-auto px-8">
        {filteredAgents.length === 0 && !isLoadingAgents ? (
          <div className="flex flex-col items-center justify-center h-full text-center py-16">
            <div className="w-16 h-16 rounded-2xl bg-[var(--color-surface)] flex items-center justify-center mb-4">
              <Plus size={24} className="text-[var(--color-text-tertiary)]" />
            </div>
            <h3 className="text-lg font-medium text-[var(--color-text-primary)] mb-2">
              No agents yet
            </h3>
            <p className="text-sm text-[var(--color-text-tertiary)] mb-6 max-w-sm">
              Create your first agent to automate tasks with AI-powered workflows
            </p>
            <button
              onClick={handleNewAgent}
              disabled={isCreating}
              className="flex items-center gap-2 px-5 py-2.5 rounded-lg bg-[var(--color-surface)] border border-[var(--color-border)] text-[var(--color-text-primary)] hover:bg-[var(--color-surface-hover)] transition-colors disabled:opacity-50"
            >
              {isCreating ? <Loader2 size={18} className="animate-spin" /> : <Plus size={18} />}
              <span className="text-sm font-medium">
                {isCreating ? 'Creating...' : 'Create an agent'}
              </span>
            </button>
          </div>
        ) : (
          <div className="space-y-1 py-2">
            {filteredAgents.map(agent => (
              <div
                key={agent.id}
                onClick={() =>
                  isSelectMode ? toggleAgentSelection(agent.id) : handleSelectAgent(agent.id)
                }
                role="button"
                tabIndex={0}
                onKeyDown={e => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    if (isSelectMode) {
                      toggleAgentSelection(agent.id);
                    } else {
                      handleSelectAgent(agent.id);
                    }
                  }
                }}
                className={cn(
                  'w-full text-left px-4 py-3 rounded-lg transition-colors group cursor-pointer flex items-center gap-3',
                  selectedIds.has(agent.id)
                    ? 'bg-[var(--color-accent)]/10 hover:bg-[var(--color-accent)]/15'
                    : 'hover:bg-[var(--color-surface-hover)]'
                )}
              >
                {isSelectMode && (
                  <div className="flex-shrink-0">
                    {selectedIds.has(agent.id) ? (
                      <CheckSquare size={18} className="text-[var(--color-accent)]" />
                    ) : (
                      <Square size={18} className="text-[var(--color-text-tertiary)]" />
                    )}
                  </div>
                )}
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span
                      className={cn(
                        'font-medium transition-colors truncate',
                        selectedIds.has(agent.id)
                          ? 'text-[var(--color-accent)]'
                          : 'text-[var(--color-text-primary)] group-hover:text-[var(--color-accent)]'
                      )}
                    >
                      {agent.name}
                    </span>
                    <Cloud
                      size={14}
                      className="text-green-500 flex-shrink-0"
                      title="Saved to cloud"
                    />
                  </div>
                  <div className="flex items-center gap-2 mt-0.5">
                    <span className="text-sm text-[var(--color-text-tertiary)]">
                      Last modified {formatDistanceToNow(agent.updatedAt)}
                    </span>
                    {agent.hasWorkflow && agent.blockCount > 1 && (
                      <span className="text-xs text-[var(--color-text-tertiary)] bg-[var(--color-surface)] px-1.5 py-0.5 rounded">
                        {agent.blockCount} blocks
                      </span>
                    )}
                  </div>
                </div>
              </div>
            ))}

            {/* Load more button */}
            {pagination.hasMore && !searchQuery && (
              <div className="py-4 text-center border-t border-[var(--color-border)] mt-2">
                <button
                  onClick={handleLoadMore}
                  disabled={pagination.isLoading}
                  className="inline-flex items-center gap-2 px-4 py-2 text-sm text-[var(--color-accent)] bg-[var(--color-accent)]/10 hover:bg-[var(--color-accent)]/20 rounded-lg transition-colors disabled:opacity-50"
                >
                  {pagination.isLoading ? (
                    <>
                      <Loader2 size={14} className="animate-spin" />
                      <span>Loading...</span>
                    </>
                  ) : (
                    <>
                      <span>Load more</span>
                      <span className="text-[var(--color-text-tertiary)]">
                        ({pagination.total - backendAgents.length} remaining)
                      </span>
                    </>
                  )}
                </button>
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
