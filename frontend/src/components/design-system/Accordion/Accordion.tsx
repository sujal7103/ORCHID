import { useState, type ReactNode } from 'react';
import './Accordion.css';

export interface AccordionItemData {
  id: string;
  title: string;
  content: ReactNode;
  icon?: ReactNode;
  disabled?: boolean;
}

export interface AccordionItemProps {
  title: string;
  content: ReactNode;
  icon?: ReactNode;
  isOpen?: boolean;
  disabled?: boolean;
  onChange?: (isOpen: boolean) => void;
  className?: string;
}

export interface AccordionProps {
  items: AccordionItemData[];
  allowMultiple?: boolean;
  defaultOpenItems?: string[];
  className?: string;
}

export const AccordionItem = ({
  title,
  content,
  icon,
  isOpen = false,
  disabled = false,
  onChange,
  className = '',
}: AccordionItemProps) => {
  const [internalOpen, setInternalOpen] = useState(isOpen);
  const open = onChange ? isOpen : internalOpen;

  const handleToggle = () => {
    if (disabled) return;
    const newState = !open;
    if (onChange) {
      onChange(newState);
    } else {
      setInternalOpen(newState);
    }
  };

  return (
    <div className={`accordion-item ${disabled ? 'accordion-item-disabled' : ''} ${className}`}>
      <button
        className={`accordion-header ${open ? 'accordion-header-open' : ''}`}
        onClick={handleToggle}
        disabled={disabled}
        aria-expanded={open}
      >
        {icon && <span className="accordion-icon">{icon}</span>}
        <span className="accordion-title">{title}</span>
        <svg
          className={`accordion-chevron ${open ? 'accordion-chevron-open' : ''}`}
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
      </button>

      <div className={`accordion-content ${open ? 'accordion-content-open' : ''}`}>
        <div className="accordion-content-inner">{content}</div>
      </div>
    </div>
  );
};

export const Accordion = ({
  items,
  allowMultiple = false,
  defaultOpenItems = [],
  className = '',
}: AccordionProps) => {
  const [openItems, setOpenItems] = useState<string[]>(defaultOpenItems);

  const handleToggle = (itemId: string) => {
    if (allowMultiple) {
      setOpenItems(prev =>
        prev.includes(itemId) ? prev.filter(id => id !== itemId) : [...prev, itemId]
      );
    } else {
      setOpenItems(prev => (prev.includes(itemId) ? [] : [itemId]));
    }
  };

  return (
    <div className={`accordion ${className}`}>
      {items.map(item => (
        <AccordionItem
          key={item.id}
          title={item.title}
          content={item.content}
          icon={item.icon}
          isOpen={openItems.includes(item.id)}
          disabled={item.disabled}
          onChange={() => handleToggle(item.id)}
        />
      ))}
    </div>
  );
};
