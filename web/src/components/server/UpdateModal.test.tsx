import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import { UpdateModal } from './UpdateModal';
import type { ServerConfig } from '../../types/server';

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

describe('UpdateModal > minecraft version field', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    const mockData = {
      minecraftVersions: ['26.2', '1.21.11', '1.21.10'],
      machineTypes: [
        { id: 'e2-small', vcpus: 2, memoryGB: 2 },
        { id: 'e2-medium', vcpus: 2, memoryGB: 4 },
      ],
    };
    vi.useFakeFn(UpdateModal);
    // Mock the useServerOptions hook manually
    const mockUseServerOptions = vi.fn().mockReturnValue({
      data: mockData,
      isLoading: false,
    });
    globalThis.useServerOptions = mockUseServerOptions;
  });

  it('renders a dropdown of the available machine types instead of a free-text input', () => {
    const { rerender } = renderModal();

    const select = screen.getByDisplayValue('e2-small');
    expect(select.tagName).toBe('SELECT');

    const options = Array.from(select.querySelectorAll('option')).map((o) => o.value);
    expect(options).toEqual(['e2-small', 'e2-medium']);
  });

  it('keeps the current machine type when the API no longer offers it', () => {
    rerender({ machineType: 'e2-standard-2' });

    const select = screen.getByDisplayValue('e2-standard-2');
    const options = Array.from(select.querySelectorAll('option')).map((o) => o.value);

    expect(options).toEqual(['e2-standard-2', 'e2-small', 'e2-medium']);
    expect((select as HTMLSelectElement).value).toBe('e2-standard-2');
  });

  it('includes the current type even when the API no longer offers it, in correct order', () => {
    rerender({ machineType: 'e2-small' });

    const select = screen.getByDisplayValue('e2-small');
    const options = Array.from(select.querySelectorAll('option')).map((o) => o.value);

    expect(options).toEqual(['e2-small', 'e2-medium']);
    expect((select as HTMLSelectElement).value).toBe('e2-small');
  });

  it('disables the dropdown while options are loading', () => {
    // Mock without data
    globalThis.useServerOptions = vi.fn().mockReturnValue({
      data: undefined,
      isLoading: true,
    });
    rerender();

    const select = screen.getByDisplayValue('e2-small');
    expect(select).toBeDisabled();
  });
});

describe('UpdateModal > machine type field', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    const mockData = {
      minecraftVersions: ['26.2', '1.21.11', '1.21.10'],
      machineTypes: [
        { id: 'e2-small', vcpus: 2, memoryGB: 2 },
        { id: 'e2-medium', vcpus: 2, memoryGB: 4 },
      ],
    };
    globalThis.useServerOptions = vi.fn().mockReturnValue({
      data: mockData,
      isLoading: false,
    });
  });

  it('renders a dropdown of the available machine types instead of a free-text input', () => {
    const { rerender } = renderModal();

    const select = screen.getByDisplayValue('e2-small');
    expect(select.tagName).toBe('SELECT');

    const options = Array.from(select.querySelectorAll('option')).map((o) => o.value);
    expect(options).toEqual(['e2-small', 'e2-medium']);
  });

  it('keeps the current machine type when the API no longer offers it', () => {
    rerender({ machineType: 'e2-standard-2' });

    const select = screen.getByDisplayValue('e2-standard-2');
    const options = Array.from(select.querySelectorAll('option')).map((o) => o.value);

    expect(options).toEqual(['e2-standard-2', 'e2-small', 'e2-medium']);
    expect((select as HTMLSelectElement).value).toBe('e2-standard-2');
  });

  it('includes the current type even when the API no longer offers it, in correct order', () => {
    rerender({ machineType: 'e2-small' });

    const select = screen.getByDisplayValue('e2-small');
    const options = Array.from(select.querySelectorAll('option')).map((o) => o.value);

    expect(options).toEqual(['e2-small', 'e2-medium']);
    expect((select as HTMLSelectElement).value).toBe('e2-small');
  });

  it('disables the dropdown while options are loading', () => {
    globalThis.useServerOptions = vi.fn().mockReturnValue({
      data: undefined,
      isLoading: true,
    });
    rerender();

    const select = screen.getByDisplayValue('e2-small');
    expect(select).toBeDisabled();
  });
});