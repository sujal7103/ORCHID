import { useState, forwardRef, memo } from 'react';
import type { HTMLAttributes } from 'react';
import { Prism as SyntaxHighlighter } from 'react-syntax-highlighter';
import { vscDarkPlus } from 'react-syntax-highlighter/dist/esm/styles/prism';
import './CodeBlock.css';

export interface CodeBlockProps extends Omit<HTMLAttributes<HTMLDivElement>, 'children'> {
  code: string;
  language?: string;
  showLineNumbers?: boolean;
  fileName?: string;
}

// Language display aliases (like OpenAI uses short names)
const LANGUAGE_ALIASES: Record<string, string> = {
  javascript: 'js',
  typescript: 'ts',
  python: 'py',
  markdown: 'md',
  dockerfile: 'docker',
  shellscript: 'sh',
  shell: 'sh',
  bash: 'bash',
  powershell: 'ps',
};

const getDisplayLanguage = (lang: string): string => {
  const lower = lang.toLowerCase();
  return LANGUAGE_ALIASES[lower] || lower;
};

const CodeBlockComponent = forwardRef<HTMLDivElement, CodeBlockProps>(
  (
    { code, language = 'text', showLineNumbers = false, fileName, className = '', ...props },
    ref
  ) => {
    const [copied, setCopied] = useState(false);

    const handleCopy = async () => {
      try {
        await navigator.clipboard.writeText(code);
        setCopied(true);
        setTimeout(() => setCopied(false), 2000);
      } catch (err) {
        console.error('Failed to copy:', err);
      }
    };

    const customStyle = {
      margin: 0,
      borderRadius: 0,
      background: '#1e1e1e',
      fontSize: '14px',
      lineHeight: '1.6',
    };

    const displayLang = getDisplayLanguage(language);

    return (
      <div ref={ref} className={`code-block ${className}`} {...props}>
        <div className="code-block-header">
          <div className="code-block-header-left">
            {language && language !== 'text' && (
              <span className="code-block-language">{displayLang}</span>
            )}
            {fileName && <span className="code-block-filename">{fileName}</span>}
          </div>
          <button
            type="button"
            onClick={handleCopy}
            className="code-block-copy-btn"
            aria-label="Copy code"
          >
            {copied ? (
              <>
                <svg
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                >
                  <polyline points="20 6 9 17 4 12" />
                </svg>
                <span>Copied!</span>
              </>
            ) : (
              <>
                <svg
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                >
                  <rect x="9" y="9" width="13" height="13" rx="2" ry="2" />
                  <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1" />
                </svg>
                <span>Copy code</span>
              </>
            )}
          </button>
        </div>
        <div className="code-block-content">
          <SyntaxHighlighter
            language={language}
            style={vscDarkPlus}
            showLineNumbers={showLineNumbers}
            customStyle={customStyle}
            wrapLines
            wrapLongLines
          >
            {code}
          </SyntaxHighlighter>
        </div>
      </div>
    );
  }
);

CodeBlockComponent.displayName = 'CodeBlock';

// Memoize to prevent unnecessary re-renders during streaming
export const CodeBlock = memo(CodeBlockComponent, (prevProps, nextProps) => {
  return (
    prevProps.code === nextProps.code &&
    prevProps.language === nextProps.language &&
    prevProps.showLineNumbers === nextProps.showLineNumbers &&
    prevProps.fileName === nextProps.fileName &&
    prevProps.className === nextProps.className
  );
});
