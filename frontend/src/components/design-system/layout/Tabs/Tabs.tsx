import { forwardRef } from 'react';
import type { HTMLAttributes, ReactNode } from 'react';
import './Tabs.css';

export interface Tab {
  id: string;
  label: ReactNode;
  icon?: ReactNode;
}

export interface TabsProps extends Omit<HTMLAttributes<HTMLDivElement>, 'onChange'> {
  tabs: Tab[];
  activeTab: string;
  onChange: (tabId: string) => void;
}

export const Tabs = forwardRef<HTMLDivElement, TabsProps>(
  ({ tabs, activeTab, onChange, className = '', ...props }, ref) => {
    return (
      <div ref={ref} className={`tabs ${className}`} {...props}>
        {tabs.map(tab => (
          <button
            key={tab.id}
            type="button"
            onClick={() => onChange(tab.id)}
            className={`tab ${activeTab === tab.id ? 'tab-active' : ''}`}
            aria-selected={activeTab === tab.id}
            role="tab"
          >
            {tab.icon && <span className="tab-icon">{tab.icon}</span>}
            <span className="tab-label">{tab.label}</span>
          </button>
        ))}
      </div>
    );
  }
);

Tabs.displayName = 'Tabs';
