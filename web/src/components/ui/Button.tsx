import { forwardRef, type ButtonHTMLAttributes } from 'react';
import { Loader2 } from 'lucide-react';
import { cn } from '../../lib/utils';

const variantStyles = {
  primary: 'btn-green',
  danger: 'btn-red',
  outline: 'btn-outline',
} as const;

const sizeStyles = {
  default: '',
  sm: 'btn-sm',
} as const;

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** Button style variant */
  variant?: keyof typeof variantStyles;
  /** Button size */
  size?: keyof typeof sizeStyles;
  /** Show loading spinner and disable button */
  loading?: boolean;
}

/**
 * Button component with multiple variants, sizes, and loading state.
 *
 * @example
 * ```tsx
 * <Button variant="primary" onClick={handleClick}>
 *   Start Server
 * </Button>
 *
 * <Button variant="danger" loading>
 *   Stopping...
 * </Button>
 * ```
 */
export const Button = forwardRef<HTMLButtonElement, ButtonProps>(
  (
    {
      className,
      variant = 'primary',
      size = 'default',
      loading = false,
      disabled,
      children,
      ...props
    },
    ref
  ) => {
    return (
      <button
        ref={ref}
        className={cn(
          'btn',
          variantStyles[variant],
          sizeStyles[size],
          className
        )}
        disabled={disabled || loading}
        {...props}
      >
        {loading && <Loader2 className="h-4 w-4 animate-spin" />}
        {children}
      </button>
    );
  }
);

Button.displayName = 'Button';
