import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Header } from './Header';

vi.mock('react-router-dom', () => {
  const actual = vi.importActual('react-router-dom');
  return {
    ...actual,
    NavLink: ({
      to,
      children,
      className,
      ...props
    }: {
      to: string;
      children: React.ReactNode;
      className?: string | ((state: { isActive: boolean }) => string);
      [key: string]: unknown;
    }) => {
      const resolved =
        typeof className === 'function'
          ? className({ isActive: to === '/' })
          : className;
      return (
        <a href={to} className={resolved} {...props}>
          {children}
        </a>
      );
    },
  };
});

vi.mock('../../hooks/useServerOptions', () => ({
  useServerOptions: vi.fn(() => ({ data: undefined })),
}));

import { useServerOptions } from '../../hooks/useServerOptions';

describe('Header', () => {
  beforeEach(() => {
    vi.mocked(useServerOptions).mockReturnValue({ data: undefined } as never);
  });

  it('renders the controller version when available', () => {
    vi.mocked(useServerOptions).mockReturnValue({
      data: { controllerVersion: 'v1.7.0' },
    } as never);
    render(<Header />);
    expect(screen.getByText('Version v1.7.0')).toBeInTheDocument();
  });

  it('hides the controller version when unavailable', () => {
    render(<Header />);
    expect(screen.queryByText(/Version/)).not.toBeInTheDocument();
  });

  it('renders title "Metio"', () => {
    render(<Header />);
    expect(screen.getByRole('heading', { name: /Metio/i })).toBeInTheDocument();
  });

  it('renders subtitle', () => {
    render(<Header />);
    expect(screen.getByText('Minecraft Server Controller')).toBeInTheDocument();
  });

  it('renders Gamepad2 icon', () => {
    const { container } = render(<Header />);
    const icon = container.querySelector('svg');
    expect(icon).toBeInTheDocument();
    expect(icon).toHaveAttribute('aria-hidden', 'true');
  });

  it('shows user menu button when showUser and email provided', () => {
    render(<Header email="test@example.com" showUser />);
    expect(
      screen.getByRole('button', { name: 'User menu' })
    ).toBeInTheDocument();
  });

  it('hides user menu when showUser is false', () => {
    render(<Header email="test@example.com" showUser={false} />);
    expect(
      screen.queryByRole('button', { name: 'User menu' })
    ).not.toBeInTheDocument();
  });

  it('hides user menu when email is undefined', () => {
    render(<Header showUser />);
    expect(
      screen.queryByRole('button', { name: 'User menu' })
    ).not.toBeInTheDocument();
  });

  it('renders a semantic header', () => {
    render(<Header />);
    expect(screen.getByRole('banner')).toBeInTheDocument();
  });

  it('shows nav links when showUser is true', () => {
    render(<Header email="test@example.com" showUser />);
    expect(screen.getByRole('link', { name: 'Servers' })).toHaveAttribute(
      'href',
      '/'
    );
    expect(screen.getByRole('link', { name: 'Backups' })).toHaveAttribute(
      'href',
      '/backups'
    );
  });

  it('hides nav links when showUser is false', () => {
    render(<Header showUser={false} />);
    expect(
      screen.queryByRole('link', { name: 'Servers' })
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole('link', { name: 'Backups' })
    ).not.toBeInTheDocument();
  });

  it('shows theme submenu in the user menu', async () => {
    const user = userEvent.setup();
    render(<Header email="test@example.com" showUser />);

    await user.click(screen.getByRole('button', { name: 'User menu' }));
    expect(screen.getByRole('menuitem', { name: 'Theme' })).toBeInTheDocument();
  });

  it('shows email in the user menu', async () => {
    const user = userEvent.setup();
    render(<Header email="test@example.com" showUser />);

    await user.click(screen.getByRole('button', { name: 'User menu' }));
    expect(screen.getByText('test@example.com')).toBeInTheDocument();
  });

  it('shows logout in the user menu', async () => {
    const user = userEvent.setup();
    render(<Header email="test@example.com" showUser />);

    await user.click(screen.getByRole('button', { name: 'User menu' }));
    const logoutLink = screen.getByRole('menuitem', { name: 'Logout' });
    expect(logoutLink).toBeInTheDocument();
    expect(logoutLink).toHaveAttribute('href', '/auth/logout');
  });
});
