import type { HTMLAttributes } from 'react';
import { cn } from '@/lib/utils';

export type CardProps = HTMLAttributes<HTMLDivElement>;

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
export function Card({ className, ...props }: CardProps) {
  return <div data-slot="card" className={cn('card', className)} {...props} />;
}

export type CardHeaderProps = HTMLAttributes<HTMLDivElement>;

/**
 * Card header section.
 */
export function CardHeader({ className, ...props }: CardHeaderProps) {
  return (
    <div
      data-slot="card-header"
      className={cn('card-header', className)}
      {...props}
    />
  );
}

export type CardTitleProps = HTMLAttributes<HTMLHeadingElement>;

/**
 * Card title element.
 */
export function CardTitle({ className, ...props }: CardTitleProps) {
  return (
    <h2
      data-slot="card-title"
      className={cn('card-title', className)}
      {...props}
    />
  );
}

export type CardContentProps = HTMLAttributes<HTMLDivElement>;

/**
 * Card content section.
 */
export function CardContent({ className, ...props }: CardContentProps) {
  return (
    <div
      data-slot="card-content"
      className={cn('card-content', className)}
      {...props}
    />
  );
}
