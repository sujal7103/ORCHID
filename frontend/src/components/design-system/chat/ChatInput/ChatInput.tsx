import { useState, useRef, forwardRef } from 'react';
import type { HTMLAttributes, KeyboardEvent, ChangeEvent } from 'react';
import './ChatInput.css';

export interface ChatInputProps extends Omit<HTMLAttributes<HTMLDivElement>, 'onSubmit'> {
  onSubmit: (message: string, files?: File[]) => void;
  placeholder?: string;
  disabled?: boolean;
  maxLength?: number;
  showFileUpload?: boolean;
  acceptFiles?: string;
}

export const ChatInput = forwardRef<HTMLDivElement, ChatInputProps>(
  (
    {
      onSubmit,
      placeholder = 'Type a message...',
      disabled = false,
      maxLength = 4000,
      showFileUpload = true,
      acceptFiles = 'image/*,.pdf,.doc,.docx,.txt',
      className = '',
      ...props
    },
    ref
  ) => {
    const [message, setMessage] = useState('');
    const [files, setFiles] = useState<File[]>([]);
    const textareaRef = useRef<HTMLTextAreaElement>(null);
    const fileInputRef = useRef<HTMLInputElement>(null);

    const handleSubmit = () => {
      const trimmedMessage = message.trim();
      if (trimmedMessage || files.length > 0) {
        onSubmit(trimmedMessage, files.length > 0 ? files : undefined);
        setMessage('');
        setFiles([]);
        if (textareaRef.current) {
          textareaRef.current.style.height = 'auto';
        }
      }
    };

    const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        handleSubmit();
      }
    };

    const handleChange = (e: ChangeEvent<HTMLTextAreaElement>) => {
      setMessage(e.target.value);
      // Auto-resize textarea
      if (textareaRef.current) {
        textareaRef.current.style.height = 'auto';
        textareaRef.current.style.height = `${Math.min(textareaRef.current.scrollHeight, 200)}px`;
      }
    };

    const handleFileChange = (e: ChangeEvent<HTMLInputElement>) => {
      if (e.target.files) {
        setFiles(Array.from(e.target.files));
      }
    };

    const removeFile = (index: number) => {
      setFiles(files.filter((_, i) => i !== index));
    };

    const triggerFileInput = () => {
      fileInputRef.current?.click();
    };

    const charCount = message.length;
    const isOverLimit = charCount > maxLength;
    const canSend = (message.trim() || files.length > 0) && !disabled && !isOverLimit;

    return (
      <div ref={ref} className={`chat-input ${className}`} {...props}>
        {files.length > 0 && (
          <div className="chat-input-files">
            {files.map((file, index) => (
              <div key={index} className="chat-input-file">
                <span className="chat-input-file-name">{file.name}</span>
                <button
                  type="button"
                  onClick={() => removeFile(index)}
                  className="chat-input-file-remove"
                  aria-label="Remove file"
                >
                  ×
                </button>
              </div>
            ))}
          </div>
        )}
        <div className="chat-input-wrapper">
          {showFileUpload && (
            <>
              <button
                type="button"
                onClick={triggerFileInput}
                className="chat-input-btn chat-input-attach-btn"
                disabled={disabled}
                aria-label="Attach file"
              >
                <svg
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                >
                  <path d="M21.44 11.05l-9.19 9.19a6 6 0 0 1-8.49-8.49l9.19-9.19a4 4 0 0 1 5.66 5.66l-9.2 9.19a2 2 0 0 1-2.83-2.83l8.49-8.48" />
                </svg>
              </button>
              <input
                ref={fileInputRef}
                type="file"
                onChange={handleFileChange}
                accept={acceptFiles}
                multiple
                className="chat-input-file-input"
              />
            </>
          )}
          <textarea
            ref={textareaRef}
            value={message}
            onChange={handleChange}
            onKeyDown={handleKeyDown}
            placeholder={placeholder}
            disabled={disabled}
            className="chat-input-textarea"
            rows={1}
            aria-label="Message input"
          />
          <button
            type="button"
            onClick={handleSubmit}
            disabled={!canSend}
            className={`chat-input-btn chat-input-send-btn ${canSend ? 'chat-input-send-btn-active' : ''}`}
            aria-label="Send message"
          >
            <svg
              width="20"
              height="20"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
            >
              <line x1="22" y1="2" x2="11" y2="13" />
              <polygon points="22 2 15 22 11 13 2 9 22 2" />
            </svg>
          </button>
        </div>
        {maxLength && (
          <div className="chat-input-footer">
            <span
              className={`chat-input-char-count ${isOverLimit ? 'chat-input-char-count-error' : ''}`}
            >
              {charCount}/{maxLength}
            </span>
            <span className="chat-input-hint">Press Enter to send, Shift+Enter for new line</span>
          </div>
        )}
      </div>
    );
  }
);

ChatInput.displayName = 'ChatInput';
