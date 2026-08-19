import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Button } from './Button';

describe('Button', () => {
  it('renders with children and the default variant', () => {
    render(<Button>Click me</Button>);
    const button = screen.getByRole('button', { name: 'Click me' });

    expect(button).toHaveAttribute('data-variant', 'default');
    expect(button).toHaveAttribute('data-size', 'default');
  });

  it('supports the canonical variants', () => {
    render(
      <>
        <Button variant="destructive">Destructive</Button>
        <Button variant="outline">Outline</Button>
        <Button variant="secondary">Secondary</Button>
      </>
    );

    expect(screen.getByRole('button', { name: 'Destructive' })).toHaveAttribute(
      'data-variant',
      'destructive'
    );
    expect(screen.getByRole('button', { name: 'Outline' })).toHaveAttribute(
      'data-variant',
      'outline'
    );
    expect(screen.getByRole('button', { name: 'Secondary' })).toHaveAttribute(
      'data-variant',
      'secondary'
    );
  });

  it('supports the small size and disabled state', () => {
    render(
      <Button size="sm" disabled>
        Small
      </Button>
    );

    expect(screen.getByRole('button')).toHaveAttribute('data-size', 'sm');
    expect(screen.getByRole('button')).toBeDisabled();
  });

  it('forwards extra props and click handlers', async () => {
    const user = userEvent.setup();
    const handleClick = vi.fn();
    render(
      <Button data-testid="button" onClick={handleClick}>
        Click
      </Button>
    );

    await user.click(screen.getByTestId('button'));
    expect(handleClick).toHaveBeenCalledTimes(1);
  });

  it('supports rendering through Slot with asChild', () => {
    render(
      <Button asChild>
        <a href="/servers">Servers</a>
      </Button>
    );

    expect(screen.getByRole('link', { name: 'Servers' })).toHaveAttribute(
      'data-slot',
      'button'
    );
  });
});
