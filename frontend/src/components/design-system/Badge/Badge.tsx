import { forwardRef } from 'react';
import type { HTMLAttributes, ReactNode } from 'react';
import './Badge.css';

export type BadgeVariant = 'default' | 'accent' | 'success' | 'warning' | 'error' | 'info';

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  variant?: BadgeVariant;
  dot?: boolean;
  icon?: ReactNode;
}

export const Badge = forwardRef<HTMLSpanElement, BadgeProps>(
  ({ variant = 'default', dot = false, icon, className = '', children, ...props }, ref) => {
    const classes = ['badge', `badge-${variant}`, className].filter(Boolean).join(' ');

    return (
      <span ref={ref} className={classes} {...props}>
        {dot && <span className="badge-dot" aria-hidden="true" />}
        {icon && (
          <span className="badge-icon" aria-hidden="true">
            {icon}
          </span>
        )}
        {children}
      </span>
    );
  }
);

Badge.displayName = 'Badge';
