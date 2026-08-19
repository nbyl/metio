import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
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

describe('UpdateModal minecraft version field', () => {
  it('renders a dropdown of the available versions', () => {
    renderModal();

    const select = screen.getByDisplayValue('1.21.11');
    expect(select.tagName).toBe('SELECT');
    expect(
      Array.from(select.querySelectorAll('option')).map((o) => o.value)
    ).toEqual(['26.2', '1.21.11', '1.21.10']);
  });

  it('keeps the versions in the order returned by the API', () => {
    renderModal();

    const select = screen.getByDisplayValue('1.21.11');
    expect(
      Array.from(select.querySelectorAll('option')).map((o) => o.value)[0]
    ).toBe('26.2');
  });

  it('includes the current version when the API no longer offers it', () => {
    renderModal({ minecraftVersion: '1.7.10' });

    const select = screen.getByDisplayValue('1.7.10');
    expect(
      Array.from(select.querySelectorAll('option')).map((o) => o.value)
    ).toEqual(['1.7.10', '26.2', '1.21.11', '1.21.10']);
  });
});

describe('UpdateModal machine type field', () => {
  it('renders a dropdown of the available machine types', () => {
    renderModal();

    const select = screen.getAllByRole('combobox')[0] as HTMLSelectElement;
    expect(select.tagName).toBe('SELECT');
    expect(select.value).toBe('e2-small');
    expect(
      Array.from(select.querySelectorAll('option')).map((o) => o.value)
    ).toEqual(['e2-small', 'e2-medium']);
  });

  it('keeps the current machine type when the API no longer offers it', () => {
    renderModal({ machineType: 'e2-standard-2' });

    const select = screen.getAllByRole('combobox')[0] as HTMLSelectElement;
    expect(
      Array.from(select.querySelectorAll('option')).map((o) => o.value)
    ).toEqual(['e2-standard-2', 'e2-small', 'e2-medium']);
  });

  it('disables both option dropdowns while options are loading', () => {
    vi.mocked(useServerOptions).mockReturnValue({
      data: undefined,
      isLoading: true,
    } as never);

    renderModal();

    expect(screen.getAllByRole('combobox')[0]).toBeDisabled();
    expect(screen.getByDisplayValue('1.21.11')).toBeDisabled();
  });
});

describe('UpdateModal tabs', () => {
  it('shows the Settings tab by default', () => {
    renderModal();

    expect(screen.getByRole('tab', { name: 'Settings' })).toHaveAttribute(
      'aria-selected',
      'true'
    );
    expect(screen.getByRole('tab', { name: 'Backup' })).toHaveAttribute(
      'aria-selected',
      'false'
    );
    expect(screen.getByDisplayValue('test-server')).toBeInTheDocument();
  });

  it('shows the backup settings when the Backup tab is selected', async () => {
    renderModal();

    const user = userEvent.setup();
    await user.click(screen.getByRole('tab', { name: 'Backup' }));

    expect(screen.getByRole('tab', { name: 'Backup' })).toHaveAttribute(
      'aria-selected',
      'true'
    );
    expect(screen.getByLabelText('Backup interval (hours)')).toHaveValue(6);
    expect(screen.getByLabelText('Retention policy')).toHaveValue(3);
  });
});

describe('UpdateModal close behaviour', () => {
  it('closes via the close icon button', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const { container } = renderModal({}, { onClose });

    const closeButton = Array.from(container.querySelectorAll('button')).find(
      (button) => button.textContent?.trim() === ''
    );
    expect(closeButton).toBeDefined();
    await user.click(closeButton as HTMLButtonElement);

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('closes via the Cancel button', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    renderModal({}, { onClose });

    await user.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(onClose).toHaveBeenCalledTimes(1);
  });
});

describe('UpdateModal submit behaviour', () => {
  it('submits only the changed fields', async () => {
    const user = userEvent.setup();
    const onUpdate = vi.fn();
    renderModal({}, { onUpdate });

    await user.clear(screen.getByDisplayValue('test-server'));
    await user.type(screen.getByDisplayValue(''), 'renamed-server');
    await user.click(screen.getByRole('button', { name: 'Update Server' }));

    expect(onUpdate).toHaveBeenCalledWith({ name: 'renamed-server' });
  });

  it('submits the machine type when changed', async () => {
    const user = userEvent.setup();
    const onUpdate = vi.fn();
    renderModal({}, { onUpdate });

    await user.selectOptions(screen.getAllByRole('combobox')[0], 'e2-medium');
    await user.click(screen.getByRole('button', { name: 'Update Server' }));

    expect(onUpdate).toHaveBeenCalledWith({ machineType: 'e2-medium' });
  });

  it('submits the disk size when changed', async () => {
    const user = userEvent.setup();
    const onUpdate = vi.fn();
    renderModal({}, { onUpdate });

    const diskInput = screen.getByDisplayValue('50');
    await user.clear(diskInput);
    await user.type(diskInput, '100');
    await user.click(screen.getByRole('button', { name: 'Update Server' }));

    expect(onUpdate).toHaveBeenCalledWith({ diskSizeGB: 100 });
  });

  it('submits the minecraft version when changed', async () => {
    const user = userEvent.setup();
    const onUpdate = vi.fn();
    renderModal({}, { onUpdate });

    await user.selectOptions(screen.getByDisplayValue('1.21.11'), '26.2');
    await user.click(screen.getByRole('button', { name: 'Update Server' }));

    expect(onUpdate).toHaveBeenCalledWith({ minecraftVersion: '26.2' });
  });

  it('disables the Update button when nothing changed and not outdated', () => {
    renderModal();

    expect(
      screen.getByRole('button', { name: 'Update Server' })
    ).toBeDisabled();
  });

  it('shows the outdated banner and allows submitting an empty payload', async () => {
    const user = userEvent.setup();
    const onUpdate = vi.fn();
    renderModal({}, { outdated: true, onUpdate });

    expect(
      screen.getByText('Infrastructure Update Available')
    ).toBeInTheDocument();
    expect(screen.getByText(/Controller version: v1/)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Update Server' }));

    expect(onUpdate).toHaveBeenCalledWith({});
  });
});
