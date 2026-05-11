import { useEffect, useState, useMemo, useRef, useCallback } from 'react';
import { motion, AnimatePresence, useMotionValue, useSpring } from 'framer-motion';
import {
  Loader2,
  Check,
  BarChart2,
  Search,
  FileText,
  Mic,
  Clock,
  MessageCircle,
  Video,
  CheckSquare,
  Users,
  GitBranch,
  Layout,
  ShoppingBag,
  Twitter,
  Sparkles,
  Wrench,
} from 'lucide-react';
import { cn } from '@/lib/utils';
import type { GenerationStep, SelectedTool, ToolDefinition } from '@/services/workflowService';

// Map category IDs to icons
const categoryIcons: Record<string, React.ComponentType<{ size?: number; className?: string }>> = {
  data_analysis: BarChart2,
  search_web: Search,
  content_creation: FileText,
  media_processing: Mic,
  utilities: Clock,
  messaging: MessageCircle,
  video_conferencing: Video,
  project_management: CheckSquare,
  crm_sales: Users,
  analytics: BarChart2,
  code_devops: GitBranch,
  productivity: Layout,
  ecommerce: ShoppingBag,
  social_media: Twitter,
};

interface ToolSelectionProgressProps {
  steps: GenerationStep[];
  currentStep: number;
  selectedTools?: SelectedTool[];
  toolRegistry?: ToolDefinition[];
  isComplete?: boolean;
}

interface FleeingIconProps {
  category: string;
  index: number;
  totalCount: number;
  radius: number;
  containerRef: React.RefObject<HTMLDivElement>;
  isComplete?: boolean;
}

/**
 * Individual orbiting icon that flees from cursor
 */
function FleeingIcon({
  category,
  index,
  totalCount,
  radius,
  containerRef,
  isComplete,
}: FleeingIconProps) {
  const Icon = categoryIcons[category] || Wrench;

  // Base position (where it wants to be)
  const baseAngle = (index / totalCount) * 2 * Math.PI - Math.PI / 2;
  const baseX = Math.cos(baseAngle) * radius;
  const baseY = Math.sin(baseAngle) * radius;

  // Current offset from cursor avoidance
  const offsetX = useMotionValue(0);
  const offsetY = useMotionValue(0);

  // Smooth spring animation for the offset
  const springX = useSpring(offsetX, { stiffness: 300, damping: 25 });
  const springY = useSpring(offsetY, { stiffness: 300, damping: 25 });

  // Handle mouse movement
  const handleMouseMove = useCallback(
    (e: MouseEvent) => {
      if (!containerRef.current || isComplete) return;

      const rect = containerRef.current.getBoundingClientRect();
      const centerX = rect.left + rect.width / 2;
      const centerY = rect.top + rect.height / 2;

      // Icon's current world position
      const iconWorldX = centerX + baseX;
      const iconWorldY = centerY + baseY;

      // Distance from cursor to icon
      const dx = e.clientX - iconWorldX;
      const dy = e.clientY - iconWorldY;
      const distance = Math.sqrt(dx * dx + dy * dy);

      // Flee threshold - how close before it starts running
      const fleeThreshold = 80;
      // How far to flee
      const fleeDistance = 40;

      if (distance < fleeThreshold && distance > 0) {
        // Calculate flee direction (away from cursor)
        const fleeStrength = 1 - distance / fleeThreshold;
        const fleeX = (-dx / distance) * fleeDistance * fleeStrength;
        const fleeY = (-dy / distance) * fleeDistance * fleeStrength;

        offsetX.set(fleeX);
        offsetY.set(fleeY);
      } else {
        // Return to base position
        offsetX.set(0);
        offsetY.set(0);
      }
    },
    [baseX, baseY, containerRef, isComplete, offsetX, offsetY]
  );

  useEffect(() => {
    window.addEventListener('mousemove', handleMouseMove);
    return () => window.removeEventListener('mousemove', handleMouseMove);
  }, [handleMouseMove]);

  // Reset offset when complete
  useEffect(() => {
    if (isComplete) {
      offsetX.set(0);
      offsetY.set(0);
    }
  }, [isComplete, offsetX, offsetY]);

  return (
    <motion.div
      className="absolute w-12 h-12 rounded-xl bg-[var(--color-surface-elevated)] border border-[var(--color-border)] flex items-center justify-center shadow-lg cursor-pointer select-none"
      initial={{
        opacity: 0,
        scale: 0,
        x: 0,
        y: 0,
      }}
      animate={{
        opacity: 1,
        scale: 1,
        x: baseX,
        y: baseY,
      }}
      exit={{
        opacity: 0,
        scale: 0,
      }}
      transition={{
        delay: index * 0.1,
        duration: 0.5,
        type: 'spring',
        stiffness: 200,
      }}
      style={{
        // Add spring offset on top of base position
        translateX: springX,
        translateY: springY,
      }}
      whileHover={{
        scale: 1.1,
        borderColor: 'var(--color-accent)',
      }}
    >
      <Icon size={20} className="text-[var(--color-accent)]" />
    </motion.div>
  );
}

