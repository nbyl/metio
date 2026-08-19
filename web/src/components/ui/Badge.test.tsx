import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Badge } from './Badge';

describe('Badge', () => {
  it('renders with the default variant', () => {
    render(<Badge>Status</Badge>);

    expect(screen.getByText('Status')).toHaveAttribute(
      'data-variant',
      'default'
    );
  });

  it('supports the canonical variants', () => {
    render(
      <>
        <Badge variant="secondary">Secondary</Badge>
        <Badge variant="destructive">Destructive</Badge>
        <Badge variant="outline">Outline</Badge>
      </>
    );

    expect(screen.getByText('Secondary')).toHaveAttribute(
      'data-variant',
      'secondary'
    );
    expect(screen.getByText('Destructive')).toHaveAttribute(
      'data-variant',
      'destructive'
    );
    expect(screen.getByText('Outline')).toHaveAttribute(
      'data-variant',
      'outline'
    );
  });

  it('forwards extra props', () => {
    render(<Badge data-testid="badge">Custom</Badge>);
    expect(screen.getByTestId('badge')).toBeInTheDocument();
  });
});
