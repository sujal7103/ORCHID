import { forwardRef } from 'react';
import type { ButtonHTMLAttributes } from 'react';
import './Button.css';

export type ButtonVariant = 'primary' | 'secondary' | 'ghost';
export type ButtonSize = 'sm' | 'md' | 'lg' | 'xl';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  fullWidth?: boolean;
  loading?: boolean;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      variant = 'primary',
      size = 'md',
      fullWidth = false,
      loading = false,
      className = '',
      children,
      disabled,
      ...props
    },
    ref
  ) => {
    const classes = [
      'btn',
      `btn-${variant}`,
      `btn-${size}`,
      fullWidth && 'btn-full-width',
      loading && 'btn-loading',
      className,
    ]
      .filter(Boolean)
      .join(' ');

    return (
      <button ref={ref} className={classes} disabled={disabled || loading} {...props}>
        {loading && (
          <span className="btn-spinner" aria-hidden="true">
            <svg
              className="btn-spinner-icon"
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
            >
              <circle
                className="btn-spinner-circle"
                cx="12"
                cy="12"
                r="10"
                stroke="currentColor"
                strokeWidth="4"
              />
              <path
                className="btn-spinner-path"
                fill="currentColor"
                d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
              />
            </svg>
          </span>
        )}
        <span className={loading ? 'btn-content-loading' : ''}>{children}</span>
      </button>
    );
  }
);

Button.displayName = 'Button';
