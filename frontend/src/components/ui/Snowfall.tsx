import { useMemo, useState, useEffect } from 'react';

// Check if it's Christmas season (Dec 15 - Jan 1)
function isChristmasSeason(): boolean {
  const now = new Date();
  const month = now.getMonth();
  const day = now.getDate();
  return (month === 11 && day >= 15) || (month === 0 && day <= 1);
}

// Generate stable snowflake positions
function generateSnowflakes(count: number) {
  return Array.from({ length: count }, (_, i) => ({
    id: i,
    left: `${(i * 37 + 13) % 100}%`,
    animationDuration: `${8 + (i % 7)}s`,
    animationDelay: `${(i * 0.7) % 5}s`,
    size: 0.8 + (i % 3) * 0.2,
  }));
}

interface SnowfallProps {
  count?: number;
  /** Duration in ms before snowfall starts fading out. If not set, snowfall continues indefinitely. */
  fadeAfter?: number;
  /** Duration in ms for the fade out transition. Defaults to 2000ms (2 seconds). */
  fadeDuration?: number;
}

export function Snowfall({ count = 25, fadeAfter, fadeDuration = 2000 }: SnowfallProps) {
  const showSnow = useMemo(() => isChristmasSeason(), []);
  const snowflakes = useMemo(() => generateSnowflakes(count), [count]);
  const [isFading, setIsFading] = useState(false);
  const [isHidden, setIsHidden] = useState(false);

  useEffect(() => {
    if (!showSnow || fadeAfter === undefined) return;

    // Start fading after the specified duration
    const fadeTimer = setTimeout(() => {
      setIsFading(true);
    }, fadeAfter);

    // Fully hide after fade completes
    const hideTimer = setTimeout(() => {
      setIsHidden(true);
    }, fadeAfter + fadeDuration);

    return () => {
      clearTimeout(fadeTimer);
      clearTimeout(hideTimer);
    };
  }, [showSnow, fadeAfter, fadeDuration]);

  if (!showSnow || isHidden) return null;

  return (
    <div
      className="snowfall"
      style={{
        opacity: isFading ? 0 : 1,
        transition: `opacity ${fadeDuration}ms ease-out`,
      }}
    >
      {snowflakes.map(flake => (
        <div
          key={flake.id}
          className="snowflake"
          style={
            {
              left: flake.left,
              animationDuration: flake.animationDuration,
              animationDelay: flake.animationDelay,
              '--flake-size': flake.size,
            } as React.CSSProperties
          }
        >
          ❄
        </div>
      ))}
    </div>
  );
}
