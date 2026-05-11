import { useEffect, forwardRef } from 'react';
import type { HTMLAttributes, ReactNode } from 'react';
import './Modal.css';

export type ModalSize = 'sm' | 'md' | 'lg' | 'full';

export interface ModalProps extends Omit<HTMLAttributes<HTMLDivElement>, 'title'> {
  isOpen: boolean;
  onClose: () => void;
  title?: ReactNode;
  footer?: ReactNode;
  size?: ModalSize;
  closeOnBackdrop?: boolean;
  closeOnEscape?: boolean;
  showClose?: boolean;
  children: ReactNode;
}

export const Modal = forwardRef<HTMLDivElement, ModalProps>(
  (
    {
      isOpen,
      onClose,
      title,
      footer,
      size = 'md',
      closeOnBackdrop = true,
      closeOnEscape = true,
      showClose = true,
      children,
      className = '',
      ...props
    },
    ref
  ) => {
    useEffect(() => {
      if (!isOpen) return;

      const handleEscape = (e: KeyboardEvent) => {
        if (closeOnEscape && e.key === 'Escape') {
          onClose();
        }
      };

      document.addEventListener('keydown', handleEscape);
      document.body.style.overflow = 'hidden';

      return () => {
        document.removeEventListener('keydown', handleEscape);
        document.body.style.overflow = '';
      };
    }, [isOpen, closeOnEscape, onClose]);

    if (!isOpen) return null;

    const handleBackdropClick = (e: React.MouseEvent) => {
      if (closeOnBackdrop && e.target === e.currentTarget) {
        onClose();
      }
    };

    return (
      <div className="modal-backdrop" onClick={handleBackdropClick}>
        <div ref={ref} className={`modal modal-${size} ${className}`} {...props}>
          {(title || showClose) && (
            <div className="modal-header">
              {title && <h3 className="modal-title">{title}</h3>}
              {showClose && (
                <button type="button" onClick={onClose} className="modal-close" aria-label="Close">
                  <svg
                    width="24"
                    height="24"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                  >
                    <line x1="18" y1="6" x2="6" y2="18" />
                    <line x1="6" y1="6" x2="18" y2="18" />
                  </svg>
                </button>
              )}
            </div>
          )}
          <div className="modal-body">{children}</div>
          {footer && <div className="modal-footer">{footer}</div>}
        </div>
      </div>
    );
  }
);

Modal.displayName = 'Modal';
