import { useCallback } from 'react';
import { BaseEdge, EdgeLabelRenderer, getBezierPath } from '@xyflow/react';
import type { EdgeProps, Edge } from '@xyflow/react';
import { X } from 'lucide-react';
import { useAgentBuilderStore } from '@/store/useAgentBuilderStore';

export function DeletableEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  style,
  markerEnd,
  className,
}: EdgeProps<Edge>) {
  const removeConnection = useAgentBuilderStore(s => s.removeConnection);

  const [edgePath, labelX, labelY] = getBezierPath({
    sourceX,
    sourceY,
    targetX,
    targetY,
    sourcePosition,
    targetPosition,
  });

  const handleDelete = useCallback(
    (e: React.MouseEvent) => {
      e.stopPropagation();
      removeConnection(id);
    },
    [id, removeConnection]
  );

  return (
    <>
      <BaseEdge
        path={edgePath}
        markerEnd={markerEnd}
        style={style}
        className={className}
      />
      <EdgeLabelRenderer>
        <button
          className="edge-delete-btn"
          style={{
            position: 'absolute',
            transform: `translate(-50%, -50%) translate(${labelX}px,${labelY}px)`,
            pointerEvents: 'all',
          }}
          onClick={handleDelete}
        >
          <X size={10} />
        </button>
      </EdgeLabelRenderer>
    </>
  );
}
