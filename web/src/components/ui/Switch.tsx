import { Switch as SwitchPrimitive } from 'radix-ui';
import { cn } from '@/lib/utils';

export interface SwitchProps {
  /** Whether the switch is checked */
  checked: boolean;
  /** Callback when the switch is toggled */
  onChange: (checked: boolean) => void;
  /** Whether the switch is disabled */
  disabled?: boolean;
  /** Additional CSS classes */
  className?: string;
  /** Accessible label */
  'aria-label'?: string;
}

/**
 * Switch component for boolean toggles.
 *
 * Built on Radix UI's Switch primitive so it is keyboard-operable (space/enter)
 * and exposes the correct `switch` role and `aria-checked` state.
 */
export function Switch({
  checked,
  onChange,
  disabled = false,
  className,
  ...props
}: SwitchProps) {
  return (
    <SwitchPrimitive.Root
      data-slot="switch"
      checked={checked}
      onCheckedChange={onChange}
      disabled={disabled}
      className={cn('switch', className)}
      {...props}
    >
      <SwitchPrimitive.Thumb
        data-slot="switch-thumb"
        className="switch-thumb"
      />
    </SwitchPrimitive.Root>
  );
}
