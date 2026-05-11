import type { ChangeEvent } from 'react';
import './Checkbox.css';

export interface CheckboxProps {
  label?: string;
  checked?: boolean;
  defaultChecked?: boolean;
  disabled?: boolean;
  indeterminate?: boolean;
  error?: string;
  helperText?: string;
  required?: boolean;
  onChange?: (checked: boolean) => void;
  className?: string;
  name?: string;
  value?: string;
}

export const Checkbox = ({
  label,
  checked,
  defaultChecked,
  disabled = false,
  indeterminate = false,
  error,
  helperText,
  required = false,
  onChange,
  className = '',
  name,
  value,
}: CheckboxProps) => {
  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    if (!disabled) {
      onChange?.(e.target.checked);
    }
  };

  return (
    <div className={`checkbox-wrapper ${className}`}>
      <label className={`checkbox-label ${disabled ? 'checkbox-label-disabled' : ''}`}>
        <input
          type="checkbox"
          className="checkbox-input"
          checked={checked}
          defaultChecked={defaultChecked}
          disabled={disabled}
          onChange={handleChange}
          name={name}
          value={value}
          aria-invalid={!!error}
          aria-required={required}
        />
        <span
          className={`checkbox-box ${error ? 'checkbox-box-error' : ''} ${
            indeterminate ? 'checkbox-box-indeterminate' : ''
          }`}
        >
          {indeterminate ? (
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
              <path d="M3 7H11" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
            </svg>
          ) : (
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none">
              <path
                d="M11.6667 3.5L5.25 9.91667L2.33333 7"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          )}
        </span>
        {label && (
          <span className="checkbox-label-text">
            {label}
            {required && <span className="checkbox-required">*</span>}
          </span>
        )}
      </label>

      {(error || helperText) && (
        <span className={`checkbox-helper-text ${error ? 'checkbox-helper-error' : ''}`}>
          {error || helperText}
        </span>
      )}
    </div>
  );
};
