import type { ReactNode } from 'react';
import { cn } from '../../lib/utils';

export interface StatItem {
  /** Stat label */
  label: string;
  /** Stat value */
  value: string | number;
  /** Optional icon to display with the label */
  icon?: ReactNode;
}

export interface StatsGridProps {
  /** Array of stat items to display */
  stats: StatItem[];
  /** Additional CSS classes */
  className?: string;
}

/**
 * Grid component for displaying server statistics.
 *
 * @example
 * ```tsx
 * <StatsGrid
 *   stats={[
 *     { label: 'Status', value: 'Running' },
 *     { label: 'Players', value: '5/20' },
 *     { label: 'Uptime', value: '3h 45m' },
 *     { label: 'IP', value: '192.168.1.1' },
 *   ]}
 * />
 * ```
 */
export function StatsGrid({ stats, className }: StatsGridProps) {
  if (stats.length === 0) {
    return null;
  }

  return (
    <div className={cn('stats-grid', className)}>
      {stats.map((stat) => (
        <div key={stat.label} className="stat">
          <span className="stat-label">
            {stat.icon}
            {stat.label}
          </span>
          <span className="stat-value">{stat.value}</span>
        </div>
      ))}
    </div>
  );
}
