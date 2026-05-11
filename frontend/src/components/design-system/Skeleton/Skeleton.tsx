import { forwardRef } from 'react';
import type { HTMLAttributes } from 'react';
import './Skeleton.css';

export type SkeletonVariant = 'text' | 'circular' | 'rectangular';

export interface SkeletonProps extends HTMLAttributes<HTMLDivElement> {
  variant?: SkeletonVariant;
  width?: string | number;
  height?: string | number;
  count?: number;
}

export const Skeleton = forwardRef<HTMLDivElement, SkeletonProps>(
  ({ variant = 'text', width, height, count = 1, className = '', style, ...props }, ref) => {
    const classes = ['skeleton', `skeleton-${variant}`, className].filter(Boolean).join(' ');

    const skeletonStyle = {
      ...style,
      width: typeof width === 'number' ? `${width}px` : width,
      height: typeof height === 'number' ? `${height}px` : height,
    };

    if (count === 1) {
      return (
        <div
          ref={ref}
          className={classes}
          style={skeletonStyle}
          aria-busy="true"
          aria-live="polite"
          {...props}
        />
      );
    }

    return (
      <div ref={ref} className="skeleton-group" {...props}>
        {Array.from({ length: count }).map((_, index) => (
          <div
            key={index}
            className={classes}
            style={skeletonStyle}
            aria-busy="true"
            aria-live="polite"
          />
        ))}
      </div>
    );
  }
);

Skeleton.displayName = 'Skeleton';
