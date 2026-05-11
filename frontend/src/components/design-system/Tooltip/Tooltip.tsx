import type { ReactNode } from 'react';
import { useState, useRef, useEffect, useCallback } from 'react';
import { createPortal } from 'react-dom';
import './Tooltip.css';

export type TooltipPosition = 'top' | 'bottom' | 'left' | 'right';

export interface TooltipProps {
  content: ReactNode;
  children: ReactNode;
  position?: TooltipPosition;
  delay?: number;
  disabled?: boolean;
  className?: string;
}

interface TooltipCoords {
  top: number;
  left: number;
}

// Check if device is touch-based (mobile/tablet)
const isTouchDevice = () => {
  if (typeof window === 'undefined') return false;
  return 'ontouchstart' in window || navigator.maxTouchPoints > 0;
};

export const Tooltip = ({
  content,
  children,
  position = 'top',
  delay = 50,
  disabled = false,
  className = '',
}: TooltipProps) => {
  const [isVisible, setIsVisible] = useState(false);
  const [coords, setCoords] = useState<TooltipCoords>({ top: 0, left: 0 });
  const [isMobile, setIsMobile] = useState(false);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const triggerRef = useRef<HTMLDivElement>(null);

  // Detect mobile on mount
  useEffect(() => {
    setIsMobile(isTouchDevice());
  }, []);

  // Calculate position based on trigger element only - CSS transform handles centering
  const calculatePosition = useCallback(() => {
    if (!triggerRef.current) return;

    const rect = triggerRef.current.getBoundingClientRect();
    const gap = 34;

    let top = 0;
    let left = 0;

    switch (position) {
      case 'top':
        top = rect.top - gap;
        left = rect.left + rect.width / 2;
        break;
      case 'bottom':
        top = rect.bottom + gap;
        left = rect.left + rect.width / 2;
        break;
      case 'left':
        top = rect.top + rect.height / 2;
        left = rect.left - gap;
        break;
      case 'right':
        top = rect.top + rect.height / 2;
        left = rect.right + gap;
        break;
    }

    setCoords({ top, left });
  }, [position]);

  useEffect(() => {
    if (isVisible) {
      calculatePosition();
    }
  }, [isVisible, calculatePosition]);

  useEffect(() => {
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, []);

  const handleMouseEnter = () => {
    // Disable tooltips on mobile/touch devices
    if (disabled || isMobile) return;
    timeoutRef.current = setTimeout(() => {
      setIsVisible(true);
    }, delay);
  };

  const handleMouseLeave = () => {
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
    }
    setIsVisible(false);
  };

  return (
    <div
      ref={triggerRef}
      className={`tooltip-container ${className}`}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
    >
      {children}
      {isVisible &&
        !disabled &&
        createPortal(
          <div
            className={`tooltip-portal tooltip-portal-${position}`}
            style={{
              position: 'fixed',
              top: coords.top,
              left: coords.left,
              zIndex: 99999,
            }}
            role="tooltip"
          >
            <div className="tooltip tooltip-static">{content}</div>
          </div>,
          document.body
        )}
    </div>
  );
};
