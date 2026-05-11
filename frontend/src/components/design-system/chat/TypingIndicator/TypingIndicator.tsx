import { forwardRef } from 'react';
import type { HTMLAttributes } from 'react';
import './TypingIndicator.css';

export interface TypingIndicatorProps extends HTMLAttributes<HTMLDivElement> {
  text?: string;
}

export const TypingIndicator = forwardRef<HTMLDivElement, TypingIndicatorProps>(
  ({ text = 'AI is typing', className = '', ...props }, ref) => {
    return (
      <div ref={ref} className={`typing-indicator ${className}`} {...props}>
        <span className="typing-indicator-text">{text}</span>
        <div className="typing-indicator-dots">
          <span className="typing-indicator-dot"></span>
          <span className="typing-indicator-dot"></span>
          <span className="typing-indicator-dot"></span>
        </div>
      </div>
    );
  }
);

TypingIndicator.displayName = 'TypingIndicator';
