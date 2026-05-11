import type { ChangeEvent } from 'react';
import './Radio.css';

export interface RadioOption {
  label: string;
  value: string;
  disabled?: boolean;
  helperText?: string;
}

export interface RadioProps {
  label?: string;
  value: string;
  checked?: boolean;
  disabled?: boolean;
  helperText?: string;
  onChange?: (value: string) => void;
  name?: string;
  className?: string;
}

export interface RadioGroupProps {
  label?: string;
  options: RadioOption[];
  value?: string;
  defaultValue?: string;
  disabled?: boolean;
  error?: string;
  helperText?: string;
  required?: boolean;
  onChange?: (value: string) => void;
  className?: string;
  name: string;
  orientation?: 'vertical' | 'horizontal';
}

export const Radio = ({
  label,
  value,
  checked,
  disabled = false,
  helperText,
  onChange,
  name,
  className = '',
}: RadioProps) => {
  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    if (!disabled && e.target.checked) {
      onChange?.(value);
    }
  };

  return (
    <div className={`radio-wrapper ${className}`}>
      <label className={`radio-label ${disabled ? 'radio-label-disabled' : ''}`}>
        <input
          type="radio"
          className="radio-input"
          value={value}
          checked={checked}
          disabled={disabled}
          onChange={handleChange}
          name={name}
        />
        <span className="radio-box">
          <span className="radio-dot" />
        </span>
        {label && <span className="radio-label-text">{label}</span>}
      </label>

      {helperText && <span className="radio-helper-text">{helperText}</span>}
    </div>
  );
};

export const RadioGroup = ({
  label,
  options,
  value,
  defaultValue,
  disabled = false,
  error,
  helperText,
  required = false,
  onChange,
  className = '',
  name,
  orientation = 'vertical',
}: RadioGroupProps) => {
  const selectedValue = value || defaultValue || '';

  return (
    <div className={`radio-group ${className}`}>
      {label && (
        <label className="radio-group-label">
          {label}
          {required && <span className="radio-required">*</span>}
        </label>
      )}

      <div className={`radio-group-options radio-group-${orientation}`}>
        {options.map(option => (
          <Radio
            key={option.value}
            label={option.label}
            value={option.value}
            checked={selectedValue === option.value}
            disabled={disabled || option.disabled}
            helperText={option.helperText}
            onChange={onChange}
            name={name}
          />
        ))}
      </div>

      {(error || helperText) && (
        <span className={`radio-group-helper-text ${error ? 'radio-group-helper-error' : ''}`}>
          {error || helperText}
        </span>
      )}
    </div>
  );
};
