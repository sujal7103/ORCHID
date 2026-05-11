import { useState, useRef, useEffect, type ReactNode } from 'react';
import './DropdownMenu.css';

export interface DropdownMenuItem {
  label: string;
  icon?: ReactNode;
  onClick?: () => void;
  disabled?: boolean;
  divider?: boolean;
  danger?: boolean;
}

export interface DropdownMenuProps {
  trigger: ReactNode;
  items: DropdownMenuItem[];
  align?: 'left' | 'right';
  className?: string;
}

export const DropdownMenu = ({
  trigger,
  items,
  align = 'left',
  className = '',
}: DropdownMenuProps) => {
  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setIsOpen(false);
      }
    };

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
      document.addEventListener('keydown', handleEscape);
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [isOpen]);

  const handleItemClick = (item: DropdownMenuItem) => {
    if (!item.disabled && item.onClick) {
      item.onClick();
      setIsOpen(false);
    }
  };

  return (
    <div className={`dropdown-menu-container ${className}`} ref={dropdownRef}>
      <div className="dropdown-menu-trigger" onClick={() => setIsOpen(!isOpen)}>
        {trigger}
      </div>

      {isOpen && (
        <div className={`dropdown-menu-content dropdown-menu-${align}`}>
          {items.map((item, index) => (
            <div key={index}>
              {item.divider ? (
                <div className="dropdown-menu-divider" />
              ) : (
                <div
                  className={`dropdown-menu-item ${
                    item.disabled ? 'dropdown-menu-item-disabled' : ''
                  } ${item.danger ? 'dropdown-menu-item-danger' : ''}`}
                  onClick={() => handleItemClick(item)}
                >
                  {item.icon && <span className="dropdown-menu-item-icon">{item.icon}</span>}
                  <span className="dropdown-menu-item-label">{item.label}</span>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
