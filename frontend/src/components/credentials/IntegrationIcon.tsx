interface IntegrationIconProps {
  type: string;
  size?: number;
  className?: string;
}

const ICON_MAP: Record<string, string> = {
  slack: '/icons/slack.svg',
  github: '/icons/github.svg',
  google: '/icons/google.svg',
  openai: '/icons/openai.svg',
  anthropic: '/icons/anthropic.svg',
};

export const IntegrationIcon: React.FC<IntegrationIconProps> = ({
  type,
  size = 20,
  className = '',
}) => {
  const iconSrc = ICON_MAP[type.toLowerCase()];

  if (iconSrc) {
    return (
      <img
        src={iconSrc}
        alt={type}
        width={size}
        height={size}
        className={className}
        style={{ objectFit: 'contain' }}
      />
    );
  }

  // Fallback: first letter
  return (
    <div
      className={className}
      style={{
        width: size,
        height: size,
        borderRadius: '4px',
        background: 'var(--color-surface-hover, #334155)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        fontSize: size * 0.5,
        fontWeight: 600,
        color: 'var(--color-text-secondary, #94a3b8)',
      }}
    >
      {type.charAt(0).toUpperCase()}
    </div>
  );
};
