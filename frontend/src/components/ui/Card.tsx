import type { ReactNode } from 'react';
import { cn } from '../../lib/utils';

export interface CardProps {
  /** Card content */
  children: ReactNode;
  /** Additional CSS classes */
  className?: string;
}

/**
 * Card container component.
 *
 * @example
 * ```tsx
 * <Card>
 *   <CardHeader>
 *     <CardTitle>Title</CardTitle>
 *   </CardHeader>
 *   <CardContent>Content here</CardContent>
 * </Card>
 * ```
 */
export function Card({ children, className }: CardProps) {
  return <div className={cn('card', className)}>{children}</div>;
}

export interface CardHeaderProps {
  /** Header content */
  children: ReactNode;
  /** Additional CSS classes */
  className?: string;
}

/**
 * Card header section.
 */
export function CardHeader({ children, className }: CardHeaderProps) {
  return <div className={cn('card-header', className)}>{children}</div>;
}

export interface CardTitleProps {
  /** Title content */
  children: ReactNode;
  /** Additional CSS classes */
  className?: string;
}

/**
 * Card title element.
 */
export function CardTitle({ children, className }: CardTitleProps) {
  return <h2 className={cn('card-title', className)}>{children}</h2>;
}

export interface CardContentProps {
  /** Content */
  children: ReactNode;
  /** Additional CSS classes */
  className?: string;
}

/**
 * Card content section.
 */
export function CardContent({ children, className }: CardContentProps) {
  return <div className={cn('card-content', className)}>{children}</div>;
}
