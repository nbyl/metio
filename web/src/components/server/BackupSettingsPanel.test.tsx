import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { BackupSettingsPanel } from './BackupSettingsPanel';
import {
  useBackupSettings,
  useUpdateBackupSettings,
} from '../../hooks/useBackupSettings';

const mockNavigate = vi.hoisted(() => vi.fn());

vi.mock('react-router-dom', () => ({
  useNavigate: () => mockNavigate,
}));

vi.mock('../../hooks/useBackupSettings', () => ({
  useBackupSettings: vi.fn(),
  useUpdateBackupSettings: vi.fn(),
}));

const settings = {
  enabled: true,
  backupIntervalHours: 6,
  keep: 3,
  keepUnit: 'daily',
};

function renderPanel(
  props: Partial<React.ComponentProps<typeof BackupSettingsPanel>> = {}
) {
  return render(
    <BackupSettingsPanel serverId="srv1" serverName="Survival" {...props} />
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(useBackupSettings).mockReturnValue({
    data: settings,
    isLoading: false,
  } as never);
  vi.mocked(useUpdateBackupSettings).mockReturnValue({
    mutate: vi.fn(),
    isPending: false,
  } as never);
});

describe('BackupSettingsPanel loading', () => {
  it('shows a spinner while settings are loading', () => {
    vi.mocked(useBackupSettings).mockReturnValue({
      data: undefined,
      isLoading: true,
    } as never);

    renderPanel();

    expect(document.querySelector('.animate-spin')).not.toBeNull();
  });

  it('shows a spinner when data has not arrived yet', () => {
    vi.mocked(useBackupSettings).mockReturnValue({
      data: undefined,
      isLoading: false,
    } as never);

    renderPanel();

    expect(document.querySelector('.animate-spin')).not.toBeNull();
  });
});

describe('BackupSettingsPanel form', () => {
  it('prefills the form from the fetched settings', async () => {
    renderPanel();

    expect(screen.getByText('Scheduled backups enabled')).toBeInTheDocument();
    expect(
      screen.getByRole('switch', { name: 'Toggle backups' })
    ).toHaveAttribute('aria-checked', 'true');
    expect(screen.getByLabelText('Backup interval (hours)')).toHaveValue(6);
    expect(screen.getByLabelText('Retention policy')).toHaveValue(3);
    expect(screen.getByLabelText('Retention unit')).toHaveValue('daily');
  });

  it('shows the disabled label and disables inputs when backups are off', async () => {
    vi.mocked(useBackupSettings).mockReturnValue({
      data: { ...settings, enabled: false },
      isLoading: false,
    } as never);

    renderPanel();

    expect(screen.getByText('Backups disabled')).toBeInTheDocument();
    expect(screen.getByLabelText('Backup interval (hours)')).toBeDisabled();
    expect(screen.getByLabelText('Retention policy')).toBeDisabled();
    expect(screen.getByLabelText('Retention unit')).toBeDisabled();
  });

  it('enables the inputs after the backup switch is toggled on', async () => {
    vi.mocked(useBackupSettings).mockReturnValue({
      data: { ...settings, enabled: false },
      isLoading: false,
    } as never);
    const user = userEvent.setup();
    renderPanel();

    expect(screen.getByText('Backups disabled')).toBeInTheDocument();
    await user.click(screen.getByRole('switch', { name: 'Toggle backups' }));

    expect(screen.getByText('Scheduled backups enabled')).toBeInTheDocument();
    expect(screen.getByLabelText('Backup interval (hours)')).toBeEnabled();
    expect(screen.getByLabelText('Retention policy')).toBeEnabled();
    expect(screen.getByLabelText('Retention unit')).toBeEnabled();
  });

  it('updates the keep hint when the retention unit changes', async () => {
    const user = userEvent.setup();
    renderPanel();

    expect(
      screen.getByText('Keep the last N daily snapshots')
    ).toBeInTheDocument();

    await user.selectOptions(screen.getByLabelText('Retention unit'), 'weekly');

    expect(
      screen.getByText('Keep the last N weekly snapshots')
    ).toBeInTheDocument();
  });

  it('clears the interval input to its empty placeholder state', () => {
    renderPanel();

    const input = screen.getByLabelText('Backup interval (hours)');
    fireEvent.change(input, { target: { value: '' } });
    expect(input).toHaveValue(null);
  });

  it('accepts a valid numeric interval value', async () => {
    const user = userEvent.setup();
    renderPanel();

    const input = screen.getByLabelText('Backup interval (hours)');
    await user.clear(input);
    await user.type(input, '24');

    expect(input).toHaveValue(24);
  });

  it('ignores negative interval values', () => {
    renderPanel();

    const input = screen.getByLabelText('Backup interval (hours)');
    fireEvent.change(input, { target: { value: '-5' } });
    expect(input).toHaveValue(6);
  });

  it('clears the keep input to its empty placeholder state', () => {
    renderPanel();

    const input = screen.getByLabelText('Retention policy');
    fireEvent.change(input, { target: { value: '' } });
    expect(input).toHaveValue(null);
  });
});

describe('BackupSettingsPanel save', () => {
  it('saves the settings and navigates to provisioning on success', async () => {
    const user = userEvent.setup();
    const mockMutate = vi.fn(
      (_settings: unknown, opts?: { onSuccess?: () => void }) =>
        opts?.onSuccess?.()
    );
    vi.mocked(useUpdateBackupSettings).mockReturnValue({
      mutate: mockMutate,
      isPending: false,
    } as never);

    renderPanel();
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(mockMutate).toHaveBeenCalledWith(
      {
        enabled: true,
        backupIntervalHours: 6,
        keep: 3,
        keepUnit: 'daily',
      },
      expect.anything()
    );
    expect(mockNavigate).toHaveBeenCalledWith('/servers/srv1/provisioning', {
      state: { serverName: 'Survival' },
    });
  });

  it('sends undefined values when the form fields are cleared', async () => {
    const user = userEvent.setup();
    const mockMutate = vi.fn();
    vi.mocked(useUpdateBackupSettings).mockReturnValue({
      mutate: mockMutate,
      isPending: false,
    } as never);

    renderPanel();

    fireEvent.change(screen.getByLabelText('Backup interval (hours)'), {
      target: { value: '' },
    });
    fireEvent.change(screen.getByLabelText('Retention policy'), {
      target: { value: '' },
    });
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(mockMutate).toHaveBeenCalledWith(
      {
        enabled: true,
        backupIntervalHours: undefined,
        keep: undefined,
        keepUnit: undefined,
      },
      expect.anything()
    );
  });

  it('disables the form controls while the mutation is pending', async () => {
    vi.mocked(useUpdateBackupSettings).mockReturnValue({
      mutate: vi.fn(),
      isPending: true,
    } as never);

    renderPanel();

    expect(
      screen.getByRole('switch', { name: 'Toggle backups' })
    ).toBeDisabled();
    expect(screen.getByLabelText('Backup interval (hours)')).toBeDisabled();
    expect(screen.getByLabelText('Retention policy')).toBeDisabled();
    expect(screen.getByLabelText('Retention unit')).toBeDisabled();
    const saveButton = screen.getByRole('button', { name: 'Save' });
    expect(saveButton).toBeDisabled();
    expect(saveButton.querySelector('.animate-spin')).not.toBeNull();
  });
});
