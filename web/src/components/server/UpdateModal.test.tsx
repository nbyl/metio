import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { UpdateModal } from './UpdateModal';
import type { ServerConfig } from '../../types/server';

vi.mock('../../hooks/useServerOptions', () => ({
  useServerOptions: vi.fn(),
}));

import { useServerOptions } from '../../hooks/useServerOptions';

const config: ServerConfig = {
  name: 'test-server',
  region: 'us-central1',
  zone: 'us-central1-a',
  machineType: 'e2-small',
  minecraftVersion: '1.21.11',
  diskSizeGB: 50,
} as ServerConfig;

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

describe('UpdateModal minecraft version field', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useServerOptions).mockReturnValue({
      data: {
        minecraftVersions: ['26.2', '1.21.11', '1.21.10'],
        machineTypes: [
          { id: 'e2-small', vcpus: 2, memoryGB: 2 },
          { id: 'e2-medium', vcpus: 2, memoryGB: 4 },
        ],
      },
      isLoading: false,
    } as never);
  });

  it('renders a dropdown of the available machine types instead of a free-text input', () => {
    renderModal();

    const select = screen.getByDisplayValue('e2-small');
    expect(select.tagName).toBe('SELECT');

    const options = Array.from(select.querySelectorAll('option')).map((o) => o.value);
    expect(options).toEqual(['e2-small', 'e2-medium']);
  });

  it('keeps the current machine type when the API no longer offers it', () => {
    renderModal({ machineType: 'e2-standard-2' });

    const select = screen.getByDisplayValue('e2-standard-2');
    const options = Array.from(select.querySelectorAll('option')).map((o) => o.value);

    // The current type is prepended so opening the modal cannot silently
    // switch the server to a different type.
    expect(options).toEqual(['e2-standard-2', 'e2-small', 'e2-medium']);
    expect((select as HTMLSelectElement).value).toBe('e2-standard-2');
  });

  it('includes the current type even when the API no longer offers it, in correct order', () => {
    renderModal({ machineType: 'e2-small' });

    const select = screen.getByDisplayValue('e2-small');
    const options = Array.from(select.querySelectorAll('option')).map((o) => o.value);

    // The current type appears first so opening the modal cannot silently
    // switch the server to a different type.
    expect(options).toEqual(['e2-small', 'e2-medium']);
    expect((select as HTMLSelectElement).value).toBe('e2-small');
  });

  it('disables the dropdown while options are loading', () => {
    vi.mocked(useServerOptions).mockReturnValue({
      data: undefined,
      isLoading: true,
    } as never);

    renderModal();

    const select = screen.getByDisplayValue('e2-small');
    expect(select).toBeDisabled();
  });
});

  it('renders a dropdown of the available versions instead of a free-text input', () => {
    renderModal();

    const select = screen.getByDisplayValue('1.21.11');
    expect(select.tagName).toBe('SELECT');

    const options = Array.from(select.querySelectorAll('option')).map((o) => o.value);
    expect(options).toEqual(['26.2', '1.21.11', '1.21.10']);
  });

  it('keeps the versions in the order the API returned them', () => {
    renderModal();

    const select = screen.getByDisplayValue('1.21.11');
    const options = Array.from(select.querySelectorAll('option')).map((o) => o.value);
    expect(options[0]).toBe('26.2');
  });

  it('includes the current version when the API no longer offers it', () => {
    renderModal({ minecraftVersion: '1.7.10' });

    const select = screen.getByDisplayValue('1.7.10');
    const options = Array.from(select.querySelectorAll('option')).map((o) => o.value);

    // The current version is prepended so opening the modal cannot silently
    // switch the server to a different version.
    expect(options).toEqual(['1.7.10', '26.2', '1.21.11', '1.21.10']);
    expect((select as HTMLSelectElement).value).toBe('1.7.10');
  });

  it('disables the dropdown while options are loading', () => {
    vi.mocked(useServerOptions).mockReturnValue({
      data: undefined,
      isLoading: true,
    } as never);

    renderModal();

    const select = screen.getByDisplayValue('1.21.11');
    expect(select).toBeDisabled();
  });
});
