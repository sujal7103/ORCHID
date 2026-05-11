import { forwardRef } from 'react';
import type { HTMLAttributes, ReactNode } from 'react';
import './Card.css';

export type CardVariant = 'glass' | 'feature' | 'widget';

export interface CardProps extends HTMLAttributes<HTMLDivElement> {
  variant?: CardVariant;
  hoverable?: boolean;
  icon?: ReactNode;
  title?: string;
  description?: string;
}

export const Card = forwardRef<HTMLDivElement, CardProps>(
  (
    {
      variant = 'glass',
      hoverable = true,
      icon,
      title,
      description,
      className = '',
      children,
      ...props
    },
    ref
  ) => {
    const classes = ['card', `card-${variant}`, hoverable && 'card-hoverable', className]
      .filter(Boolean)
      .join(' ');

    return (
      <div ref={ref} className={classes} {...props}>
        {icon && <div className="card-icon">{icon}</div>}
        {title && <h3 className="card-title">{title}</h3>}
        {description && <p className="card-description">{description}</p>}
        {children}
      </div>
    );
  }
);

Card.displayName = 'Card';
