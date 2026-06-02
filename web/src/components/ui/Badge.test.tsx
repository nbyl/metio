import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Badge } from './Badge';

describe('Badge', () => {
  // Behavioral tests
  it('renders with children', () => {
    render(<Badge variant="online">Status</Badge>);
    expect(screen.getByText('Status')).toBeInTheDocument();
  });

  it('applies online variant', () => {
    render(<Badge variant="online">Online</Badge>);
    const badge = screen.getByText('Online');
    expect(badge).toHaveClass('badge', 'badge-online');
  });

  it('applies offline variant', () => {
    render(<Badge variant="offline">Offline</Badge>);
    const badge = screen.getByText('Offline');
    expect(badge).toHaveClass('badge', 'badge-offline');
  });

  it('applies transitioning variant', () => {
    render(<Badge variant="transitioning">Starting...</Badge>);
    const badge = screen.getByText('Starting...');
    expect(badge).toHaveClass('badge', 'badge-transitioning');
  });

  it('merges custom className', () => {
    render(
      <Badge variant="online" className="custom-class">
        Custom
      </Badge>
    );
    const badge = screen.getByText('Custom');
    expect(badge).toHaveClass('badge', 'badge-online', 'custom-class');
  });

  // Snapshot tests
  it('matches snapshot (online)', () => {
    const { container } = render(<Badge variant="online">Online</Badge>);
    expect(container).toMatchSnapshot();
  });

  it('matches snapshot (offline)', () => {
    const { container } = render(<Badge variant="offline">Offline</Badge>);
    expect(container).toMatchSnapshot();
  });

  it('matches snapshot (transitioning)', () => {
    const { container } = render(
      <Badge variant="transitioning">Starting...</Badge>
    );
    expect(container).toMatchSnapshot();
  });
});
