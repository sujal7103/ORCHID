import { useState } from 'react';
import { ChevronDown, ChevronRight } from 'lucide-react';
import * as LucideIcons from 'lucide-react';
import type { ToolCategory, Tool } from '@/services/toolsService';

interface ToolCategorySectionProps {
  category: ToolCategory;
  selectedTools: string[];
  onToolToggle: (toolName: string) => void;
}

export function ToolCategorySection({
  category,
  selectedTools,
  onToolToggle,
}: ToolCategorySectionProps) {
  const [isExpanded, setIsExpanded] = useState(true);

  const getCategoryIcon = (categoryName: string) => {
    const icons: Record<string, string> = {
      data_sources: 'Database',
      computation: 'Cpu',
      time: 'Clock',
      output: 'FileOutput',
      integration: 'Network',
      other: 'Package',
    };
    return icons[categoryName] || 'Package';
  };

  const getCategoryLabel = (categoryName: string) => {
    const labels: Record<string, string> = {
      data_sources: 'Data Sources',
      computation: 'Computation',
      time: 'Time & Date',
      output: 'Output & Files',
      integration: 'Integration',
      other: 'Other',
    };
    return labels[categoryName] || categoryName;
  };

  const CategoryIcon =
    (LucideIcons as Record<string, React.ComponentType<{ size?: number; className?: string }>>)[
      getCategoryIcon(category.name)
    ] || LucideIcons.Package;

  return (
    <div className="border border-[var(--color-border)] rounded-lg overflow-hidden">
      {/* Category Header */}
      <button
        onClick={() => setIsExpanded(!isExpanded)}
        className="w-full flex items-center justify-between px-3 py-2 bg-[var(--color-bg-secondary)] hover:bg-[var(--color-bg-tertiary)] transition-colors"
      >
        <div className="flex items-center gap-2">
          <CategoryIcon size={16} className="text-[var(--color-accent)]" />
          <span className="text-sm font-medium text-[var(--color-text-primary)]">
            {getCategoryLabel(category.name)}
          </span>
          <span className="text-xs text-[var(--color-text-tertiary)] bg-[var(--color-bg-primary)] px-2 py-0.5 rounded-full">
            {category.count}
          </span>
        </div>
        {isExpanded ? (
          <ChevronDown size={16} className="text-[var(--color-text-secondary)]" />
        ) : (
          <ChevronRight size={16} className="text-[var(--color-text-secondary)]" />
        )}
      </button>

      {/* Tool List */}
      {isExpanded && (
        <div className="p-2 space-y-1 bg-[var(--color-bg-primary)]">
          {category.tools.map(tool => (
            <ToolCheckboxItem
              key={tool.name}
              tool={tool}
              isSelected={selectedTools.includes(tool.name)}
              onToggle={() => onToolToggle(tool.name)}
            />
          ))}
        </div>
      )}
    </div>
  );
}

interface ToolCheckboxItemProps {
  tool: Tool;
  isSelected: boolean;
  onToggle: () => void;
}

function ToolCheckboxItem({ tool, isSelected, onToggle }: ToolCheckboxItemProps) {
  const ToolIcon =
    (LucideIcons as Record<string, React.ComponentType<{ size?: number; className?: string }>>)[
      tool.icon
    ] || LucideIcons.Wrench;

  return (
    <label className="flex items-start gap-3 p-2 rounded-lg hover:bg-[var(--color-bg-secondary)] cursor-pointer transition-colors group">
      {/* Checkbox */}
      <input
        type="checkbox"
        checked={isSelected}
        onChange={onToggle}
        className="mt-1 w-4 h-4 rounded border-2 border-[var(--color-border)] bg-[var(--color-bg-primary)] checked:bg-[var(--color-accent)] checked:border-[var(--color-accent)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:ring-offset-2 focus:ring-offset-[var(--color-bg-primary)] cursor-pointer"
      />

      {/* Tool Info */}
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-2">
          <ToolIcon
            size={14}
            className={`flex-shrink-0 ${
              isSelected ? 'text-[var(--color-accent)]' : 'text-[var(--color-text-tertiary)]'
            }`}
          />
          <span
            className={`text-sm font-medium ${
              isSelected ? 'text-[var(--color-text-primary)]' : 'text-[var(--color-text-secondary)]'
            } group-hover:text-[var(--color-text-primary)] transition-colors`}
          >
            {tool.display_name}
          </span>
          {tool.source === 'mcp_local' && (
            <span className="text-[10px] px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-400 font-medium">
              MCP
            </span>
          )}
        </div>
        <p className="text-xs text-[var(--color-text-tertiary)] mt-0.5 line-clamp-2">
          {tool.description}
        </p>
      </div>
    </label>
  );
}
