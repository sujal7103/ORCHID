import { forwardRef } from 'react';
import type { HTMLAttributes } from 'react';
import './Progress.css';

export interface ProgressProps extends Omit<HTMLAttributes<HTMLDivElement>, 'children'> {
  value: number;
  max?: number;
  showLabel?: boolean;
  label?: string;
}

export const Progress = forwardRef<HTMLDivElement, ProgressProps>(
  ({ value, max = 100, showLabel = false, label, className = '', ...props }, ref) => {
    const percentage = Math.min(Math.max((value / max) * 100, 0), 100);
    const displayLabel = label || `${Math.round(percentage)}%`;

    return (
      <div ref={ref} className={`progress-container ${className}`} {...props}>
        {showLabel && (
          <div className="progress-label">
            <span className="progress-label-text">{displayLabel}</span>
            <span className="progress-label-value">{Math.round(percentage)}%</span>
          </div>
        )}
        <div
          className="progress-bar"
          role="progressbar"
          aria-valuenow={value}
          aria-valuemin={0}
          aria-valuemax={max}
          aria-label={label}
        >
          <div className="progress-fill" style={{ width: `${percentage}%` }} />
        </div>
      </div>
    );
  }
);

Progress.displayName = 'Progress';
