import { forwardRef } from 'react';
import type { HTMLAttributes, ReactNode } from 'react';
import './ChatBubble.css';

export type ChatBubbleRole = 'user' | 'assistant';
export type ChatBubbleStatus = 'sending' | 'sent' | 'error';

export interface ChatBubbleProps extends Omit<HTMLAttributes<HTMLDivElement>, 'content'> {
  role: ChatBubbleRole;
  content: ReactNode;
  avatar?: ReactNode;
  timestamp?: string;
  status?: ChatBubbleStatus;
  showAvatar?: boolean;
  actions?: ReactNode;
}

export const ChatBubble = forwardRef<HTMLDivElement, ChatBubbleProps>(
  (
    {
      role,
      content,
      avatar,
      timestamp,
      status = 'sent',
      showAvatar = true,
      actions,
      className = '',
      ...props
    },
    ref
  ) => {
    const classes = [
      'chat-bubble',
      `chat-bubble-${role}`,
      status && `chat-bubble-${status}`,
      className,
    ]
      .filter(Boolean)
      .join(' ');

    const defaultAvatar = role === 'user' ? '👤' : '🤖';

    return (
      <div ref={ref} className={classes} {...props}>
        {showAvatar && (
          <div className="chat-bubble-avatar">
            {avatar || <span className="chat-bubble-avatar-default">{defaultAvatar}</span>}
          </div>
        )}
        <div className="chat-bubble-content-wrapper">
          <div className="chat-bubble-content">{content}</div>
          {(timestamp || actions) && (
            <div className="chat-bubble-meta">
              {timestamp && <span className="chat-bubble-timestamp">{timestamp}</span>}
              {status === 'sending' && <span className="chat-bubble-status">Sending...</span>}
              {status === 'error' && <span className="chat-bubble-status-error">Failed</span>}
              {actions && <div className="chat-bubble-actions">{actions}</div>}
            </div>
          )}
        </div>
      </div>
    );
  }
);

ChatBubble.displayName = 'ChatBubble';
