import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { UpdateModal } from './UpdateModal';
import type { ServerConfig } from '../../types/server';
import {
  useBackupSettings,
  useUpdateBackupSettings,
} from '../../hooks/useBackupSettings';
import { useServerOptions } from '../../hooks/useServerOptions';

vi.mock('react-router-dom', () => ({
  useNavigate: () => vi.fn(),
}));

vi.mock('../../hooks/useServerOptions', () => ({
  useServerOptions: vi.fn(),
}));

vi.mock('../../hooks/useBackupSettings', () => ({
  useBackupSettings: vi.fn(),
  useUpdateBackupSettings: vi.fn(),
}));

const config: ServerConfig = {
  name: 'test-server',
  region: 'us-central1',
  zone: 'us-central1-a',
  machineType: 'e2-small',
  minecraftVersion: '1.21.11',
  diskSizeGB: 50,
} as ServerConfig;

const machineTypes = [
  { id: 'e2-small', vcpus: 2, memoryGB: 2 },
  { id: 'e2-medium', vcpus: 2, memoryGB: 4 },
];

function renderModal(
  configOverrides: Partial<ServerConfig> = {},
  handlerOverrides: Partial<Parameters<typeof UpdateModal>[0]> = {}
) {
  return render(
    <UpdateModal
      open
      serverId="srv1"
      serverName="test-server"
      config={{ ...config, ...configOverrides }}
      currentInfraVersion={1}
      outdated={false}
      onClose={vi.fn()}
      onUpdate={vi.fn()}
      isPending={false}
      {...handlerOverrides}
    />
  );
}

async function chooseOption(
  user: ReturnType<typeof userEvent.setup>,
  label: string,
  option: string
) {
  await user.click(screen.getByRole('combobox', { name: label }));
  await user.click(screen.getByRole('option', { name: option }));
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(useServerOptions).mockReturnValue({
    data: {
      minecraftVersions: ['26.2', '1.21.11', '1.21.10'],
      machineTypes,
    },
    isLoading: false,
  } as never);
  vi.mocked(useBackupSettings).mockReturnValue({
    data: { enabled: true, backupIntervalHours: 6, keep: 3, keepUnit: 'daily' },
    isLoading: false,
  } as never);
  vi.mocked(useUpdateBackupSettings).mockReturnValue({
    mutate: vi.fn(),
    isPending: false,
  } as never);
});

