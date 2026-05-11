import { useEffect } from 'react';
import { cn } from '@/lib/utils';

interface MobileSidebarOverlayProps {
  isOpen: boolean;
  onClose: () => void;
  children: React.ReactNode;
}

/**
 * Full-screen overlay that wraps the sidebar on mobile.
 * Includes a semi-transparent backdrop that closes the sidebar when tapped.
 * Slides in from the left with animation.
 */
export function MobileSidebarOverlay({ isOpen, onClose, children }: MobileSidebarOverlayProps) {
  // Prevent body scroll when sidebar is open
  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
    }
    return () => {
      document.body.style.overflow = '';
    };
  }, [isOpen]);

  // Handle escape key to close
  useEffect(() => {
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        onClose();
      }
    };
    window.addEventListener('keydown', handleEscape);
    return () => window.removeEventListener('keydown', handleEscape);
  }, [isOpen, onClose]);

  return (
    <>
      {/* Backdrop */}
      <div
        className={cn(
          'fixed inset-0 z-40 bg-black/60 backdrop-blur-sm',
          'transition-opacity duration-300',
          isOpen ? 'opacity-100' : 'opacity-0 pointer-events-none'
        )}
        onClick={onClose}
        aria-hidden="true"
      />

      {/* Sidebar container - slides in from left */}
      <div
        className={cn(
          'fixed inset-y-0 left-0 z-50 w-[280px]',
          'transform transition-transform duration-300 ease-out',
          isOpen ? 'translate-x-0' : '-translate-x-full'
        )}
      >
        {children}
      </div>
    </>
  );
}
