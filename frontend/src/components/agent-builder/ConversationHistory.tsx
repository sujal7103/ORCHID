import { useEffect } from 'react';
import { MessageSquare, Plus, Trash2, Clock, MessageCircle } from 'lucide-react';
import { cn } from '@/lib/utils';
import { useAgentBuilderStore } from '@/store/useAgentBuilderStore';
import {
  deleteBuilderConversation,
  createBuilderConversation,
} from '@/services/conversationService';

interface ConversationHistoryProps {
  className?: string;
  onNewConversation?: () => void;
}

export function ConversationHistory({ className, onNewConversation }: ConversationHistoryProps) {
  const {
    selectedAgentId,
    currentConversationId,
    conversationList,
    isLoadingConversation,
    fetchConversationList,
    selectConversation,
    clearConversation,
    loadConversation,
  } = useAgentBuilderStore();

  // Fetch conversation list when agent changes
  useEffect(() => {
    if (selectedAgentId) {
      fetchConversationList(selectedAgentId);
    }
  }, [selectedAgentId, fetchConversationList]);

  const handleNewConversation = async () => {
    if (!selectedAgentId) return;

    try {
      // Create a new conversation on the backend
      const newConv = await createBuilderConversation(selectedAgentId, '');
      if (newConv) {
        // Clear current conversation and load the new one
        clearConversation();
        await loadConversation(selectedAgentId);
        // Refresh the list
        await fetchConversationList(selectedAgentId);
        onNewConversation?.();
      }
    } catch (error) {
      console.error('Failed to create new conversation:', error);
    }
  };

  const handleSelectConversation = async (conversationId: string) => {
    if (conversationId === currentConversationId) return;
    await selectConversation(conversationId);
  };

  const handleDeleteConversation = async (conversationId: string, e: React.MouseEvent) => {
    e.stopPropagation();
    if (!selectedAgentId) return;

    if (confirm('Delete this conversation? This cannot be undone.')) {
      try {
        await deleteBuilderConversation(selectedAgentId, conversationId);
        // Refresh the list
        await fetchConversationList(selectedAgentId);
        // If we deleted the current conversation, clear it
        if (conversationId === currentConversationId) {
          clearConversation();
          // Load the most recent conversation if available
          await loadConversation(selectedAgentId);
        }
      } catch (error) {
        console.error('Failed to delete conversation:', error);
      }
    }
  };

  const formatDate = (dateString: string) => {
    const date = new Date(dateString);
    const now = new Date();
    const diffMs = now.getTime() - date.getTime();
    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMs / 3600000);
    const diffDays = Math.floor(diffMs / 86400000);

    if (diffMins < 1) return 'Just now';
    if (diffMins < 60) return `${diffMins}m ago`;
    if (diffHours < 24) return `${diffHours}h ago`;
    if (diffDays < 7) return `${diffDays}d ago`;

    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric' });
  };

  if (!selectedAgentId) {
    return null;
  }

  return (
    <div
      className={cn(
        'flex flex-col h-full bg-[var(--color-bg-secondary)] border-l border-[var(--color-border)]',
        className
      )}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-[var(--color-border)]">
        <div className="flex items-center gap-2 text-sm font-medium text-[var(--color-text-primary)]">
          <MessageSquare size={16} className="text-[var(--color-accent)]" />
          <span>History</span>
        </div>
        <button
          onClick={handleNewConversation}
          className="p-1.5 rounded-lg hover:bg-[var(--color-bg-tertiary)] text-[var(--color-text-secondary)] hover:text-[var(--color-accent)] transition-colors"
          title="New conversation"
        >
          <Plus size={16} />
        </button>
      </div>

      {/* Conversation List */}
      <div className="flex-1 overflow-y-auto py-2">
        {conversationList.length === 0 ? (
          <div className="px-4 py-8 text-center">
            <MessageCircle
              size={32}
              className="mx-auto mb-2 text-[var(--color-text-tertiary)] opacity-50"
            />
            <p className="text-xs text-[var(--color-text-tertiary)]">No conversations yet</p>
            <p className="text-[10px] text-[var(--color-text-tertiary)] mt-1 opacity-75">
              Start chatting to create one
            </p>
          </div>
        ) : (
          <div className="space-y-1 px-2">
            {conversationList.map(conv => (
              <button
                key={conv.id}
                onClick={() => handleSelectConversation(conv.id)}
                disabled={isLoadingConversation}
                className={cn(
                  'w-full px-3 py-2.5 rounded-xl text-left transition-all group',
                  'hover:bg-[var(--color-bg-tertiary)]',
                  conv.id === currentConversationId
                    ? 'bg-[var(--color-accent)]/10 border border-[var(--color-accent)]/30'
                    : 'border border-transparent',
                  isLoadingConversation && 'opacity-50 cursor-not-allowed'
                )}
              >
                <div className="flex items-start justify-between">
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <Clock
                        size={12}
                        className="flex-shrink-0 text-[var(--color-text-tertiary)]"
                      />
                      <span className="text-xs text-[var(--color-text-secondary)]">
                        {formatDate(conv.updated_at)}
                      </span>
                    </div>
                    <div className="mt-1 flex items-center gap-1.5">
                      <MessageSquare size={10} className="text-[var(--color-text-tertiary)]" />
                      <span className="text-[10px] text-[var(--color-text-tertiary)]">
                        {conv.message_count} message{conv.message_count !== 1 ? 's' : ''}
                      </span>
                    </div>
                  </div>
                  <button
                    onClick={e => handleDeleteConversation(conv.id, e)}
                    className={cn(
                      'p-1 rounded-md opacity-0 group-hover:opacity-100 transition-opacity',
                      'hover:bg-red-500/20 text-[var(--color-text-tertiary)] hover:text-red-400'
                    )}
                    title="Delete conversation"
                  >
                    <Trash2 size={12} />
                  </button>
                </div>
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Footer info */}
      <div className="px-4 py-2 border-t border-[var(--color-border)]">
        <p className="text-[10px] text-[var(--color-text-tertiary)] text-center">
          Conversations are encrypted
        </p>
      </div>
    </div>
  );
}
