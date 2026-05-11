import { forwardRef } from 'react';
import type { TextareaHTMLAttributes } from 'react';
import './Input.css';

export interface TextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  label?: string;
  error?: string;
  helperText?: string;
}

export const Textarea = forwardRef<HTMLTextAreaElement, TextareaProps>(
  ({ label, error, helperText, className = '', id, ...props }, ref) => {
    const textareaId = id || `textarea-${Math.random().toString(36).substr(2, 9)}`;
    const hasError = !!error;

    return (
      <div className="input-group">
        {label && (
          <label htmlFor={textareaId} className="input-label">
            {label}
          </label>
        )}
        <textarea
          ref={ref}
          id={textareaId}
          className={`textarea ${hasError ? 'input-error' : ''} ${className}`}
          aria-invalid={hasError}
          aria-describedby={
            error ? `${textareaId}-error` : helperText ? `${textareaId}-helper` : undefined
          }
          {...props}
        />
        {error && (
          <span id={`${textareaId}-error`} className="input-error-text">
            {error}
          </span>
        )}
        {!error && helperText && (
          <span id={`${textareaId}-helper`} className="input-helper-text">
            {helperText}
          </span>
        )}
      </div>
    );
  }
);

Textarea.displayName = 'Textarea';
