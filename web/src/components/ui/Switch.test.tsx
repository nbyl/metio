import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Switch } from './Switch';

describe('Switch', () => {
  it('renders a switch with the correct ARIA role', () => {
    render(<Switch checked={false} onChange={() => {}} />);
    expect(screen.getByRole('switch')).toBeInTheDocument();
  });

  it('exposes checked state via aria-checked', () => {
    render(<Switch checked={true} onChange={() => {}} />);
    expect(screen.getByRole('switch')).toHaveAttribute('aria-checked', 'true');
  });

  it('calls onChange with the new value when toggled', async () => {
    const user = userEvent.setup();
    const handleChange = vi.fn();
    render(<Switch checked={false} onChange={handleChange} />);

    await user.click(screen.getByRole('switch'));
    expect(handleChange).toHaveBeenCalledWith(true);
  });

  it('is reachable via keyboard tab order', async () => {
    const user = userEvent.setup();
    render(<Switch checked={false} onChange={() => {}} />);

    await user.tab();
    expect(screen.getByRole('switch')).toHaveFocus();
  });

  it('is disabled when disabled prop is set and does not toggle', async () => {
    const user = userEvent.setup();
    const handleChange = vi.fn();
    render(<Switch checked={false} onChange={handleChange} disabled />);

    const switcher = screen.getByRole('switch');
    expect(switcher).toBeDisabled();

    await user.click(switcher);
    expect(handleChange).not.toHaveBeenCalled();
  });

  it('exposes an accessible label', () => {
    render(
      <Switch
        checked={false}
        onChange={() => {}}
        aria-label="Toggle whitelist"
      />
    );
    expect(
      screen.getByRole('switch', { name: 'Toggle whitelist' })
    ).toBeInTheDocument();
  });
});
