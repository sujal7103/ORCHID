import type { ChangeEvent } from 'react';
import './Switch.css';

export type SwitchSize = 'sm' | 'md' | 'lg';

export interface SwitchProps {
  label?: string;
  checked?: boolean;
  defaultChecked?: boolean;
  disabled?: boolean;
  size?: SwitchSize;
  helperText?: string;
  onChange?: (checked: boolean) => void;
  className?: string;
  name?: string;
}

export const Switch = ({
  label,
  checked,
  defaultChecked,
  disabled = false,
  size = 'md',
  helperText,
  onChange,
  className = '',
  name,
}: SwitchProps) => {
  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    if (!disabled) {
      onChange?.(e.target.checked);
    }
  };

  return (
    <div className={`switch-wrapper ${className}`}>
      <label className={`switch-label ${disabled ? 'switch-label-disabled' : ''}`}>
        <input
          type="checkbox"
          className="switch-input"
          checked={checked}
          defaultChecked={defaultChecked}
          disabled={disabled}
          onChange={handleChange}
          name={name}
        />
        <span className={`switch-track switch-track-${size}`}>
          <span className={`switch-thumb switch-thumb-${size}`} />
        </span>
        {label && <span className="switch-label-text">{label}</span>}
      </label>

      {helperText && <span className="switch-helper-text">{helperText}</span>}
    </div>
  );
};
