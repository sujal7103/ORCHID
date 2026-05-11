import { useMemo } from 'react';
import {
  Brain,
  Variable,
  ChevronRight,
  CheckCircle2,
  Loader2,
  XCircle,
  Circle,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import type { Block, Connection, BlockExecutionState, BlockType } from '@/types/agent';

interface MobileBlockListViewProps {
  blocks: Block[];
  connections: Connection[];
  blockStates: Record<string, BlockExecutionState>;
  selectedBlockId: string | null;
  onBlockSelect: (blockId: string) => void;
}

/**
 * Get icon component for block type
 */
function getBlockIcon(type: BlockType) {
  switch (type) {
    case 'llm_inference':
      return Brain;
    case 'variable':
      return Variable;
    default:
      return Circle;
  }
}

/**
 * Get human-readable label for block type
 */
function getBlockTypeLabel(type: BlockType): string {
  switch (type) {
    case 'llm_inference':
      return 'AI Block';
    case 'variable':
      return 'Input/Variable';
    default:
      return type;
  }
}

/**
 * Sort blocks by execution order (topological sort based on connections)
 */
function sortBlocksByExecutionOrder(blocks: Block[], connections: Connection[]): Block[] {
  if (blocks.length === 0) return [];

  // Build adjacency list
  const graph = new Map<string, string[]>();
  const inDegree = new Map<string, number>();

  // Initialize
  blocks.forEach(block => {
    graph.set(block.id, []);
    inDegree.set(block.id, 0);
  });

  // Build graph from connections
  connections.forEach(conn => {
    const targets = graph.get(conn.sourceBlockId);
    if (targets) {
      targets.push(conn.targetBlockId);
      inDegree.set(conn.targetBlockId, (inDegree.get(conn.targetBlockId) || 0) + 1);
    }
  });

  // Kahn's algorithm for topological sort
  const queue: string[] = [];
  const result: Block[] = [];
  const blockMap = new Map(blocks.map(b => [b.id, b]));

  // Find all nodes with no incoming edges
  inDegree.forEach((degree, blockId) => {
    if (degree === 0) queue.push(blockId);
  });

  while (queue.length > 0) {
    const blockId = queue.shift()!;
    const block = blockMap.get(blockId);
    if (block) result.push(block);

    const neighbors = graph.get(blockId) || [];
    neighbors.forEach(neighbor => {
      const newDegree = (inDegree.get(neighbor) || 0) - 1;
      inDegree.set(neighbor, newDegree);
      if (newDegree === 0) queue.push(neighbor);
    });
  }

  // If not all blocks are in result (cycle detected), add remaining
  if (result.length < blocks.length) {
    blocks.forEach(block => {
      if (!result.find(b => b.id === block.id)) {
        result.push(block);
      }
    });
  }

  return result;
}

/**
 * Simplified workflow block list for mobile.
 * Shows blocks in execution order with status indicators.
 * No drag-drop functionality.
 */
export function MobileBlockListView({
  blocks,
  connections,
  blockStates,
  selectedBlockId,
  onBlockSelect,
}: MobileBlockListViewProps) {
  // Sort blocks by execution order
  const orderedBlocks = useMemo(
    () => sortBlocksByExecutionOrder(blocks, connections),
    [blocks, connections]
  );

  if (blocks.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center p-6 text-center">
        <div className="space-y-2">
          <p className="text-[var(--color-text-secondary)]">No blocks in workflow</p>
          <p className="text-sm text-[var(--color-text-tertiary)]">
            Chat with the agent to build your workflow
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex-1 overflow-y-auto">
      {/* Header */}
      <div className="sticky top-0 z-10 px-4 py-3 bg-[var(--color-bg-primary)] border-b border-[var(--color-border)]">
        <h2 className="text-sm font-semibold text-[var(--color-text-secondary)] uppercase tracking-wide">
          Workflow Blocks ({blocks.length})
        </h2>
      </div>

      {/* Block list */}
      <div className="p-4 space-y-3">
        {orderedBlocks.map((block, index) => {
          const state = blockStates[block.id];
          const isSelected = selectedBlockId === block.id;
          const Icon = getBlockIcon(block.type);
          const isStartBlock = block.type === 'variable' && block.config?.operation === 'read';

          return (
            <div key={block.id} className="relative">
              {/* Connection line to next block */}
              {index < orderedBlocks.length - 1 && (
                <div className="absolute left-[26px] top-full h-3 w-0.5 bg-[var(--color-border)] z-0" />
              )}

              <button
                onClick={() => onBlockSelect(block.id)}
                className={cn(
                  'w-full text-left p-4 rounded-xl border transition-all relative',
                  'active:scale-[0.98]',
                  isSelected
                    ? 'bg-[var(--color-accent)]/10 border-[var(--color-accent)]'
                    : 'bg-[var(--color-surface)] border-[var(--color-border)] hover:border-[var(--color-accent)]/50'
                )}
              >
                <div className="flex items-center gap-3">
                  {/* Block type icon */}
                  <div
                    className={cn(
                      'w-10 h-10 rounded-lg flex items-center justify-center flex-shrink-0',
                      block.type === 'llm_inference'
                        ? 'bg-purple-500/20 text-purple-400'
                        : 'bg-orange-500/20 text-orange-400'
                    )}
                  >
                    <Icon size={20} />
                  </div>

                  {/* Block info */}
                  <div className="flex-1 min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-[var(--color-text-primary)] truncate">
                        {block.name}
                      </span>
                      {isStartBlock && (
                        <span className="text-xs px-1.5 py-0.5 rounded bg-green-500/20 text-green-400">
                          Start
                        </span>
                      )}
                    </div>
                    <div className="text-xs text-[var(--color-text-tertiary)] truncate mt-0.5">
                      {block.description || getBlockTypeLabel(block.type)}
                    </div>
                  </div>

                  {/* Status indicator */}
                  <div className="flex items-center gap-2 flex-shrink-0">
                    <BlockStatusBadge status={state?.status} />
                    <ChevronRight size={18} className="text-[var(--color-text-tertiary)]" />
                  </div>
                </div>
              </button>
            </div>
          );
        })}
      </div>
    </div>
  );
}

/**
 * Status badge for block execution state
 */
function BlockStatusBadge({ status }: { status?: string }) {
  if (!status || status === 'pending') {
    return <Circle size={16} className="text-[var(--color-text-tertiary)]" />;
  }

  switch (status) {
    case 'running':
      return <Loader2 size={16} className="text-blue-400 animate-spin" />;
    case 'completed':
      return <CheckCircle2 size={16} className="text-green-400" />;
    case 'failed':
      return <XCircle size={16} className="text-red-400" />;
    default:
      return <Circle size={16} className="text-[var(--color-text-tertiary)]" />;
  }
}
