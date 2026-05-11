import { useState } from 'react';
import {
  X,
  CheckCircle,
  XCircle,
  Clock,
  Loader2,
  ArrowDownToLine,
  ArrowUpFromLine,
  AlertTriangle,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { formatDuration } from '@/services/executionService';
import type { BlockState } from '@/services/executionService';

interface ExecutionBlockInspectorProps {
  blockId: string;
  blockName: string;
  blockType: string;
  blockState: BlockState | null;
  onClose: () => void;
  className?: string;
}

type InspectorTab = 'output' | 'input';

export function ExecutionBlockInspector({
  blockName,
  blockType,
  blockState,
  onClose,
  className,
}: ExecutionBlockInspectorProps) {
  const [activeTab, setActiveTab] = useState<InspectorTab>('output');

  const getStatusIcon = (status?: string) => {
    switch (status) {
      case 'completed':
        return <CheckCircle size={14} className="text-green-400" />;
      case 'failed':
        return <XCircle size={14} className="text-red-400" />;
      case 'running':
        return <Loader2 size={14} className="text-[var(--color-accent)] animate-spin" />;
      case 'pending':
        return <Clock size={14} className="text-yellow-400" />;
      case 'skipped':
        return <Clock size={14} className="text-[var(--color-text-tertiary)]" />;
      default:
        return <Clock size={14} className="text-[var(--color-text-tertiary)]" />;
    }
  };

  const getStatusColor = (status?: string) => {
    switch (status) {
      case 'completed':
        return 'bg-green-500/15 text-green-400';
      case 'failed':
        return 'bg-red-500/15 text-red-400';
      case 'running':
        return 'bg-[var(--color-accent)]/15 text-[var(--color-accent)]';
      case 'pending':
        return 'bg-yellow-500/15 text-yellow-400';
      case 'skipped':
        return 'bg-gray-500/15 text-gray-400';
      default:
        return 'bg-gray-500/15 text-gray-400';
    }
  };

  // Format timing info
  const formatTiming = () => {
    if (!blockState?.startedAt) return null;
    const start = new Date(blockState.startedAt);
    const parts: string[] = [
      start.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' }),
    ];
    if (blockState.completedAt) {
      const end = new Date(blockState.completedAt);
      parts.push(
        end.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
      );
    }
    return parts;
  };

  const timing = formatTiming();

  return (
    <div className={cn('flex flex-col h-full bg-[var(--color-bg-secondary)]', className)}>
      {/* Header */}
      <div className="flex-shrink-0 px-4 py-3 border-b border-[var(--color-border)]">
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2 min-w-0">
            {getStatusIcon(blockState?.status)}
            <h3 className="text-sm font-semibold text-[var(--color-text-primary)] truncate">
              {blockName}
            </h3>
          </div>
          <button
            onClick={onClose}
            className="p-1 rounded-md hover:bg-[var(--color-surface-hover)] text-[var(--color-text-tertiary)] transition-colors flex-shrink-0"
          >
            <X size={16} />
          </button>
        </div>

        {/* Status + Type + Duration */}
        <div className="flex items-center gap-2 flex-wrap">
          <span
            className={cn(
              'px-2 py-0.5 rounded text-[10px] font-medium capitalize',
              getStatusColor(blockState?.status)
            )}
          >
            {blockState?.status || 'unknown'}
          </span>
          <span className="px-2 py-0.5 rounded text-[10px] font-medium bg-[var(--color-surface)] text-[var(--color-text-tertiary)]">
            {blockType}
          </span>
          {blockState?.durationMs !== undefined && (
            <span className="text-[10px] text-[var(--color-text-tertiary)]">
              {formatDuration(blockState.durationMs)}
            </span>
          )}
        </div>

        {/* Timing */}
        {timing && (
          <div className="flex items-center gap-2 mt-1.5 text-[10px] text-[var(--color-text-tertiary)]">
            <span>{timing[0]}</span>
            {timing[1] && (
              <>
                <span>&rarr;</span>
                <span>{timing[1]}</span>
              </>
            )}
          </div>
        )}
      </div>

      {/* Tabs */}
      <div className="flex-shrink-0 flex border-b border-[var(--color-border)]">
        <button
          onClick={() => setActiveTab('output')}
          className={cn(
            'flex items-center gap-1.5 px-4 py-2.5 text-xs font-medium transition-colors relative',
            activeTab === 'output'
              ? 'text-[var(--color-accent)]'
              : 'text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]'
          )}
        >
          <ArrowDownToLine size={12} />
          Output
          {activeTab === 'output' && (
            <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-[var(--color-accent)]" />
          )}
        </button>
        <button
          onClick={() => setActiveTab('input')}
          className={cn(
            'flex items-center gap-1.5 px-4 py-2.5 text-xs font-medium transition-colors relative',
            activeTab === 'input'
              ? 'text-[var(--color-accent)]'
              : 'text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]'
          )}
        >
          <ArrowUpFromLine size={12} />
          Input
          {activeTab === 'input' && (
            <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-[var(--color-accent)]" />
          )}
        </button>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4">
        {!blockState ? (
          <div className="flex flex-col items-center justify-center py-8 text-center">
            <Clock size={24} className="text-[var(--color-text-tertiary)] mb-2 opacity-40" />
            <p className="text-xs text-[var(--color-text-secondary)]">
              No execution data for this block
            </p>
            <p className="text-[10px] text-[var(--color-text-tertiary)] mt-1">
              This block may not have been reached during execution
            </p>
          </div>
        ) : activeTab === 'output' ? (
          <div className="space-y-3">
            {/* Error Banner */}
            {blockState.error && (
              <div className="p-3 rounded-lg bg-red-500/10 border border-red-500/20">
                <div className="flex items-start gap-2">
                  <AlertTriangle size={14} className="text-red-400 mt-0.5 flex-shrink-0" />
                  <div>
                    <p className="text-xs font-medium text-red-400 mb-1">Error</p>
                    <p className="text-xs text-red-300/80 font-mono whitespace-pre-wrap break-all">
                      {blockState.error}
                    </p>
                  </div>
                </div>
              </div>
            )}

            {/* Outputs */}
            {blockState.outputs && Object.keys(blockState.outputs).length > 0 ? (
              <div>
                <div className="flex items-center gap-1.5 text-[10px] font-medium text-[var(--color-text-tertiary)] mb-2">
                  <ArrowDownToLine size={11} />
                  Output Data
                </div>
                <pre className="p-3 rounded-lg bg-[var(--color-bg-primary)] border border-[var(--color-border)] text-[11px] text-[var(--color-text-secondary)] font-mono overflow-x-auto max-h-[500px] overflow-y-auto whitespace-pre-wrap break-all">
                  {JSON.stringify(blockState.outputs, null, 2)}
                </pre>
              </div>
            ) : !blockState.error ? (
              <div className="text-center py-6 text-xs text-[var(--color-text-tertiary)]">
                No output data recorded
              </div>
            ) : null}
          </div>
        ) : (
          <div className="space-y-3">
            {/* Inputs */}
            {blockState.inputs && Object.keys(blockState.inputs).length > 0 ? (
              <div>
                <div className="flex items-center gap-1.5 text-[10px] font-medium text-[var(--color-text-tertiary)] mb-2">
                  <ArrowUpFromLine size={11} />
                  Input Data
                </div>
                <pre className="p-3 rounded-lg bg-[var(--color-bg-primary)] border border-[var(--color-border)] text-[11px] text-[var(--color-text-secondary)] font-mono overflow-x-auto max-h-[500px] overflow-y-auto whitespace-pre-wrap break-all">
                  {JSON.stringify(blockState.inputs, null, 2)}
                </pre>
              </div>
            ) : (
              <div className="text-center py-6 text-xs text-[var(--color-text-tertiary)]">
                No input data recorded
              </div>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
