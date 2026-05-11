import { useState } from 'react';
import './Avatar.css';

export type AvatarSize = 'xs' | 'sm' | 'md' | 'lg' | 'xl' | '2xl';

export interface AvatarProps {
  src?: string;
  alt?: string;
  name?: string;
  size?: AvatarSize;
  fallback?: string;
  status?: 'online' | 'offline' | 'busy' | 'away';
  className?: string;
}

const getInitials = (name: string): string => {
  const parts = name.trim().split(' ');
  if (parts.length >= 2) {
    return `${parts[0][0]}${parts[parts.length - 1][0]}`.toUpperCase();
  }
  return name.slice(0, 2).toUpperCase();
};

export const Avatar = ({
  src,
  alt,
  name,
  size = 'md',
  fallback,
  status,
  className = '',
}: AvatarProps) => {
  const [imageError, setImageError] = useState(false);

  const displayInitials = name ? getInitials(name) : fallback || '?';
  const showImage = src && !imageError;

  return (
    <div className={`avatar-container ${className}`}>
      <div className={`avatar avatar-${size}`}>
        {showImage ? (
          <img
            src={src}
            alt={alt || name || 'Avatar'}
            className="avatar-image"
            onError={() => setImageError(true)}
          />
        ) : (
          <span className="avatar-initials">{displayInitials}</span>
        )}
      </div>
      {status && <span className={`avatar-status avatar-status-${status} avatar-status-${size}`} />}
    </div>
  );
};
