import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Switch } from './Switch';

describe('Switch', () => {
  it('renders a switch with the correct ARIA role', () => {
    render(<Switch />);
    expect(screen.getByRole('switch')).toBeInTheDocument();
  });

  it('exposes checked state via aria-checked', () => {
    render(<Switch checked />);
    expect(screen.getByRole('switch')).toHaveAttribute('aria-checked', 'true');
  });

  it('calls onCheckedChange with the new value when toggled', async () => {
    const user = userEvent.setup();
    const handleChange = vi.fn();
    render(<Switch onCheckedChange={handleChange} />);

    await user.click(screen.getByRole('switch'));
    expect(handleChange).toHaveBeenCalledWith(true);
  });

  it('supports the small size', () => {
    render(<Switch size="sm" />);
    expect(screen.getByRole('switch')).toHaveAttribute('data-size', 'sm');
  });

  it('is disabled when disabled is set and does not toggle', async () => {
    const user = userEvent.setup();
    const handleChange = vi.fn();
    render(<Switch onCheckedChange={handleChange} disabled />);

    const switcher = screen.getByRole('switch');
    expect(switcher).toBeDisabled();

    await user.click(switcher);
    expect(handleChange).not.toHaveBeenCalled();
  });

  it('exposes an accessible label', () => {
    render(<Switch aria-label="Toggle whitelist" />);
    expect(
      screen.getByRole('switch', { name: 'Toggle whitelist' })
    ).toBeInTheDocument();
  });
});
