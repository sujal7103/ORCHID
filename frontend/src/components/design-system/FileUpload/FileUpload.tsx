import { useRef, useState, type DragEvent, type ChangeEvent } from 'react';
import './FileUpload.css';

export interface FileUploadProps {
  accept?: string;
  multiple?: boolean;
  maxSize?: number; // in bytes
  disabled?: boolean;
  onUpload?: (files: File[]) => void;
  className?: string;
}

export const FileUpload = ({
  accept,
  multiple = false,
  maxSize,
  disabled = false,
  onUpload,
  className = '',
}: FileUploadProps) => {
  const [isDragging, setIsDragging] = useState(false);
  const [error, setError] = useState<string>('');
  const [files, setFiles] = useState<File[]>([]);
  const inputRef = useRef<HTMLInputElement>(null);

  const validateFiles = (fileList: FileList): File[] => {
    setError('');
    const validFiles: File[] = [];

    Array.from(fileList).forEach(file => {
      if (maxSize && file.size > maxSize) {
        setError(
          `File "${file.name}" exceeds maximum size of ${(maxSize / 1024 / 1024).toFixed(2)}MB`
        );
        return;
      }
      validFiles.push(file);
    });

    return validFiles;
  };

  const handleFiles = (fileList: FileList | null) => {
    if (!fileList || disabled) return;

    const validFiles = validateFiles(fileList);
    if (validFiles.length > 0) {
      setFiles(validFiles);
      onUpload?.(validFiles);
    }
  };

  const handleDragOver = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    if (!disabled) setIsDragging(true);
  };

  const handleDragLeave = () => {
    setIsDragging(false);
  };

  const handleDrop = (e: DragEvent<HTMLDivElement>) => {
    e.preventDefault();
    setIsDragging(false);
    handleFiles(e.dataTransfer.files);
  };

  const handleChange = (e: ChangeEvent<HTMLInputElement>) => {
    handleFiles(e.target.files);
  };

  const handleClick = () => {
    inputRef.current?.click();
  };

  const handleRemove = (index: number) => {
    setFiles(prev => prev.filter((_, i) => i !== index));
  };

  return (
    <div className={`file-upload ${className}`}>
      <div
        className={`file-upload-dropzone ${isDragging ? 'file-upload-dropzone-active' : ''} ${
          disabled ? 'file-upload-dropzone-disabled' : ''
        } ${error ? 'file-upload-dropzone-error' : ''}`}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
        onClick={handleClick}
      >
        <svg width="48" height="48" viewBox="0 0 48 48" fill="none" className="file-upload-icon">
          <path
            d="M24 32V16M24 16L18 22M24 16L30 22"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <path
            d="M8 28V36C8 38.2091 9.79086 40 12 40H36C38.2091 40 40 38.2091 40 36V28"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
          />
        </svg>

        <div className="file-upload-text">
          <p className="file-upload-title">Drop files here or click to browse</p>
          <p className="file-upload-subtitle">
            {accept ? `Accepted: ${accept}` : 'All file types accepted'}
            {maxSize && ` • Max size: ${(maxSize / 1024 / 1024).toFixed(0)}MB`}
          </p>
        </div>

        <input
          ref={inputRef}
          type="file"
          className="file-upload-input"
          accept={accept}
          multiple={multiple}
          disabled={disabled}
          onChange={handleChange}
        />
      </div>

      {error && <p className="file-upload-error">{error}</p>}

      {files.length > 0 && (
        <div className="file-upload-list">
          {files.map((file, index) => (
            <div key={index} className="file-upload-item">
              <svg width="20" height="20" viewBox="0 0 20 20" fill="none">
                <path
                  d="M11 2H5C4.44772 2 4 2.44772 4 3V17C4 17.5523 4.44772 18 5 18H15C15.5523 18 16 17.5523 16 17V8L11 2Z"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinejoin="round"
                />
                <path
                  d="M11 2V8H16"
                  stroke="currentColor"
                  strokeWidth="1.5"
                  strokeLinejoin="round"
                />
              </svg>

              <div className="file-upload-item-info">
                <span className="file-upload-item-name">{file.name}</span>
                <span className="file-upload-item-size">{(file.size / 1024).toFixed(2)} KB</span>
              </div>

              <button
                className="file-upload-item-remove"
                onClick={() => handleRemove(index)}
                aria-label="Remove file"
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
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
