import { useEffect, forwardRef } from 'react';
import type { HTMLAttributes, ReactNode } from 'react';
import './Toast.css';

export type ToastVariant = 'success' | 'error' | 'warning' | 'info';

export interface ToastProps extends Omit<HTMLAttributes<HTMLDivElement>, 'title'> {
  variant?: ToastVariant;
  title?: string;
  message: ReactNode;
  duration?: number;
  onClose?: () => void;
  action?: { label: string; onClick: () => void };
}

export const Toast = forwardRef<HTMLDivElement, ToastProps>(
  (
    {
      variant = 'info',
      title,
      message,
      duration = 5000,
      onClose,
      action,
      className = '',
      ...props
    },
    ref
  ) => {
    useEffect(() => {
      if (duration && onClose) {
        const timer = setTimeout(onClose, duration);
        return () => clearTimeout(timer);
      }
    }, [duration, onClose]);

    const icons = {
      success: '✓',
      error: '✕',
      warning: '⚠',
      info: 'ℹ',
    };

    return (
      <div ref={ref} className={`toast toast-${variant} ${className}`} {...props}>
        <div className="toast-icon">{icons[variant]}</div>
        <div className="toast-content">
          {title && <div className="toast-title">{title}</div>}
          <div className="toast-message">{message}</div>
          {action && (
            <button type="button" onClick={action.onClick} className="toast-action">
              {action.label}
            </button>
          )}
        </div>
        {onClose && (
          <button type="button" onClick={onClose} className="toast-close" aria-label="Close">
            ×
          </button>
        )}
      </div>
    );
  }
);

Toast.displayName = 'Toast';
