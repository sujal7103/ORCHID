import { forwardRef } from 'react';
import type { HTMLAttributes, ElementType, ReactNode } from 'react';
import './Typography.css';

export type TypographyVariant =
  | 'display'
  | 'h1'
  | 'h2'
  | 'h3'
  | 'h4'
  | 'h5'
  | 'h6'
  | 'xl'
  | 'lg'
  | 'base'
  | 'sm'
  | 'xs';

export type TypographyWeight = 'light' | 'normal' | 'medium' | 'semibold' | 'bold' | 'black';

export type TypographyAlign = 'left' | 'center' | 'right';

export interface TypographyProps extends HTMLAttributes<HTMLElement> {
  variant?: TypographyVariant;
  weight?: TypographyWeight;
  align?: TypographyAlign;
  gradient?: boolean;
  as?: ElementType;
  children: ReactNode;
}

const variantMapping: Record<TypographyVariant, ElementType> = {
  display: 'h1',
  h1: 'h1',
  h2: 'h2',
  h3: 'h3',
  h4: 'h4',
  h5: 'h5',
  h6: 'h6',
  xl: 'p',
  lg: 'p',
  base: 'p',
  sm: 'p',
  xs: 'span',
};

export const Typography = forwardRef<HTMLElement, TypographyProps>(
  (
    { variant = 'base', weight, align, gradient = false, as, className = '', children, ...props },
    ref
  ) => {
    const Component = as || variantMapping[variant];

    const classes = [
      'typography',
      `typography-${variant}`,
      weight && `typography-weight-${weight}`,
      align && `typography-align-${align}`,
      gradient && 'typography-gradient',
      className,
    ]
      .filter(Boolean)
      .join(' ');

    return (
      <Component ref={ref} className={classes} {...props}>
        {children}
      </Component>
    );
  }
);

Typography.displayName = 'Typography';
