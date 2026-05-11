import { Menu, X } from 'lucide-react';
import { cn } from '@/lib/utils';

interface MobileMenuButtonProps {
  isOpen: boolean;
  onToggle: () => void;
  className?: string;
}

/**
 * Fixed hamburger menu button for mobile navigation.
 * Positioned in the top-left corner.
 */
export function MobileMenuButton({ isOpen, onToggle, className }: MobileMenuButtonProps) {
  return (
    <button
      onClick={onToggle}
      className={cn(
        'fixed top-4 left-4 z-50 p-2.5 rounded-xl',
        'bg-[var(--color-surface-elevated)] border border-[var(--color-border)]',
        'shadow-lg backdrop-blur-sm',
        'text-[var(--color-text-secondary)] hover:text-[var(--color-text-primary)]',
        'transition-all duration-200',
        'active:scale-95',
        className
      )}
      aria-label={isOpen ? 'Close menu' : 'Open menu'}
      aria-expanded={isOpen}
    >
      {isOpen ? <X size={22} /> : <Menu size={22} />}
    </button>
  );
}
