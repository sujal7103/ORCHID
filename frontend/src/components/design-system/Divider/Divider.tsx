import type { ReactNode } from 'react';
import './Divider.css';

export type DividerOrientation = 'horizontal' | 'vertical';

export interface DividerProps {
  orientation?: DividerOrientation;
  label?: ReactNode;
  className?: string;
}

export const Divider = ({ orientation = 'horizontal', label, className = '' }: DividerProps) => {
  if (label) {
    return (
      <div className={`divider-with-label ${className}`}>
        <div className="divider-line" />
        <span className="divider-label">{label}</span>
        <div className="divider-line" />
      </div>
    );
  }

  return (
    <div
      className={`divider divider-${orientation} ${className}`}
      role="separator"
      aria-orientation={orientation}
    />
  );
};
