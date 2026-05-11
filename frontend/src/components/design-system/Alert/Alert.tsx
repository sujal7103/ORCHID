import type { ReactNode } from 'react';
import './Alert.css';

export type AlertVariant = 'info' | 'success' | 'warning' | 'error';

export interface AlertProps {
  variant?: AlertVariant;
  title?: string;
  message: ReactNode;
  icon?: ReactNode;
  dismissible?: boolean;
  onDismiss?: () => void;
  action?: ReactNode;
  className?: string;
}

const DefaultIcons = {
  info: (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
      <circle cx="10" cy="10" r="8" stroke="currentColor" strokeWidth="1.5" />
      <path d="M10 10V14" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <circle cx="10" cy="7" r="0.75" fill="currentColor" />
    </svg>
  ),
  success: (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
      <circle cx="10" cy="10" r="8" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="M7 10L9 12L13 8"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  ),
  warning: (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
      <path
        d="M10 2L2 16H18L10 2Z"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
      <path d="M10 8V11" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      <circle cx="10" cy="14" r="0.75" fill="currentColor" />
    </svg>
  ),
  error: (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
      <circle cx="10" cy="10" r="8" stroke="currentColor" strokeWidth="1.5" />
      <path
        d="M12 8L8 12M8 8L12 12"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
      />
    </svg>
  ),
};

export const Alert = ({
  variant = 'info',
  title,
  message,
  icon,
  dismissible = false,
  onDismiss,
  action,
  className = '',
}: AlertProps) => {
  const displayIcon = icon !== undefined ? icon : DefaultIcons[variant];

  return (
    <div className={`alert alert-${variant} ${className}`} role="alert">
      {displayIcon && <div className="alert-icon">{displayIcon}</div>}

      <div className="alert-content">
        {title && <div className="alert-title">{title}</div>}
        <div className="alert-message">{message}</div>
      </div>

      {action && <div className="alert-action">{action}</div>}

      {dismissible && (
        <button className="alert-dismiss" onClick={onDismiss} aria-label="Dismiss alert">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
            <path
              d="M12 4L4 12M4 4L12 12"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
            />
          </svg>
        </button>
      )}
    </div>
  );
};
