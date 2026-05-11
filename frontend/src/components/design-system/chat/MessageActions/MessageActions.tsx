import { forwardRef } from 'react';
import type { HTMLAttributes } from 'react';
import { Tooltip } from '@/components/design-system/Tooltip/Tooltip';
import './MessageActions.css';

export interface MessageActionsProps extends HTMLAttributes<HTMLDivElement> {
  onCopy?: () => void;
  onRegenerate?: () => void;
  onEdit?: () => void;
  onDelete?: () => void;
}

export const MessageActions = forwardRef<HTMLDivElement, MessageActionsProps>(
  ({ onCopy, onRegenerate, onEdit, onDelete, className = '', ...props }, ref) => {
    return (
      <div ref={ref} className={`message-actions ${className}`} {...props}>
        {onCopy && (
          <Tooltip content="Copy" position="top">
            <button type="button" onClick={onCopy} className="message-actions-btn">
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
              </svg>
            </button>
          </Tooltip>
        )}
        {onRegenerate && (
          <Tooltip content="Regenerate" position="top">
            <button type="button" onClick={onRegenerate} className="message-actions-btn">
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <polyline points="23 4 23 10 17 10" />
                <path d="M20.49 15a9 9 0 1 1-2.12-9.36L23 10" />
              </svg>
            </button>
          </Tooltip>
        )}
        {onEdit && (
          <Tooltip content="Edit" position="top">
            <button type="button" onClick={onEdit} className="message-actions-btn">
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7" />
                <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z" />
              </svg>
            </button>
          </Tooltip>
        )}
        {onDelete && (
          <Tooltip content="Delete" position="top">
            <button
              type="button"
              onClick={onDelete}
              className="message-actions-btn message-actions-btn-danger"
            >
              <svg
                width="16"
                height="16"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <polyline points="3 6 5 6 21 6" />
                <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2" />
              </svg>
            </button>
          </Tooltip>
        )}
      </div>
    );
  }
);

MessageActions.displayName = 'MessageActions';
