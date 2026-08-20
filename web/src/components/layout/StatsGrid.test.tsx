import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Activity } from 'lucide-react';
import { StatsGrid } from './StatsGrid';

describe('StatsGrid', () => {
  const mockStats = [
    { label: 'Status', value: 'Running' },
    { label: 'Players', value: '5/20' },
    { label: 'Uptime', value: '3h 45m' },
    { label: 'IP', value: '192.168.1.1' },
  ];

  it('renders all stat items', () => {
    render(<StatsGrid stats={mockStats} />);
    expect(screen.getByText('Status')).toBeInTheDocument();
    expect(screen.getByText('Players')).toBeInTheDocument();
    expect(screen.getByText('Uptime')).toBeInTheDocument();
    expect(screen.getByText('IP')).toBeInTheDocument();
  });

  it('renders stat labels', () => {
    render(<StatsGrid stats={mockStats} />);
    const labels = screen.getAllByText(/Status|Players|Uptime|IP/);
    expect(labels).toHaveLength(4);
  });

  it('renders stat values', () => {
    render(<StatsGrid stats={mockStats} />);
    expect(screen.getByText('Running')).toBeInTheDocument();
    expect(screen.getByText('5/20')).toBeInTheDocument();
    expect(screen.getByText('3h 45m')).toBeInTheDocument();
    expect(screen.getByText('192.168.1.1')).toBeInTheDocument();
  });

  it('renders icons when provided', () => {
    const statsWithIcon = [
      {
        label: 'Status',
        value: 'Running',
        icon: <Activity data-testid="status-icon" />,
      },
    ];
    render(<StatsGrid stats={statsWithIcon} />);
    expect(screen.getByTestId('status-icon')).toBeInTheDocument();
  });

  it('handles empty stats array', () => {
    const { container } = render(<StatsGrid stats={[]} />);
    expect(container.firstChild).toBeNull();
  });

  it('merges custom className on the grid', () => {
    const { container } = render(
      <StatsGrid stats={mockStats} className="custom-grid" />
    );
    const grid = container.firstChild;
    expect(grid).toHaveClass('grid', 'custom-grid');
  });
});
