import { useState, useRef, useEffect } from 'react';
import './Select.css';

export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export interface SelectProps {
  label?: string;
  options: SelectOption[];
  value?: string;
  defaultValue?: string;
  placeholder?: string;
  disabled?: boolean;
  error?: string;
  helperText?: string;
  required?: boolean;
  fullWidth?: boolean;
  onChange?: (value: string) => void;
  className?: string;
}

export const Select = ({
  label,
  options,
  value,
  defaultValue,
  placeholder = 'Select an option...',
  disabled = false,
  error,
  helperText,
  required = false,
  fullWidth = false,
  onChange,
  className = '',
}: SelectProps) => {
  const [selectedValue, setSelectedValue] = useState(value || defaultValue || '');
  const [isOpen, setIsOpen] = useState(false);
  const selectRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (value !== undefined) {
      setSelectedValue(value);
    }
  }, [value]);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (selectRef.current && !selectRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const handleSelect = (optionValue: string) => {
    if (disabled) return;
    setSelectedValue(optionValue);
    setIsOpen(false);
    onChange?.(optionValue);
  };

  const selectedOption = options.find(opt => opt.value === selectedValue);

  return (
    <div
      className={`select-wrapper ${fullWidth ? 'select-full-width' : ''} ${className}`}
      ref={selectRef}
    >
      {label && (
        <label className="select-label">
          {label}
          {required && <span className="select-required">*</span>}
        </label>
      )}

      <div
        className={`select-control ${isOpen ? 'select-control-open' : ''} ${
          error ? 'select-control-error' : ''
        } ${disabled ? 'select-control-disabled' : ''}`}
        onClick={() => !disabled && setIsOpen(!isOpen)}
      >
        <span className={`select-value ${!selectedValue ? 'select-placeholder' : ''}`}>
          {selectedOption?.label || placeholder}
        </span>
        <svg
          className={`select-arrow ${isOpen ? 'select-arrow-open' : ''}`}
          width="16"
          height="16"
          viewBox="0 0 16 16"
          fill="none"
        >
          <path
            d="M4 6L8 10L12 6"
            stroke="currentColor"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </div>

      {isOpen && !disabled && (
        <div className="select-dropdown">
          {options.map(option => (
            <div
              key={option.value}
              className={`select-option ${
                option.value === selectedValue ? 'select-option-selected' : ''
              } ${option.disabled ? 'select-option-disabled' : ''}`}
              onClick={() => !option.disabled && handleSelect(option.value)}
            >
              {option.label}
              {option.value === selectedValue && (
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                  <path
                    d="M13 4L6 11L3 8"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
              )}
            </div>
          ))}
        </div>
      )}

      {(error || helperText) && (
        <span className={`select-helper-text ${error ? 'select-helper-error' : ''}`}>
          {error || helperText}
        </span>
      )}
    </div>
  );
};
