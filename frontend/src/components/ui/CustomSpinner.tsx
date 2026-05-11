import React from 'react';
import spinnerGif from '@/assets/spinner.gif';

interface CustomSpinnerProps {
  size?: number;
  className?: string;
}

export const CustomSpinner: React.FC<CustomSpinnerProps> = ({ size = 16, className = '' }) => {
  return (
    <img
      src={spinnerGif}
      alt="Loading..."
      style={{
        width: `${size * 1.5}px`,
        height: `${size * 1.5}px`,
        objectFit: 'cover',
        border: 'none',
        outline: 'none',
        transform: 'scaleX(-1)',
      }}
      className={className}
    />
  );
};
