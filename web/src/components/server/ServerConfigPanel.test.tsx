import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { ServerConfigPanel } from './ServerConfigPanel';
import type { ServerConfig } from '../../types/server';

const config: ServerConfig = {
  name: 'survival',
  region: 'europe-west1',
  zone: 'europe-west1-b',
  machineType: 'e2-small',
  minecraftVersion: '1.20.4',
  diskSizeGB: 20,
  createdAt: '2026-01-01T00:00:00Z',
  updatedAt: '2026-02-02T12:00:00Z',
};

function renderPanel(
  props: Partial<React.ComponentProps<typeof ServerConfigPanel>> = {}
) {
  return render(
    <ServerConfigPanel
      config={config}
      infrahVersion={3}
      outdated={false}
      {...props}
    />
  );
}

describe('ServerConfigPanel compact', () => {
  it('renders collapsed by default', () => {
    renderPanel({ compact: true });

    expect(screen.getByText('Configuration')).toBeInTheDocument();
    expect(screen.queryByText('Region')).toBeNull();
  });

  it('expands to show the config stats when clicked', async () => {
    const user = userEvent.setup();
    renderPanel({ compact: true });

    await user.click(screen.getByText('Configuration'));

    expect(screen.getByText('europe-west1/europe-west1-b')).toBeInTheDocument();
    expect(screen.getByText('e2-small')).toBeInTheDocument();
    expect(screen.getByText('1.20.4')).toBeInTheDocument();
    expect(screen.getByText('20 GB')).toBeInTheDocument();
  });

  it('collapses again when clicked a second time', async () => {
    const user = userEvent.setup();
    renderPanel({ compact: true });

    await user.click(screen.getByText('Configuration'));
    expect(screen.getByText('Region')).toBeInTheDocument();

    await user.click(screen.getByText('Configuration'));
    expect(screen.queryByText('Region')).toBeNull();
  });

  it('shows the Update Available badge when outdated', async () => {
    const user = userEvent.setup();
    renderPanel({ compact: true, outdated: true });

    expect(screen.getByText('Update Available')).toBeInTheDocument();

    await user.click(screen.getByText('Configuration'));
    expect(screen.getByText('Update Available')).toBeInTheDocument();
  });

  it('reflects the open state via data-state', async () => {
    const user = userEvent.setup();
    const { container } = renderPanel({ compact: true });

    const content = container.querySelector(
      '[data-slot="collapsible-content"]'
    );
    expect(content).toHaveAttribute('data-state', 'closed');

    await user.click(screen.getByText('Configuration'));
    expect(content).toHaveAttribute('data-state', 'open');
  });
});

describe('ServerConfigPanel full', () => {
  it('renders the full card variant with all stats', async () => {
    const user = userEvent.setup();
    renderPanel();

    expect(screen.getByText('Server Configuration')).toBeInTheDocument();
    expect(screen.queryByText('survival')).toBeNull();

    await user.click(screen.getByText('Server Configuration'));

    expect(screen.getByText('survival')).toBeInTheDocument();
    expect(screen.getByText('europe-west1/europe-west1-b')).toBeInTheDocument();
    expect(screen.getByText('e2-small')).toBeInTheDocument();
    expect(screen.getByText('20 GB')).toBeInTheDocument();
    expect(screen.getByText('v3')).toBeInTheDocument();
    expect(screen.getByText(/Created:/)).toBeInTheDocument();
    expect(screen.getByText(/Updated:/)).toBeInTheDocument();
  });

  it('renders the formatter output for the created and updated dates', async () => {
    const user = userEvent.setup();
    renderPanel();

    await user.click(screen.getByText('Server Configuration'));

    expect(screen.getByText(/Created:/)).toBeInTheDocument();
    expect(screen.getByText(/Updated:/)).toBeInTheDocument();
    expect(screen.getByText(/Created:/)).toHaveTextContent('2026');
    expect(screen.getByText(/Updated:/)).toHaveTextContent('2026');
  });
});

describe('ServerConfigPanel with hooks', () => {
  it('renders the Update Available badge in the full variant', async () => {
    const user = userEvent.setup();
    renderPanel({ outdated: true });

    expect(screen.getByText('Update Available')).toBeInTheDocument();

    await user.click(screen.getByText('Server Configuration'));
    expect(screen.getByText(/Created:/)).toBeInTheDocument();
  });
});
