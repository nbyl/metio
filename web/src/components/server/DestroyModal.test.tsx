import { useState } from 'react';
import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DestroyModal } from './DestroyModal';

function renderModal(
  props: Partial<React.ComponentProps<typeof DestroyModal>> = {}
) {
  return render(
    <DestroyModal
      open
      serverName="survival"
      onClose={vi.fn()}
      onConfirm={vi.fn()}
      isPending={false}
      {...props}
    />
  );
}

describe('DestroyModal', () => {
  it('renders nothing when closed', () => {
    const { container } = renderModal({ open: false });
    expect(container.firstChild).toBeNull();
  });

  it('shows the warning step and deletion list by default', () => {
    renderModal();

    const dialog = screen.getByRole('alertdialog', { name: 'Destroy Server' });
    expect(dialog).toHaveAttribute('aria-labelledby');
    expect(screen.getByText('Destroy Server')).toBeInTheDocument();
    expect(
      screen.getByText(/This action will permanently destroy/)
    ).toBeInTheDocument();
    expect(screen.getByText('The Minecraft server VM')).toBeInTheDocument();
    expect(
      screen.getByText('All world data and configurations')
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'Backups in the central catalog are preserved and expire automatically once their retention period ends.'
      )
    ).toBeInTheDocument();
    expect(screen.queryByText('Backup bucket and backups')).toBeNull();
    expect(screen.getByText('Static IP address')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeInTheDocument();
    expect(
      screen.getByRole('button', { name: 'Continue →' })
    ).toBeInTheDocument();
  });

  it('closes when Cancel is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderModal({ onClose });

    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(onClose).toHaveBeenCalled();
  });

  it('closes when the close icon is clicked', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderModal({ onClose });

    await user.click(screen.getByRole('button', { name: 'Close' }));
    expect(onClose).toHaveBeenCalled();
  });

  it('closes through Escape and resets the controlled dialog', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderModal({ onClose });

    await user.keyboard('{Escape}');
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('advances to the confirm step when Continue is clicked', async () => {
    const user = userEvent.setup();
    renderModal();

    await user.click(screen.getByRole('button', { name: 'Continue →' }));

    expect(
      screen.getByText('Type the server name to confirm:')
    ).toBeInTheDocument();
    expect(screen.getByPlaceholderText('survival')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Back' })).toBeInTheDocument();
    const destroyButton = screen.getByRole('button', {
      name: 'Destroy Server',
    });
    expect(destroyButton).toBeDisabled();
  });

  it('enables the Destroy button only when the server name is typed exactly', async () => {
    const user = userEvent.setup();
    renderModal();

    await user.click(screen.getByRole('button', { name: 'Continue →' }));

    const input = screen.getByPlaceholderText('survival');
    await user.type(input, 'survi');
    expect(
      screen.getByRole('button', { name: 'Destroy Server' })
    ).toBeDisabled();

    await user.type(input, 'val');
    expect(
      screen.getByRole('button', { name: 'Destroy Server' })
    ).toBeEnabled();
  });

  it('goes back to the warning step when Back is clicked', async () => {
    const user = userEvent.setup();
    renderModal();

    await user.click(screen.getByRole('button', { name: 'Continue →' }));
    await user.click(screen.getByRole('button', { name: 'Back' }));

    expect(screen.getByText('Continue →')).toBeInTheDocument();
    expect(screen.queryByText('Type the server name to confirm:')).toBeNull();
  });

  it('calls onConfirm when Destroy Server is confirmed', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    renderModal({ onConfirm });

    await user.click(screen.getByRole('button', { name: 'Continue →' }));
    await user.type(screen.getByPlaceholderText('survival'), 'survival');
    await user.click(screen.getByRole('button', { name: 'Destroy Server' }));

    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('disables the controls and shows the loader while pending', async () => {
    const user = userEvent.setup();
    renderModal({ isPending: true });

    await user.click(screen.getByRole('button', { name: 'Continue →' }));

    const destroyButton = screen.getByRole('button', {
      name: 'Destroy Server',
    });
    expect(destroyButton).toBeDisabled();
    expect(destroyButton.querySelector('.animate-spin')).not.toBeNull();
    expect(screen.getByRole('button', { name: 'Back' })).toBeDisabled();
  });
});

describe('DestroyModal internal', () => {
  it('resets the step when closed via Cancel and reopened', async () => {
    const user = userEvent.setup();

    function Harness() {
      const [open, setOpen] = useState(true);
      return (
        <div>
          <button type="button" onClick={() => setOpen(true)}>
            reopen
          </button>
          <DestroyModal
            open={open}
            serverName="survival"
            onClose={() => setOpen(false)}
            onConfirm={vi.fn()}
            isPending={false}
          />
        </div>
      );
    }

    render(<Harness />);

    await user.click(screen.getByRole('button', { name: 'Continue →' }));
    expect(
      screen.getByText('Type the server name to confirm:')
    ).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Back' }));
    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(screen.queryByText('Destroy Server')).toBeNull();

    await user.click(screen.getByRole('button', { name: 'reopen' }));
    expect(screen.getByText('Continue →')).toBeInTheDocument();
    expect(screen.queryByText('Type the server name to confirm:')).toBeNull();
  });
});
