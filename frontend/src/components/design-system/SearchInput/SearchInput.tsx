import { useState, type ChangeEvent } from 'react';
import './SearchInput.css';

export interface SearchInputProps {
  value?: string;
  placeholder?: string;
  onSearch?: (value: string) => void;
  onChange?: (value: string) => void;
  onClear?: () => void;
  disabled?: boolean;
  loading?: boolean;
  className?: string;
}

export const SearchInput = ({
  value: controlledValue,
  placeholder = 'Search...',
  onSearch,
  onChange,
  onClear,
  disabled = false,
  loading = false,
  className = '',
}: SearchInputProps) => {
  const [value, setValue] = useState(controlledValue || '');

  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    const newValue = e.target.value;
    setValue(newValue);
    onChange?.(newValue);
  };

  const handleClear = () => {
    setValue('');
    onChange?.('');
    onClear?.();
  };

  const handleKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' && onSearch) {
      onSearch(value);
    }
  };

  const displayValue = controlledValue !== undefined ? controlledValue : value;

  return (
    <div className={`search-input-wrapper ${className}`}>
      <svg className="search-input-icon" width="18" height="18" viewBox="0 0 18 18" fill="none">
        <circle cx="8" cy="8" r="5.5" stroke="currentColor" strokeWidth="1.5" />
        <path d="M12 12L16 16" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
      </svg>

      <input
        type="text"
        className="search-input"
        value={displayValue}
        onChange={handleChange}
        onKeyPress={handleKeyPress}
        placeholder={placeholder}
        disabled={disabled || loading}
      />

      {loading && (
        <div className="search-input-loading">
          <div className="search-input-spinner" />
        </div>
      )}

      {displayValue && !loading && (
        <button
          className="search-input-clear"
          onClick={handleClear}
          aria-label="Clear search"
          type="button"
        >
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
            <path
              d="M12 4L4 12M4 4L12 12"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
            />
          </svg>
        </button>
      )}
    </div>
  );
};
