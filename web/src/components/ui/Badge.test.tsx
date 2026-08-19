import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Badge } from './Badge';

describe('Badge', () => {
  it('renders with children', () => {
    render(<Badge variant="online">Status</Badge>);
    expect(screen.getByText('Status')).toBeInTheDocument();
  });

  it('applies online variant', () => {
    render(<Badge variant="online">Online</Badge>);
    expect(screen.getByText('Online')).toHaveAttribute(
      'data-variant',
      'online'
    );
  });

  it('applies offline variant', () => {
    render(<Badge variant="offline">Offline</Badge>);
    expect(screen.getByText('Offline')).toHaveAttribute(
      'data-variant',
      'offline'
    );
  });

  it('applies transitioning variant', () => {
    render(<Badge variant="transitioning">Starting...</Badge>);
    expect(screen.getByText('Starting...')).toHaveAttribute(
      'data-variant',
      'transitioning'
    );
  });

  it('forwards extra props', () => {
    render(
      <Badge variant="online" data-testid="badge">
        Custom
      </Badge>
    );
    expect(screen.getByTestId('badge')).toBeInTheDocument();
  });
});
