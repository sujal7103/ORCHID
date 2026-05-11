import type { ReactNode } from 'react';
import './EmptyState.css';

export interface EmptyStateProps {
  icon?: ReactNode;
  title: string;
  description?: string;
  action?: ReactNode;
  className?: string;
}

export const EmptyState = ({
  icon,
  title,
  description,
  action,
  className = '',
}: EmptyStateProps) => {
  return (
    <div className={`empty-state ${className}`}>
      {icon && <div className="empty-state-icon">{icon}</div>}

      <div className="empty-state-content">
        <h3 className="empty-state-title">{title}</h3>
        {description && <p className="empty-state-description">{description}</p>}
      </div>

      {action && <div className="empty-state-action">{action}</div>}
    </div>
  );
};
