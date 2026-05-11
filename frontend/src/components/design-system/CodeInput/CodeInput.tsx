import { useRef, Fragment, type KeyboardEvent, type ClipboardEvent } from 'react';
import './CodeInput.css';

export interface CodeInputProps {
  /** Number of input boxes (default 8) */
  length?: number;
  /** Current value */
  value: string;
  /** Called when value changes */
  onChange: (value: string) => void;
  /** Called when all boxes are filled */
  onComplete?: (value: string) => void;
  /** Disable all inputs */
  disabled?: boolean;
  /** Auto-focus first input on mount */
  autoFocus?: boolean;
  /** Add visual separator after N characters (default 4 for XXXX-XXXX format) */
  separator?: number;
  /** Show error state with shake animation */
  error?: boolean;
  /** Additional CSS class */
  className?: string;
}

export const CodeInput = ({
  length = 8,
  value,
  onChange,
  onComplete,
  disabled = false,
  autoFocus = false,
  separator = 4,
  error = false,
  className = '',
}: CodeInputProps) => {
  const inputRefs = useRef<(HTMLInputElement | null)[]>([]);

  // Split value into array, pad with empty strings
  const chars = value
    .toUpperCase()
    .replace(/[^A-Z0-9]/g, '')
    .split('')
    .slice(0, length);
  while (chars.length < length) chars.push('');

  const handleChange = (index: number, newChar: string) => {
    const char = newChar.toUpperCase().replace(/[^A-Z0-9]/g, '');
    if (!char) return;

    const newChars = [...chars];
    newChars[index] = char[0];
    const newValue = newChars.join('');
    onChange(newValue);

    // Auto-advance to next input
    if (index < length - 1) {
      inputRefs.current[index + 1]?.focus();
    }

    // Check if complete (excluding empty chars)
    if (newValue.replace(/\s/g, '').length === length && onComplete) {
      onComplete(newValue);
    }
  };

  const handleKeyDown = (index: number, e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Backspace') {
      e.preventDefault();
      const newChars = [...chars];

      if (chars[index]) {
        // Clear current box
        newChars[index] = '';
        onChange(newChars.join(''));
      } else if (index > 0) {
        // Move to previous box and clear it
        newChars[index - 1] = '';
        onChange(newChars.join(''));
        inputRefs.current[index - 1]?.focus();
      }
    } else if (e.key === 'ArrowLeft' && index > 0) {
      e.preventDefault();
      inputRefs.current[index - 1]?.focus();
    } else if (e.key === 'ArrowRight' && index < length - 1) {
      e.preventDefault();
      inputRefs.current[index + 1]?.focus();
    } else if (e.key === 'Delete') {
      e.preventDefault();
      const newChars = [...chars];
      newChars[index] = '';
      onChange(newChars.join(''));
    }
  };

  const handlePaste = (e: ClipboardEvent<HTMLInputElement>) => {
    e.preventDefault();
    const pasted = e.clipboardData
      .getData('text')
      .toUpperCase()
      .replace(/[^A-Z0-9]/g, '');
    const newValue = pasted.slice(0, length);
    onChange(newValue);

    // Focus the next empty input or last input
    const focusIndex = Math.min(newValue.length, length - 1);
    inputRefs.current[focusIndex]?.focus();

    if (newValue.length === length && onComplete) {
      onComplete(newValue);
    }
  };

  const handleFocus = (e: React.FocusEvent<HTMLInputElement>) => {
    // Select content on focus for easy replacement
    e.target.select();
  };

  return (
    <div className={`code-input ${error ? 'code-input-error' : ''} ${className}`}>
      <div className="code-input-boxes">
        {chars.map((char, i) => (
          <Fragment key={i}>
            {separator && i === separator && <span className="code-input-separator">-</span>}
            <input
              ref={el => (inputRefs.current[i] = el)}
              type="text"
              inputMode="text"
              maxLength={1}
              value={char}
              disabled={disabled}
              autoFocus={autoFocus && i === 0}
              onChange={e => handleChange(i, e.target.value)}
              onKeyDown={e => handleKeyDown(i, e)}
              onPaste={handlePaste}
              onFocus={handleFocus}
              className={`code-input-box ${char ? 'filled' : ''}`}
              aria-label={`Character ${i + 1} of ${length}`}
            />
          </Fragment>
        ))}
      </div>
    </div>
  );
};
