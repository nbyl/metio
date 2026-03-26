import { useState, useRef, useEffect } from 'react';
import { cn } from '../../lib/utils';

export interface TooltipProps {
  /** The content to show in the tooltip */
  content: React.ReactNode;
  /** The element that triggers the tooltip */
  children: React.ReactNode;
  /** Additional CSS classes for the tooltip */
  className?: string;
  /** Delay before showing tooltip in ms */
  delay?: number;
}

/**
 * Tooltip component that shows content on hover
 */
export function Tooltip({
  content,
  children,
  className,
  delay = 200,
}: TooltipProps) {
  const [isVisible, setIsVisible] = useState(false);
  const timeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const showTooltip = () => {
    timeoutRef.current = setTimeout(() => {
      setIsVisible(true);
    }, delay);
  };

  const hideTooltip = () => {
    if (timeoutRef.current) {
      clearTimeout(timeoutRef.current);
    }
    setIsVisible(false);
  };

  useEffect(() => {
    return () => {
      if (timeoutRef.current) {
        clearTimeout(timeoutRef.current);
      }
    };
  }, []);

  return (
    <span
      className="tooltip-wrapper"
      onMouseEnter={showTooltip}
      onMouseLeave={hideTooltip}
      onFocus={showTooltip}
      onBlur={hideTooltip}
    >
      {children}
      {isVisible && (
        <span
          className={cn('tooltip', className)}
          role="tooltip"
        >
          {content}
        </span>
      )}
    </span>
  );
}