/**
 * Floating tool icons animation component
 * Shows selected tools orbiting around a central icon during workflow generation
 * Icons flee from the cursor on hover for a playful "don't touch me" effect
 */
export function ToolSelectionProgress({
  steps,
  currentStep,
  selectedTools,
  toolRegistry,
  isComplete,
}: ToolSelectionProgressProps) {
  const [showTools, setShowTools] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  // Show tools after step 1 completes
  useEffect(() => {
    if (currentStep >= 2 || (steps[0]?.status === 'completed' && selectedTools?.length)) {
      setShowTools(true);
    }
  }, [currentStep, steps, selectedTools]);

  // Get unique categories from selected tools
  const selectedCategories = useMemo(() => {
    if (!selectedTools) return [];
    const categories = new Set(selectedTools.map(t => t.category));
    return Array.from(categories);
  }, [selectedTools]);

  // Get tool definitions for selected tools
  const toolDetails = useMemo(() => {
    if (!selectedTools || !toolRegistry) return [];
    return selectedTools
      .map(st => toolRegistry.find(t => t.id === st.tool_id))
      .filter((t): t is ToolDefinition => !!t);
  }, [selectedTools, toolRegistry]);

  return (
    <div className="flex flex-col items-center gap-8">
      {/* Floating Tools Animation */}
      <div ref={containerRef} className="relative w-64 h-64 flex items-center justify-center">
        {/* Central icon */}
        <motion.div
          className={cn(
            'w-20 h-20 rounded-2xl flex items-center justify-center z-10',
            isComplete
              ? 'bg-green-500/20 border-2 border-green-500'
              : 'bg-[var(--color-accent)]/20 border-2 border-[var(--color-accent)]'
          )}
          animate={{
            scale: isComplete ? [1, 1.1, 1] : [1, 1.05, 1],
          }}
          transition={{
            duration: 2,
            repeat: isComplete ? 0 : Infinity,
            ease: 'easeInOut',
          }}
        >
          {isComplete ? (
            <Check size={40} className="text-green-500" />
          ) : currentStep === 1 ? (
            <Search size={40} className="text-[var(--color-accent)]" />
          ) : (
            <Sparkles size={40} className="text-[var(--color-accent)]" />
          )}
        </motion.div>

        {/* Orbiting tool icons that flee from cursor */}
        <AnimatePresence>
          {showTools &&
            selectedCategories.map((category, index) => (
              <FleeingIcon
                key={category}
                category={category}
                index={index}
                totalCount={selectedCategories.length}
                radius={90}
                containerRef={containerRef as React.RefObject<HTMLDivElement>}
                isComplete={isComplete}
              />
            ))}
        </AnimatePresence>

        {/* Connecting lines to orbiting icons */}
        {showTools && !isComplete && (
          <svg className="absolute inset-0 w-full h-full pointer-events-none">
            {selectedCategories.map((category, index) => {
              const angle = (index / selectedCategories.length) * 2 * Math.PI - Math.PI / 2;
              const radius = 90;
              const x = 128 + Math.cos(angle) * radius;
              const y = 128 + Math.sin(angle) * radius;

              return (
                <motion.line
                  key={category}
                  x1="128"
                  y1="128"
                  x2={x}
                  y2={y}
                  stroke="var(--color-accent)"
                  strokeWidth="1"
                  strokeDasharray="4 4"
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 0.3 }}
                  transition={{ delay: index * 0.1 + 0.3 }}
                />
              );
            })}
          </svg>
        )}
      </div>

      {/* Progress Steps */}
      <div className="flex flex-col items-center gap-4 w-full max-w-md">
        {steps.map((step, index) => (
          <motion.div
            key={step.step_number}
            className={cn(
              'flex items-center gap-4 w-full p-4 rounded-xl transition-all',
              step.status === 'running' &&
                'bg-[var(--color-accent)]/10 border border-[var(--color-accent)]/30',
              step.status === 'completed' && 'bg-green-500/10 border border-green-500/30',
              step.status === 'pending' &&
                'bg-[var(--color-surface-elevated)] border border-[var(--color-border)] opacity-50',
              step.status === 'failed' && 'bg-red-500/10 border border-red-500/30'
            )}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ delay: index * 0.1 }}
          >
            {/* Step indicator */}
            <div
              className={cn(
                'w-10 h-10 rounded-full flex items-center justify-center flex-shrink-0',
                step.status === 'running' && 'bg-[var(--color-accent)]',
                step.status === 'completed' && 'bg-green-500',
                step.status === 'pending' && 'bg-[var(--color-border)]',
                step.status === 'failed' && 'bg-red-500'
              )}
            >
              {step.status === 'running' ? (
                <Loader2 size={20} className="text-white animate-spin" />
              ) : step.status === 'completed' ? (
                <Check size={20} className="text-white" />
              ) : (
                <span className="text-sm font-bold text-white">{step.step_number}</span>
              )}
            </div>

            {/* Step info */}
            <div className="flex-1 min-w-0">
              <div className="flex items-center justify-between">
                <h4
                  className={cn(
                    'font-semibold',
                    step.status === 'completed' && 'text-green-400',
                    step.status === 'running' && 'text-[var(--color-accent)]',
                    step.status === 'pending' && 'text-[var(--color-text-tertiary)]',
                    step.status === 'failed' && 'text-red-400'
                  )}
                >
                  {step.step_name}
                </h4>
                {step.status === 'completed' && step.tools && (
                  <span className="text-xs text-green-400/70">
                    {step.tools.length} tools selected
                  </span>
                )}
              </div>
              <p className="text-sm text-[var(--color-text-tertiary)] truncate">
                {step.description}
              </p>
            </div>
          </motion.div>
        ))}
      </div>

      {/* Selected Tools List */}
      {showTools && toolDetails.length > 0 && (
        <motion.div
          className="w-full max-w-md"
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.3 }}
        >
          <h4 className="text-sm font-medium text-[var(--color-text-tertiary)] mb-3 text-center">
            Selected Tools
          </h4>
          <div className="flex flex-wrap justify-center gap-2">
            {toolDetails.slice(0, 8).map(tool => {
              const Icon = categoryIcons[tool.category] || Wrench;
              return (
                <motion.div
                  key={tool.id}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-[var(--color-surface-elevated)] border border-[var(--color-border)] text-xs"
                  initial={{ opacity: 0, scale: 0.8 }}
                  animate={{ opacity: 1, scale: 1 }}
                  transition={{ delay: Math.random() * 0.3 }}
                  title={tool.description}
                >
                  <Icon size={12} className="text-[var(--color-accent)]" />
                  <span className="text-[var(--color-text-secondary)]">{tool.name}</span>
                </motion.div>
              );
            })}
            {toolDetails.length > 8 && (
              <span className="text-xs text-[var(--color-text-tertiary)] self-center">
                +{toolDetails.length - 8} more
              </span>
            )}
          </div>
        </motion.div>
      )}
    </div>
  );
}
