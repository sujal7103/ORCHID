import { useState } from 'react';
import {
  ChevronDown,
  ChevronRight,
  Copy,
  Check,
  Loader2,
  CheckCircle2,
  XCircle,
  Circle,
  AlertCircle,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { useAgentBuilderStore } from '@/store/useAgentBuilderStore';
import { MarkdownRenderer } from '@/components/design-system';
import type { ExecutionStatus } from '@/types/agent';

/**
 * Mobile-optimized execution output view.
 * Shows execution status and block outputs in a full-height scrollable layout.
 */
export function MobileExecutionOutput() {
  const { blockStates, workflow, executionStatus, lastExecutionResult } = useAgentBuilderStore();
  const [expandedBlocks, setExpandedBlocks] = useState<Set<string>>(new Set());
  const [copiedId, setCopiedId] = useState<string | null>(null);

  const toggleBlock = (blockId: string) => {
    setExpandedBlocks(prev => {
      const next = new Set(prev);
      if (next.has(blockId)) {
        next.delete(blockId);
      } else {
        next.add(blockId);
      }
      return next;
    });
  };

  const copyToClipboard = async (text: string, id: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedId(id);
      setTimeout(() => setCopiedId(null), 2000);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  };

  // Extract readable content from output
  const extractReadableContent = (output: unknown): string => {
    if (!output) return 'No output';

    // Helper to safely stringify a value
    const stringify = (val: unknown): string => {
      if (typeof val === 'string') return val;
      if (typeof val === 'object' && val !== null) return JSON.stringify(val, null, 2);
      return String(val);
    };

    const data = typeof output === 'object' && output !== null ? output : { value: output };
    const obj = data as Record<string, unknown>;

    // Look for common output fields
    if (obj.output && typeof obj.output === 'object') {
      const innerOutput = obj.output as Record<string, unknown>;
      if (innerOutput.summary) return stringify(innerOutput.summary);
      if (innerOutput.result) return stringify(innerOutput.result);
      if (innerOutput.content) return stringify(innerOutput.content);
      if (innerOutput.text) return stringify(innerOutput.text);
      if (innerOutput.response) return stringify(innerOutput.response);
    }

    // Direct fields - response may now be an object (from schema formatting)
    if (obj.response !== undefined) return stringify(obj.response);
    if (obj.summary) return stringify(obj.summary);
    if (obj.result) return stringify(obj.result);
    if (obj.content) return stringify(obj.content);
    if (obj.rawResponse) return stringify(obj.rawResponse);
    if (obj.text) return stringify(obj.text);

    if (typeof output === 'string') return output;

    return JSON.stringify(output, null, 2);
  };

  const completedCount = Object.values(blockStates).filter(s => s.status === 'completed').length;
  const failedCount = Object.values(blockStates).filter(s => s.status === 'failed').length;
  const runningCount = Object.values(blockStates).filter(s => s.status === 'running').length;
  const totalBlocks = workflow?.blocks?.length || 0;

  // No execution yet
  if (!executionStatus && Object.keys(blockStates).length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center p-6 text-center">
        <div className="space-y-3">
          <div className="w-16 h-16 mx-auto rounded-full bg-[var(--color-surface)] flex items-center justify-center">
            <Circle size={32} className="text-[var(--color-text-tertiary)]" />
          </div>
          <p className="text-[var(--color-text-secondary)]">No execution yet</p>
          <p className="text-sm text-[var(--color-text-tertiary)]">
            Run the agent to see output here
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      {/* Status header */}
      <div className="flex-shrink-0 px-4 py-3 border-b border-[var(--color-border)] bg-[var(--color-bg-secondary)]">
        <ExecutionStatusHeader status={executionStatus} />
        <div className="flex items-center gap-4 mt-2 text-xs text-[var(--color-text-tertiary)]">
          {runningCount > 0 && (
            <span className="flex items-center gap-1">
              <Loader2 size={12} className="animate-spin text-blue-400" />
              {runningCount} running
            </span>
          )}
          <span className="flex items-center gap-1">
            <CheckCircle2 size={12} className="text-green-400" />
            {completedCount}/{totalBlocks} completed
          </span>
          {failedCount > 0 && (
            <span className="flex items-center gap-1">
              <XCircle size={12} className="text-red-400" />
              {failedCount} failed
            </span>
          )}
        </div>
      </div>

      {/* Final result section */}
      {lastExecutionResult?.result && (
        <div className="flex-shrink-0 p-4 border-b border-[var(--color-border)]">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-semibold text-[var(--color-text-primary)]">
              Final Output
            </span>
            <button
              onClick={() => copyToClipboard(lastExecutionResult.result!, 'final')}
              className="p-1.5 rounded-md hover:bg-[var(--color-surface-hover)] text-[var(--color-text-tertiary)]"
            >
              {copiedId === 'final' ? <Check size={14} /> : <Copy size={14} />}
            </button>
          </div>
          <div className="p-3 rounded-lg bg-[var(--color-surface)] text-sm text-[var(--color-text-primary)] max-h-48 overflow-y-auto">
            <MarkdownRenderer
              content={lastExecutionResult.result}
              className="[&_p]:mb-2 [&_p:last-child]:mb-0 [&_pre]:bg-black/20 [&_code]:text-xs"
            />
          </div>
        </div>
      )}

      {/* Block outputs */}
      <div className="flex-1 overflow-y-auto">
        <div className="p-4 space-y-2">
          <h3 className="text-xs font-semibold text-[var(--color-text-tertiary)] uppercase tracking-wide mb-3">
            Block Outputs
          </h3>

          {workflow?.blocks?.map(block => {
            const state = blockStates[block.id];
            const isExpanded = expandedBlocks.has(block.id);
            const hasOutput = state?.outputs && Object.keys(state.outputs).length > 0;
            const outputText = hasOutput ? extractReadableContent(state.outputs) : null;

            return (
              <div
                key={block.id}
                className="rounded-xl border border-[var(--color-border)] bg-[var(--color-surface)] overflow-hidden"
              >
                {/* Block header */}
                <button
                  onClick={() => hasOutput && toggleBlock(block.id)}
                  disabled={!hasOutput}
                  className={cn(
                    'w-full flex items-center gap-3 p-3 text-left',
                    hasOutput && 'hover:bg-[var(--color-surface-hover)]'
                  )}
                >
                  {/* Expand icon */}
                  <div className="flex-shrink-0 w-5 h-5 flex items-center justify-center">
                    {hasOutput ? (
                      isExpanded ? (
                        <ChevronDown size={16} className="text-[var(--color-text-tertiary)]" />
                      ) : (
                        <ChevronRight size={16} className="text-[var(--color-text-tertiary)]" />
                      )
                    ) : null}
                  </div>

                  {/* Block info */}
                  <div className="flex-1 min-w-0">
                    <span className="text-sm font-medium text-[var(--color-text-primary)] truncate block">
                      {block.name}
                    </span>
                  </div>

                  {/* Status */}
                  <BlockStatusIcon status={state?.status} />
                </button>

                {/* Expanded output */}
                {isExpanded && outputText && (
                  <div className="px-3 pb-3">
                    <div className="relative">
                      <button
                        onClick={() => copyToClipboard(outputText, block.id)}
                        className="absolute top-2 right-2 p-1.5 rounded-md bg-[var(--color-bg-secondary)] hover:bg-[var(--color-surface-hover)] text-[var(--color-text-tertiary)]"
                      >
                        {copiedId === block.id ? <Check size={12} /> : <Copy size={12} />}
                      </button>
                      <div className="p-3 rounded-lg bg-[var(--color-bg-secondary)] text-xs text-[var(--color-text-secondary)] whitespace-pre-wrap break-words max-h-64 overflow-y-auto font-mono">
                        {outputText}
                      </div>
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

/**
 * Execution status header with icon
 */
function ExecutionStatusHeader({ status }: { status: ExecutionStatus | null }) {
  const getStatusInfo = () => {
    switch (status) {
      case 'running':
        return {
          icon: <Loader2 size={18} className="animate-spin" />,
          text: 'Running...',
          color: 'text-blue-400',
          bg: 'bg-blue-500/10',
        };
      case 'completed':
        return {
          icon: <CheckCircle2 size={18} />,
          text: 'Completed',
          color: 'text-green-400',
          bg: 'bg-green-500/10',
        };
      case 'failed':
        return {
          icon: <XCircle size={18} />,
          text: 'Failed',
          color: 'text-red-400',
          bg: 'bg-red-500/10',
        };
      case 'partial_failure':
        return {
          icon: <AlertCircle size={18} />,
          text: 'Partial Failure',
          color: 'text-yellow-400',
          bg: 'bg-yellow-500/10',
        };
      default:
        return {
          icon: <Circle size={18} />,
          text: 'Pending',
          color: 'text-[var(--color-text-tertiary)]',
          bg: 'bg-[var(--color-surface)]',
        };
    }
  };

  const info = getStatusInfo();

  return (
    <div
      className={cn('inline-flex items-center gap-2 px-3 py-1.5 rounded-full', info.bg, info.color)}
    >
      {info.icon}
      <span className="text-sm font-medium">{info.text}</span>
    </div>
  );
}

/**
 * Small status icon for block row
 */
function BlockStatusIcon({ status }: { status?: string }) {
  switch (status) {
    case 'running':
      return <Loader2 size={16} className="text-blue-400 animate-spin" />;
    case 'completed':
      return <CheckCircle2 size={16} className="text-green-400" />;
    case 'failed':
      return <XCircle size={16} className="text-red-400" />;
    case 'skipped':
      return <Circle size={16} className="text-yellow-400" />;
    default:
      return <Circle size={16} className="text-[var(--color-text-tertiary)]" />;
  }
}
