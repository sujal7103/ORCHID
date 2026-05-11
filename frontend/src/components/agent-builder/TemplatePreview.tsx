import { useMemo } from 'react';
import { useAgentBuilderStore } from '@/store/useAgentBuilderStore';

interface TemplatePreviewProps {
  value: string;
}

/** Regex to match all {{path.to.value}} template references */
const TEMPLATE_RE = /\{\{([^}]+)\}\}/g;

/**
 * Walk a nested object/array by a dot-separated path.
 * Numeric segments index into arrays (e.g. "0" → arr[0]).
 */
function resolvePath(data: unknown, path: string): unknown {
  const parts = path.split('.');
  let cur: unknown = data;
  for (const part of parts) {
    if (cur == null) return undefined;
    if (Array.isArray(cur)) {
      const idx = Number(part);
      if (Number.isNaN(idx)) return undefined;
      cur = cur[idx];
    } else if (typeof cur === 'object') {
      cur = (cur as Record<string, unknown>)[part];
    } else {
      return undefined;
    }
  }
  return cur;
}

/** Truncate a preview string to a reasonable length */
function formatPreview(value: unknown): string {
  if (value === undefined) return '';
  if (value === null) return 'null';
  if (typeof value === 'string') {
    return value.length > 120 ? value.slice(0, 117) + '...' : value;
  }
  if (typeof value === 'number' || typeof value === 'boolean') {
    return String(value);
  }
  // Arrays/objects — compact JSON preview
  const json = JSON.stringify(value);
  return json.length > 120 ? json.slice(0, 117) + '...' : json;
}

export function TemplatePreview({ value }: TemplatePreviewProps) {
  const { workflow, blockOutputCache, blockStates } = useAgentBuilderStore();

  const resolved = useMemo(() => {
    if (!value || !workflow) return [];

    // Extract all template refs
    const matches: string[] = [];
    let m: RegExpExecArray | null;
    const re = new RegExp(TEMPLATE_RE.source, TEMPLATE_RE.flags);
    while ((m = re.exec(value)) !== null) {
      matches.push(m[1]);
    }
    if (matches.length === 0) return [];

    // Build normalizedId → blockId lookup
    const normMap = new Map<string, string>();
    for (const b of workflow.blocks) {
      normMap.set(b.normalizedId, b.id);
    }

    return matches.map(template => {
      const dotIdx = template.indexOf('.');
      const blockRef = dotIdx === -1 ? template : template.slice(0, dotIdx);
      const dataPath = dotIdx === -1 ? '' : template.slice(dotIdx + 1);

      // Special case: {{input}} refers to workflow-level input, not a block
      if (blockRef === 'input') {
        return { template, resolved: false as const };
      }

      const blockId = normMap.get(blockRef);
      if (!blockId) {
        return { template, resolved: false as const };
      }

      // Try blockOutputCache first, then blockStates outputs
      let data: unknown = blockOutputCache[blockId];
      if (data === undefined) {
        const state = blockStates[blockId];
        if (state?.outputs) {
          data = state.outputs;
        }
      }

      if (data === undefined) {
        return { template, resolved: false as const };
      }

      const result = dataPath ? resolvePath(data, dataPath) : data;
      if (result === undefined) {
        return { template, resolved: false as const };
      }

      return { template, resolved: true as const, value: result };
    });
  }, [value, workflow, blockOutputCache, blockStates]);

  // Only render when we have templates and at least one resolves
  const hasTemplates = resolved.length > 0;
  const anyResolved = resolved.some(r => r.resolved);
  if (!hasTemplates || !anyResolved) return null;

  return (
    <div className="mt-1 space-y-0.5">
      {resolved.map((r, i) => (
        <div
          key={i}
          className="flex items-start gap-1.5 px-2 py-1 rounded bg-white/[0.03] text-[10px] font-mono leading-snug"
        >
          <span className="text-[var(--color-text-tertiary)] shrink-0">{`{{${r.template}}}`}</span>
          <span className="text-[var(--color-text-tertiary)] shrink-0">&rarr;</span>
          {r.resolved ? (
            <span className="text-[var(--color-accent)] break-all">{formatPreview(r.value)}</span>
          ) : (
            <span className="text-amber-400/70 italic">no data yet</span>
          )}
        </div>
      ))}
    </div>
  );
}
