import { describe, it, expect } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from './Tooltip';

function renderTooltip() {
  return render(
    <TooltipProvider delayDuration={0}>
      <Tooltip>
        <TooltipTrigger asChild>
          <button type="button">Trigger</button>
        </TooltipTrigger>
        <TooltipContent>Tooltip text</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

describe('Tooltip', () => {
  it('renders the trigger without showing content initially', () => {
    renderTooltip();
    expect(screen.getByRole('button', { name: 'Trigger' })).toBeInTheDocument();
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
  });

  it('shows content on hover', async () => {
    const user = userEvent.setup();
    renderTooltip();

    await user.hover(screen.getByRole('button', { name: 'Trigger' }));
    expect(await screen.findByRole('tooltip')).toHaveTextContent(
      'Tooltip text'
    );
  });

  it('closes content when Escape is pressed', async () => {
    const user = userEvent.setup();
    renderTooltip();

    await user.hover(screen.getByRole('button', { name: 'Trigger' }));
    expect(await screen.findByRole('tooltip')).toBeInTheDocument();
    await user.keyboard('{Escape}');
    await waitFor(() => {
      expect(screen.queryByRole('tooltip')).not.toBeInTheDocument();
    });
  });
});
