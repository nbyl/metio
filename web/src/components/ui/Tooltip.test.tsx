import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Tooltip } from './Tooltip';

describe('Tooltip', () => {
  it('renders the trigger children without showing content initially', () => {
    render(
      <Tooltip content="Tooltip text">
        <span>Trigger</span>
      </Tooltip>
    );
    expect(screen.getByText('Trigger')).toBeInTheDocument();
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });

  it('shows the content on hover', async () => {
    const user = userEvent.setup();
    render(
      <Tooltip content="Tooltip text" delay={0}>
        <span>Trigger</span>
      </Tooltip>
    );

    await user.hover(screen.getByText('Trigger'));
    expect(await screen.findByRole('tooltip')).toHaveTextContent(
      'Tooltip text'
    );
  });

  it('hides the content after leaving the trigger', async () => {
    const user = userEvent.setup();
    render(
      <Tooltip content="Tooltip text" delay={0}>
        <span>Trigger</span>
      </Tooltip>
    );

    await user.hover(screen.getByText('Trigger'));
    expect(await screen.findByRole('tooltip')).toBeInTheDocument();

    await user.unhover(screen.getByText('Trigger'));
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });
});
