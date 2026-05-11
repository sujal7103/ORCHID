import type { ReactNode } from 'react';
import './Breadcrumb.css';

export interface BreadcrumbItem {
  label: string;
  href?: string;
  icon?: ReactNode;
  onClick?: () => void;
}

export interface BreadcrumbProps {
  items: BreadcrumbItem[];
  separator?: ReactNode;
  className?: string;
}

const DefaultSeparator = () => (
  <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
    <path
      d="M6 4L10 8L6 12"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  </svg>
);

export const Breadcrumb = ({
  items,
  separator = <DefaultSeparator />,
  className = '',
}: BreadcrumbProps) => {
  return (
    <nav className={`breadcrumb ${className}`} aria-label="Breadcrumb">
      <ol className="breadcrumb-list">
        {items.map((item, index) => {
          const isLast = index === items.length - 1;

          return (
            <li key={index} className="breadcrumb-item">
              {item.href || item.onClick ? (
                <a
                  href={item.href}
                  onClick={e => {
                    if (item.onClick) {
                      e.preventDefault();
                      item.onClick();
                    }
                  }}
                  className={`breadcrumb-link ${isLast ? 'breadcrumb-link-current' : ''}`}
                  aria-current={isLast ? 'page' : undefined}
                >
                  {item.icon && <span className="breadcrumb-icon">{item.icon}</span>}
                  <span>{item.label}</span>
                </a>
              ) : (
                <span
                  className={`breadcrumb-text ${isLast ? 'breadcrumb-text-current' : ''}`}
                  aria-current={isLast ? 'page' : undefined}
                >
                  {item.icon && <span className="breadcrumb-icon">{item.icon}</span>}
                  <span>{item.label}</span>
                </span>
              )}

              {!isLast && <span className="breadcrumb-separator">{separator}</span>}
            </li>
          );
        })}
      </ol>
    </nav>
  );
};
