import { forwardRef, type ButtonHTMLAttributes } from 'react';
import { Loader2 } from 'lucide-react';
import { cva, type VariantProps } from 'class-variance-authority';
import { cn } from '@/lib/utils';

const buttonVariants = cva('btn', {
  variants: {
    variant: {
      primary: 'btn-green',
      danger: 'btn-red',
      outline: 'btn-outline',
    },
    size: {
      default: '',
      sm: 'btn-sm',
    },
  },
  defaultVariants: {
    variant: 'primary',
    size: 'default',
  },
});

export interface ButtonProps
  extends
    ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {
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
    { className, variant, size, loading = false, disabled, children, ...props },
    ref
  ) => {
    return (
      <button
        ref={ref}
        data-slot="button"
        data-variant={variant ?? 'primary'}
        data-size={size ?? 'default'}
        className={cn(buttonVariants({ variant, size }), className)}
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
// eslint-disable-next-line react-refresh/only-export-components
export { buttonVariants };
