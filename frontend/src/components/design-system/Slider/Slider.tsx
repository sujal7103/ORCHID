import type { ChangeEvent } from 'react';
import './Slider.css';

export interface SliderProps {
  value?: number;
  min?: number;
  max?: number;
  step?: number;
  label?: string;
  showValue?: boolean;
  disabled?: boolean;
  onChange?: (value: number) => void;
  className?: string;
}

export const Slider = ({
  value = 50,
  min = 0,
  max = 100,
  step = 1,
  label,
  showValue = true,
  disabled = false,
  onChange,
  className = '',
}: SliderProps) => {
  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    onChange?.(Number(e.target.value));
  };

  const percentage = ((value - min) / (max - min)) * 100;

  return (
    <div className={`slider-wrapper ${className}`}>
      {(label || showValue) && (
        <div className="slider-header">
          {label && <label className="slider-label">{label}</label>}
          {showValue && <span className="slider-value">{value}</span>}
        </div>
      )}

      <div className="slider-track-wrapper">
        <input
          type="range"
          className="slider-input"
          value={value}
          min={min}
          max={max}
          step={step}
          disabled={disabled}
          onChange={handleChange}
          style={
            {
              '--slider-percentage': `${percentage}%`,
            } as React.CSSProperties
          }
        />
      </div>
    </div>
  );
};
