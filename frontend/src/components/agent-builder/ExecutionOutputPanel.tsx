import { useState } from 'react';
import {
  X,
  ChevronDown,
  ChevronRight,
  ChevronLeft,
  Copy,
  Check,
  FileJson,
  Maximize2,
  Minimize2,
  Repeat,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import { useAgentBuilderStore } from '@/store/useAgentBuilderStore';
import type { ForEachIterationState } from '@/types/agent';

// Compact iteration output view for for-each blocks
function ForEachOutputView({ state }: { state: ForEachIterationState }) {
  const [selectedIdx, setSelectedIdx] = useState(0);

  const { iterations, totalItems } = state;
  const selected = iterations[selectedIdx];
  const useCompact = totalItems <= 20;

  return (
    <div className="space-y-2">
      {/* Selector */}
      <div className="flex items-center gap-1">
        <button
          onClick={() => setSelectedIdx(Math.max(0, selectedIdx - 1))}
          disabled={selectedIdx === 0}
          className="p-0.5 rounded hover:bg-white/5 text-[var(--color-text-tertiary)] disabled:opacity-30"
        >
          <ChevronLeft size={12} />
        </button>

        {useCompact ? (
          <div className="flex flex-wrap gap-0.5 flex-1">
            {iterations.map((iter, idx) => (
              <button
                key={idx}
                onClick={() => setSelectedIdx(idx)}
                className={cn(
                  'w-5 h-5 rounded text-[9px] font-medium transition-colors',
                  idx === selectedIdx
                    ? 'bg-[var(--color-accent)] text-white'
                    : iter?.status === 'failed'
                      ? 'bg-red-500/15 text-red-400'
                      : 'bg-white/5 text-[var(--color-text-tertiary)]'
                )}
              >
                {idx + 1}
              </button>
            ))}
          </div>
        ) : (
          <div className="flex items-center gap-1 flex-1">
            <input
              type="number"
              min={1}
              max={totalItems}
              value={selectedIdx + 1}
              onChange={e => {
                const val = parseInt(e.target.value);
                if (val >= 1 && val <= totalItems) setSelectedIdx(val - 1);
              }}
              className="w-14 px-1.5 py-0.5 rounded text-[10px] text-center bg-white/5 border border-[var(--color-border)] text-[var(--color-text-primary)] focus:outline-none"
            />
            <span className="text-[10px] text-[var(--color-text-tertiary)]">of {totalItems}</span>
          </div>
        )}

        <button
          onClick={() => setSelectedIdx(Math.min(totalItems - 1, selectedIdx + 1))}
          disabled={selectedIdx >= totalItems - 1}
          className="p-0.5 rounded hover:bg-white/5 text-[var(--color-text-tertiary)] disabled:opacity-30"
        >
          <ChevronRight size={12} />
        </button>
      </div>

      {/* Selected Output */}
      {selected?.output ? (
        <pre className="text-xs text-[var(--color-text-secondary)] whitespace-pre-wrap break-words font-mono max-h-[120px] overflow-y-auto bg-[var(--color-surface)] rounded-lg p-2 border border-[var(--color-border)]">
          {JSON.stringify(selected.output, null, 2)}
        </pre>
      ) : selected?.error ? (
        <div className="text-xs text-red-400 font-mono p-2 rounded-lg bg-red-500/10 border border-red-500/20">
          Error: {selected.error}
        </div>
      ) : (
        <p className="text-xs text-[var(--color-text-tertiary)] italic">No data</p>
      )}
    </div>
  );
}

interface ExecutionOutputPanelProps {
  onClose: () => void;
}

export function ExecutionOutputPanel({ onClose }: ExecutionOutputPanelProps) {
  const {
    blockStates,
    blockOutputCache,
    workflow,
    executionStatus,
    lastExecutionResult,
    forEachStates,
  } = useAgentBuilderStore();
  const [expandedBlocks, setExpandedBlocks] = useState<Set<string>>(new Set());
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [isExpanded, setIsExpanded] = useState(false);

  // Use the clean API response result if available
  const getCleanResult = () => {
    if (lastExecutionResult?.result) {
      return lastExecutionResult.result;
    }
    return null;
  };

  // Get the last block's output as final output (fallback)
  const getLastBlockOutput = () => {
    if (!workflow?.connections || !workflow?.blocks) return null;

    // Find blocks that are not sources (i.e., terminal blocks)
    const sourceBlockIds = new Set(workflow.connections.map(c => c.sourceBlockId));
    const terminalBlocks = workflow.blocks.filter(b => !sourceBlockIds.has(b.id));

    // Get output from the first terminal block that has completed
    for (const block of terminalBlocks) {
      const state = blockStates[block.id];
      if (state?.status === 'completed' && state.outputs) {
        return { blockId: block.id, blockName: block.name, output: state.outputs };
      }
    }

    // Fallback: get any block with output
    const completedBlocks = Object.entries(blockStates)
      .filter(([_, state]) => state.status === 'completed' && state.outputs)
      .map(([id, state]) => ({ id, ...state }));

    if (completedBlocks.length > 0) {
      const lastBlock = completedBlocks[completedBlocks.length - 1];
      const block = workflow.blocks.find(b => b.id === lastBlock.id);
      return {
        blockId: lastBlock.id,
        blockName: block?.name || 'Unknown',
        output: lastBlock.outputs,
      };
    }

    return null;
  };

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

  const formatOutput = (output: unknown): string => {
    if (typeof output === 'string') {
      // Try to parse as JSON for better formatting
      try {
        const parsed = JSON.parse(output);
        return JSON.stringify(parsed, null, 2);
      } catch {
        return output;
      }
    }
    return JSON.stringify(output, null, 2);
  };

  // Extract readable content from output
  const extractReadableContent = (output: unknown): string => {
    if (!output) return 'No output';

    // Helper to safely convert value to string
    const stringify = (val: unknown): string => {
      if (typeof val === 'string') return val;
      if (typeof val === 'object' && val !== null) return JSON.stringify(val, null, 2);
      return String(val);
    };

    // Handle nested output structure
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

    // Direct fields - check for response first (common in code_block outputs)
    if (obj.response !== undefined) return stringify(obj.response);
    if (obj.summary) return stringify(obj.summary);
    if (obj.result) return stringify(obj.result);
    if (obj.content) return stringify(obj.content);
    if (obj.rawResponse) return stringify(obj.rawResponse);
    if (obj.text) return stringify(obj.text);

    // If it's a string, return it
    if (typeof output === 'string') return output;

    // Fallback to JSON
    return JSON.stringify(output, null, 2);
  };

  const cleanResult = getCleanResult();
  const lastOutput = getLastBlockOutput();
  const completedBlocksCount =
    lastExecutionResult?.metadata?.blocks_executed ||
    Object.values(blockStates).filter(s => s.status === 'completed').length;
  const failedBlocksCount =
    lastExecutionResult?.metadata?.blocks_failed ||
    Object.values(blockStates).filter(s => s.status === 'failed').length;

  if (!executionStatus || executionStatus === 'running') return null;

  return (
    <div
      className={cn(
        'absolute bg-[var(--color-surface-elevated)] backdrop-blur-xl border border-[var(--color-border)] rounded-2xl shadow-2xl overflow-hidden transition-all duration-300 z-[1000]',
        isExpanded ? 'bottom-4 right-4 left-4 top-16' : 'bottom-4 right-4 w-[420px] max-h-[60vh]'
      )}
      style={{
        animation: 'slideUp 0.2s ease-out',
      }}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-5 py-4 bg-[var(--color-surface)] border-b border-[var(--color-border)]">
        <div className="flex items-center gap-3">
          <div className="p-1.5 rounded-lg bg-[var(--color-accent)]/10">
            <FileJson size={16} className="text-[var(--color-accent)]" />
          </div>
          <div>
            <span className="font-semibold text-sm text-[var(--color-text-primary)]">
              Execution Output
            </span>
            <span className="ml-2 text-xs text-[var(--color-text-tertiary)]">
              {completedBlocksCount} completed
              {failedBlocksCount > 0 ? ` · ${failedBlocksCount} failed` : ''}
            </span>
          </div>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={() => setIsExpanded(!isExpanded)}
            className="p-2 rounded-lg hover:bg-[var(--color-surface-hover)] text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors"
            title={isExpanded ? 'Minimize' : 'Maximize'}
          >
            {isExpanded ? <Minimize2 size={16} /> : <Maximize2 size={16} />}
          </button>
          <button
            onClick={onClose}
            className="p-2 rounded-lg hover:bg-[var(--color-surface-hover)] text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)] transition-colors"
            title="Close"
          >
            <X size={16} />
          </button>
        </div>
      </div>

      {/* Content */}
      <div
        className="overflow-y-auto custom-scrollbar"
        style={{ maxHeight: isExpanded ? 'calc(100% - 64px)' : '50vh' }}
      >
        {/* Final Output Section - Prefer clean API result */}
        {(cleanResult || lastOutput) && (
          <div className="p-5 border-b border-[var(--color-border)]">
            <div className="flex items-center justify-between mb-3">
              <h3 className="text-sm font-semibold text-[var(--color-text-primary)]">
                Final Output
              </h3>
              <button
                onClick={() =>
                  copyToClipboard(cleanResult || formatOutput(lastOutput?.output), 'final')
                }
                className="p-1.5 rounded-lg hover:bg-[var(--color-surface-hover)] text-[var(--color-text-tertiary)] hover:text-[var(--color-text-primary)] transition-colors"
                title="Copy output"
              >
                {copiedId === 'final' ? (
                  <Check size={14} className="text-green-400" />
                ) : (
                  <Copy size={14} />
                )}
              </button>
            </div>
            <div className="bg-[var(--color-background)] rounded-xl p-4 border border-[var(--color-border)]">
              <pre className="text-sm text-[var(--color-text-secondary)] whitespace-pre-wrap break-words font-mono max-h-[200px] overflow-y-auto leading-relaxed">
                {cleanResult || extractReadableContent(lastOutput?.output)}
              </pre>
            </div>

            {/* Show artifacts if available */}
            {lastExecutionResult?.artifacts && lastExecutionResult.artifacts.length > 0 && (
              <div className="mt-4">
                <h4 className="text-xs font-medium text-[var(--color-text-tertiary)] mb-2">
                  Generated Charts ({lastExecutionResult.artifacts.length})
                </h4>
                <div className="flex flex-wrap gap-2">
                  {lastExecutionResult.artifacts.map((artifact, idx) => (
                    <div key={idx} className="relative group">
                      <img
                        src={
                          artifact.data.startsWith('data:')
                            ? artifact.data
                            : `data:image/${artifact.format || 'png'};base64,${artifact.data}`
                        }
                        alt={artifact.title || `Chart ${idx + 1}`}
                        className="h-24 w-auto rounded-lg border border-[var(--color-border)] object-contain bg-white"
                      />
                      <div className="absolute inset-0 bg-black/50 opacity-0 group-hover:opacity-100 transition-opacity rounded-lg flex items-center justify-center">
                        <span className="text-xs text-white">
                          {artifact.title || `Chart ${idx + 1}`}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            )}

            {/* Show files if available */}
            {lastExecutionResult?.files && lastExecutionResult.files.length > 0 && (
              <div className="mt-4">
                <h4 className="text-xs font-medium text-[var(--color-text-tertiary)] mb-2">
                  Generated Files ({lastExecutionResult.files.length})
                </h4>
                <div className="space-y-1">
                  {lastExecutionResult.files.map((file, idx) => (
                    <a
                      key={idx}
                      href={file.download_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="flex items-center gap-2 text-xs text-[var(--color-accent)] hover:underline"
                    >
                      <FileJson size={12} />
                      {file.filename}
                    </a>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {/* Block Outputs */}
        <div className="p-5">
          <h3 className="text-sm font-semibold text-[var(--color-text-primary)] mb-4">
            Block Outputs
          </h3>
          <div className="space-y-2">
            {workflow?.blocks.map(block => {
              const state = blockStates[block.id];
              const output = blockOutputCache[block.id] || state?.outputs;
              const isBlockExpanded = expandedBlocks.has(block.id);

              return (
                <div
                  key={block.id}
                  className="border border-[var(--color-border)] rounded-xl overflow-hidden bg-[var(--color-surface)]"
                >
                  <button
                    onClick={() => toggleBlock(block.id)}
                    className="w-full flex items-center justify-between px-4 py-3 hover:bg-[var(--color-surface-hover)] transition-colors"
                  >
                    <div className="flex items-center gap-2.5">
                      {isBlockExpanded ? (
                        <ChevronDown size={14} className="text-[var(--color-text-tertiary)]" />
                      ) : (
                        <ChevronRight size={14} className="text-[var(--color-text-tertiary)]" />
                      )}
                      <span className="text-sm font-medium text-[var(--color-text-primary)]">
                        {block.name}
                      </span>
                      <span
                        className={cn(
                          'text-xs px-2 py-0.5 rounded-full font-medium',
                          state?.status === 'completed' && 'bg-green-500/15 text-green-400',
                          state?.status === 'failed' && 'bg-red-500/15 text-red-400',
                          state?.status === 'running' && 'bg-blue-500/15 text-blue-400',
                          state?.status === 'skipped' && 'bg-gray-500/15 text-gray-400',
                          !state?.status && 'bg-gray-500/15 text-gray-400'
                        )}
                      >
                        {state?.status || 'pending'}
                      </span>
                      {/* For-each iteration count badge */}
                      {block.type === 'for_each' && forEachStates[block.id] && (
                        <span className="text-xs px-2 py-0.5 rounded-full font-medium bg-amber-500/15 text-amber-400 flex items-center gap-1">
                          <Repeat size={10} />
                          {forEachStates[block.id].currentIteration}/
                          {forEachStates[block.id].totalItems} iterations
                        </span>
                      )}
                    </div>
                    {output && (
                      <button
                        onClick={e => {
                          e.stopPropagation();
                          copyToClipboard(formatOutput(output), block.id);
                        }}
                        className="p-1.5 rounded-lg hover:bg-[var(--color-background)] text-[var(--color-text-tertiary)] hover:text-[var(--color-text-primary)] transition-colors"
                        title="Copy output"
                      >
                        {copiedId === block.id ? (
                          <Check size={12} className="text-green-400" />
                        ) : (
                          <Copy size={12} />
                        )}
                      </button>
                    )}
                  </button>
                  {isBlockExpanded && (
                    <div className="px-4 py-3 bg-[var(--color-background)] border-t border-[var(--color-border)] space-y-3">
                      {/* Debug: Show what we have */}
                      {console.log(`[DEBUG] Block ${block.id}:`, {
                        hasState: !!state,
                        hasInputs: !!state?.inputs,
                        inputsKeys: state?.inputs ? Object.keys(state.inputs) : [],
                        inputs: state?.inputs,
                      })}

                      {/* Available Inputs Section */}
                      {state?.inputs && Object.keys(state.inputs).length > 0 && (
                        <div className="space-y-2">
                          <div className="flex items-center gap-2">
                            <h4 className="text-xs font-semibold text-[var(--color-text-tertiary)] uppercase tracking-wide">
                              Available Inputs
                            </h4>
                            <span className="text-xs text-[var(--color-text-tertiary)] bg-[var(--color-surface)] px-2 py-0.5 rounded-full">
                              {Object.keys(state.inputs).length} keys
                            </span>
                          </div>
                          <div className="bg-[var(--color-surface)] rounded-lg p-3 border border-[var(--color-border)]">
                            <div className="flex flex-wrap gap-1.5">
                              {Object.keys(state.inputs).map(key => (
                                <span
                                  key={key}
                                  className="text-xs font-mono px-2 py-1 rounded-md bg-blue-500/10 text-blue-400 border border-blue-500/20"
                                  title={`Access with {{${key}}}`}
                                >
                                  {key}
                                </span>
                              ))}
                            </div>
                            <div className="mt-2 text-xs text-[var(--color-text-tertiary)] italic">
                              Use these in variable paths like{' '}
                              <span className="font-mono text-[var(--color-text-secondary)]">
                                {'{{'}
                                {Object.keys(state.inputs)[0] || 'key'}
                                {'}}'}
                              </span>
                            </div>
                          </div>
                        </div>
                      )}

                      {/* For-Each Iteration Results */}
                      {block.type === 'for_each' && forEachStates[block.id] && (
                        <div className="space-y-2">
                          <h4 className="text-xs font-semibold text-[var(--color-text-tertiary)] uppercase tracking-wide">
                            Iteration Results
                          </h4>
                          <ForEachOutputView state={forEachStates[block.id]} />
                        </div>
                      )}

                      {/* Output Section */}
                      <div className="space-y-2">
                        <h4 className="text-xs font-semibold text-[var(--color-text-tertiary)] uppercase tracking-wide">
                          Output
                        </h4>
                        {state?.error ? (
                          <div className="text-sm text-red-400 font-mono p-3 rounded-lg bg-red-500/10 border border-red-500/20">
                            Error: {state.error}
                          </div>
                        ) : output ? (
                          <pre className="text-xs text-[var(--color-text-secondary)] whitespace-pre-wrap break-words font-mono max-h-[200px] overflow-y-auto leading-relaxed bg-[var(--color-surface)] rounded-lg p-3 border border-[var(--color-border)]">
                            {formatOutput(output)}
                          </pre>
                        ) : (
                          <div className="text-sm text-[var(--color-text-tertiary)] italic p-3">
                            No output yet
                          </div>
                        )}
                      </div>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}
