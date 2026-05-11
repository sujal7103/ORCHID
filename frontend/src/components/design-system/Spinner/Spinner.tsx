import './Spinner.css';

export type SpinnerSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl';
export type SpinnerVariant = 'default' | 'accent' | 'success' | 'warning' | 'error';

export interface SpinnerProps {
  size?: SpinnerSize;
  variant?: SpinnerVariant;
  label?: string;
  className?: string;
}

export const Spinner = ({
  size = 'md',
  variant = 'accent',
  label,
  className = '',
}: SpinnerProps) => {
  return (
    <div className={`spinner-wrapper ${className}`} role="status" aria-label={label || 'Loading'}>
      <div className={`spinner spinner-${size} spinner-${variant}`}>
        <div className="spinner-circle" />
      </div>
      {label && <span className="spinner-label">{label}</span>}
    </div>
  );
};