describe('UpdateModal dialog behavior', () => {
  it('renders an accessible dialog with a labelled title', () => {
    renderModal();

    const dialog = screen.getByRole('dialog', { name: 'Server Settings' });
    expect(dialog).toHaveAttribute('aria-labelledby');
    expect(dialog).toHaveAttribute('aria-describedby');
  });

  it('closes through the close button, Cancel, and Escape', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderModal({}, { onClose });

    await user.click(screen.getByRole('button', { name: 'Close' }));
    expect(onClose).toHaveBeenCalledTimes(1);

    onClose.mockClear();
    renderModal({}, { onClose });
    await user.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(onClose).toHaveBeenCalledTimes(1);

    onClose.mockClear();
    renderModal({}, { onClose });
    await user.keyboard('{Escape}');
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('keeps focus inside the dialog when tabbing', async () => {
    const user = userEvent.setup();
    renderModal();

    await waitFor(() => {
      expect(screen.getByRole('dialog')).toContainElement(
        document.activeElement
      );
    });
    await user.tab({ shift: true });
    expect(screen.getByRole('dialog')).toContainElement(document.activeElement);
  });
});

describe('UpdateModal select fields', () => {
  it('renders versions in API order', async () => {
    const user = userEvent.setup();
    renderModal();

    await user.click(
      screen.getByRole('combobox', { name: 'Minecraft Version' })
    );
    expect(
      screen.getAllByRole('option').map((option) => option.textContent)
    ).toEqual(['26.2', '1.21.11', '1.21.10']);
  });

  it('includes the current version when the API no longer offers it', async () => {
    const user = userEvent.setup();
    renderModal({ minecraftVersion: '1.7.10' });

    await user.click(
      screen.getByRole('combobox', { name: 'Minecraft Version' })
    );
    expect(
      screen.getAllByRole('option').map((option) => option.textContent)
    ).toEqual(['1.7.10', '26.2', '1.21.11', '1.21.10']);
  });

  it('keeps the current machine type when the API no longer offers it', async () => {
    const user = userEvent.setup();
    renderModal({ machineType: 'e2-standard-2' });

    await user.click(screen.getByRole('combobox', { name: 'Machine Type' }));
    expect(
      screen.getAllByRole('option').map((option) => option.textContent)
    ).toEqual([
      'e2-standard-2',
      'e2-small (2 vCPU · 2 GB RAM)',
      'e2-medium (2 vCPU · 4 GB RAM)',
    ]);
  });

  it('disables both selects while options are loading', () => {
    vi.mocked(useServerOptions).mockReturnValue({
      data: undefined,
      isLoading: true,
    } as never);

    renderModal();

    expect(
      screen.getByRole('combobox', { name: 'Machine Type' })
    ).toBeDisabled();
    expect(
      screen.getByRole('combobox', { name: 'Minecraft Version' })
    ).toBeDisabled();
  });
});

describe('UpdateModal tabs and submit behavior', () => {
  it('shows Settings by default and switches to Backup', async () => {
    const user = userEvent.setup();
    renderModal();

    expect(screen.getByRole('tab', { name: 'Settings' })).toHaveAttribute(
      'aria-selected',
      'true'
    );
    expect(screen.getByDisplayValue('test-server')).toBeInTheDocument();

    await user.click(screen.getByRole('tab', { name: 'Backup' }));
    expect(screen.getByRole('tab', { name: 'Backup' })).toHaveAttribute(
      'aria-selected',
      'true'
    );
    expect(screen.getByLabelText('Backup interval (hours)')).toHaveValue(6);
  });

  it('submits only changed fields', async () => {
    const user = userEvent.setup();
    const onUpdate = vi.fn();
    renderModal({}, { onUpdate });

    await user.clear(screen.getByLabelText('Server Name'));
    await user.type(screen.getByLabelText('Server Name'), 'renamed-server');
    await user.click(screen.getByRole('button', { name: 'Update Server' }));

    expect(onUpdate).toHaveBeenCalledWith({ name: 'renamed-server' });
  });

  it('submits machine type and Minecraft version changes', async () => {
    const user = userEvent.setup();
    const onUpdate = vi.fn();
    renderModal({}, { onUpdate });

    await chooseOption(user, 'Machine Type', 'e2-medium (2 vCPU · 4 GB RAM)');
    await chooseOption(user, 'Minecraft Version', '26.2');
    await user.click(screen.getByRole('button', { name: 'Update Server' }));

    expect(onUpdate).toHaveBeenCalledWith({
      machineType: 'e2-medium',
      minecraftVersion: '26.2',
    });
  });

  it('submits disk size changes', async () => {
    const user = userEvent.setup();
    const onUpdate = vi.fn();
    renderModal({}, { onUpdate });

    const diskInput = screen.getByLabelText('Disk Size (GB)');
    await user.clear(diskInput);
    await user.type(diskInput, '100');
    await user.click(screen.getByRole('button', { name: 'Update Server' }));

    expect(onUpdate).toHaveBeenCalledWith({ diskSizeGB: 100 });
  });

  it('disables Update when unchanged and permits an outdated empty update', async () => {
    const user = userEvent.setup();
    const onUpdate = vi.fn();
    const { unmount } = renderModal({}, { onUpdate });
    expect(
      screen.getByRole('button', { name: 'Update Server' })
    ).toBeDisabled();

    unmount();
    renderModal({}, { outdated: true, onUpdate });
    await user.click(screen.getByRole('button', { name: 'Update Server' }));
    expect(onUpdate).toHaveBeenCalledWith({});
  });
});
