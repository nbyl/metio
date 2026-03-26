import { cn } from '../../lib/utils';

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
 * Switch component for boolean toggles
 */
export function Switch({
  checked,
  onChange,
  disabled = false,
  className,
  'aria-label': ariaLabel,
}: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel}
      data-state={checked ? 'checked' : 'unchecked'}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={cn('switch', className)}
    >
      <span className="switch-thumb" />
    </button>
  );
}
