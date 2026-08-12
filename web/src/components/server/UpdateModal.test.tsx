import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { UpdateModal } from './UpdateModal';
import type { ServerConfig } from '../../types/server';
import { useServerOptions } from '../../hooks/useServerOptions';

vi.mock('../../hooks/useServerOptions', () => ({
  useServerOptions: vi.fn(),
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

function renderModal(overrides: Partial<ServerConfig> = {}) {
  return render(
    <UpdateModal
      open
      config={{ ...config, ...overrides }}
      currentInfraVersion={1}
      outdated={false}
      onClose={vi.fn()}
      onUpdate={vi.fn()}
      isPending={false}
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
});

describe('UpdateModal minecraft version field', () => {
  it('renders a dropdown of the available versions', () => {
    renderModal();

    const select = screen.getByDisplayValue('1.21.11');
    expect(select.tagName).toBe('SELECT');
    expect(Array.from(select.querySelectorAll('option')).map((o) => o.value)).toEqual([
      '26.2',
      '1.21.11',
      '1.21.10',
    ]);
  });

  it('keeps the versions in the order returned by the API', () => {
    renderModal();

    const select = screen.getByDisplayValue('1.21.11');
    expect(Array.from(select.querySelectorAll('option')).map((o) => o.value)[0]).toBe('26.2');
  });

  it('includes the current version when the API no longer offers it', () => {
    renderModal({ minecraftVersion: '1.7.10' });

    const select = screen.getByDisplayValue('1.7.10');
    expect(Array.from(select.querySelectorAll('option')).map((o) => o.value)).toEqual([
      '1.7.10',
      '26.2',
      '1.21.11',
      '1.21.10',
    ]);
  });
});

describe('UpdateModal machine type field', () => {
  it('renders a dropdown of the available machine types', () => {
    renderModal();

    const select = screen.getAllByRole('combobox')[0] as HTMLSelectElement;
    expect(select.tagName).toBe('SELECT');
    expect(select.value).toBe('e2-small');
    expect(Array.from(select.querySelectorAll('option')).map((o) => o.value)).toEqual([
      'e2-small',
      'e2-medium',
    ]);
  });

  it('keeps the current machine type when the API no longer offers it', () => {
    renderModal({ machineType: 'e2-standard-2' });

    const select = screen.getAllByRole('combobox')[0] as HTMLSelectElement;
    expect(Array.from(select.querySelectorAll('option')).map((o) => o.value)).toEqual([
      'e2-standard-2',
      'e2-small',
      'e2-medium',
    ]);
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
