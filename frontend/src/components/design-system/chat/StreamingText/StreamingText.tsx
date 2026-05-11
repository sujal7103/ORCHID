import { useState, useEffect, forwardRef } from 'react';
import type { HTMLAttributes } from 'react';
import './StreamingText.css';

export interface StreamingTextProps extends Omit<HTMLAttributes<HTMLDivElement>, 'children'> {
  text: string;
  speed?: number;
  onComplete?: () => void;
  showCursor?: boolean;
}

export const StreamingText = forwardRef<HTMLDivElement, StreamingTextProps>(
  ({ text, speed = 30, onComplete, showCursor = true, className = '', ...props }, ref) => {
    const [displayedText, setDisplayedText] = useState('');
    const [currentIndex, setCurrentIndex] = useState(0);

    useEffect(() => {
      if (currentIndex < text.length) {
        const timeout = setTimeout(() => {
          setDisplayedText(prev => prev + text[currentIndex]);
          setCurrentIndex(prev => prev + 1);
        }, speed);

        return () => clearTimeout(timeout);
      } else if (currentIndex === text.length && onComplete) {
        onComplete();
      }
    }, [currentIndex, text, speed, onComplete]);

    useEffect(() => {
      setDisplayedText('');
      setCurrentIndex(0);
    }, [text]);

    return (
      <div ref={ref} className={`streaming-text ${className}`} {...props}>
        {displayedText}
        {showCursor && currentIndex < text.length && (
          <span className="streaming-text-cursor">▊</span>
        )}
      </div>
    );
  }
);

StreamingText.displayName = 'StreamingText';
