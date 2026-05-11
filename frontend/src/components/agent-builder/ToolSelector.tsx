import { useState, useEffect, useCallback } from 'react';
import { Search, Loader2, AlertCircle } from 'lucide-react';
import { fetchTools, getToolRecommendations } from '@/services/toolsService';
import type { ToolCategory, ToolRecommendation } from '@/services/toolsService';
import { ToolCategorySection } from './ToolCategorySection';
import { ToolRecommendationBadge } from './ToolRecommendationBadge';

interface ToolSelectorProps {
  /**
   * Currently selected tool names
   */
  selectedTools: string[];

  /**
   * Callback when tools selection changes
   */
  onSelectionChange: (tools: string[]) => void;

  /**
   * Block context for recommendations
   */
  blockContext?: {
    name: string;
    description: string;
    type: string;
  };
}

export function ToolSelector({
  selectedTools,
  onSelectionChange,
  blockContext,
}: ToolSelectorProps) {
  const [categories, setCategories] = useState<ToolCategory[]>([]);
  const [recommendations, setRecommendations] = useState<ToolRecommendation[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');

  // Define callbacks first (before useEffects that reference them)
  const loadTools = useCallback(async () => {
    try {
      setIsLoading(true);
      setError(null);
      const data = await fetchTools();
      setCategories(data.categories);
    } catch (err) {
      console.error('Failed to load tools:', err);
      setError('Failed to load tools. Please try again.');
    } finally {
      setIsLoading(false);
    }
  }, []);

  const loadRecommendations = useCallback(async () => {
    if (!blockContext) return;

    try {
      const recs = await getToolRecommendations({
        block_name: blockContext.name,
        block_description: blockContext.description,
        block_type: blockContext.type,
      });
      setRecommendations(recs);
    } catch (err) {
      console.error('Failed to load recommendations:', err);
      // Don't show error for recommendations - it's optional
    }
  }, [blockContext]);

  // Fetch tools on mount
  useEffect(() => {
    loadTools();
  }, [loadTools]);

  // Fetch recommendations when block context changes
  useEffect(() => {
    if (blockContext) {
      loadRecommendations();
    }
  }, [blockContext, loadRecommendations]);

  const toggleTool = (toolName: string) => {
    const newSelection = selectedTools.includes(toolName)
      ? selectedTools.filter(t => t !== toolName)
      : [...selectedTools, toolName];

    onSelectionChange(newSelection);
  };

  const toggleRecommendedTool = (toolName: string) => {
    toggleTool(toolName);
  };

  // Filter categories based on search query
  const filteredCategories = searchQuery
    ? categories
        .map(category => ({
          ...category,
          tools: category.tools.filter(
            tool =>
              tool.display_name.toLowerCase().includes(searchQuery.toLowerCase()) ||
              tool.description.toLowerCase().includes(searchQuery.toLowerCase()) ||
              tool.keywords.some(keyword =>
                keyword.toLowerCase().includes(searchQuery.toLowerCase())
              )
          ),
        }))
        .filter(category => category.tools.length > 0)
    : categories;

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-8">
        <Loader2 className="animate-spin text-[var(--color-text-secondary)]" size={24} />
        <span className="ml-2 text-[var(--color-text-secondary)]">Loading tools...</span>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center gap-2 p-4 rounded-lg bg-red-500/10 border border-red-500/20">
        <AlertCircle size={18} className="text-red-400" />
        <span className="text-red-400 text-sm">{error}</span>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Recommendations Section */}
      {recommendations.length > 0 && (
        <div className="space-y-2">
          <h4 className="text-sm font-medium text-[var(--color-text-secondary)] flex items-center gap-2">
            <span className="inline-block w-2 h-2 rounded-full bg-[var(--color-accent)] animate-pulse" />
            Recommended Tools
          </h4>
          <div className="flex flex-wrap gap-2">
            {recommendations.slice(0, 5).map(rec => (
              <ToolRecommendationBadge
                key={rec.name}
                recommendation={rec}
                isSelected={selectedTools.includes(rec.name)}
                onToggle={() => toggleRecommendedTool(rec.name)}
              />
            ))}
          </div>
          <p className="text-xs text-[var(--color-text-tertiary)]">
            Based on: {recommendations[0]?.reason}
          </p>
        </div>
      )}

      {/* Search Bar */}
      <div className="relative">
        <Search
          size={16}
          className="absolute left-3 top-1/2 transform -translate-y-1/2 text-[var(--color-text-tertiary)]"
        />
        <input
          type="text"
          placeholder="Search tools..."
          value={searchQuery}
          onChange={e => setSearchQuery(e.target.value)}
          className="w-full pl-10 pr-3 py-2 rounded-lg bg-[var(--color-bg-primary)] border border-[var(--color-border)] text-[var(--color-text-primary)] placeholder:text-[var(--color-text-tertiary)] focus:outline-none focus:ring-2 focus:ring-[var(--color-accent)] focus:border-transparent"
        />
      </div>

      {/* Tool Categories */}
      <div className="space-y-3 max-h-[400px] overflow-y-auto pr-2">
        {filteredCategories.length === 0 ? (
          <div className="text-center py-8 text-[var(--color-text-tertiary)]">
            No tools found matching "{searchQuery}"
          </div>
        ) : (
          filteredCategories.map(category => (
            <ToolCategorySection
              key={category.name}
              category={category}
              selectedTools={selectedTools}
              onToolToggle={toggleTool}
            />
          ))
        )}
      </div>

      {/* Selection Summary */}
      <div className="pt-3 border-t border-[var(--color-border)]">
        <div className="flex items-center justify-between text-sm">
          <span className="text-[var(--color-text-secondary)]">Selected tools:</span>
          <span className="font-medium text-[var(--color-text-primary)]">
            {selectedTools.length} / {categories.reduce((sum, cat) => sum + cat.count, 0)}
          </span>
        </div>
      </div>
    </div>
  );
}
