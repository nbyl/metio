import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Header } from './Header';
import { ThemeProvider, useTheme } from '../theme-provider';

vi.mock('react-router-dom', () => ({
  Link: ({
    to,
    children,
    ...props
  }: {
    to: string;
    children: React.ReactNode;
    [key: string]: unknown;
  }) => (
    <a href={to} {...props}>
      {children}
    </a>
  ),
}));

vi.mock('../../hooks/useServerOptions', () => ({
  useServerOptions: vi.fn(() => ({ data: undefined })),
}));

import { useServerOptions } from '../../hooks/useServerOptions';

function ThemeProbe() {
  const { theme } = useTheme();
  return <span data-testid="theme">{theme}</span>;
}

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

  it('shows user email when showUser and email provided', () => {
    render(<Header email="test@example.com" showUser />);
    expect(screen.getByText('test@example.com')).toBeInTheDocument();
  });

  it('shows logout button when showUser and email provided', () => {
    render(<Header email="test@example.com" showUser />);
    const logoutLink = screen.getByRole('link', { name: 'Logout' });
    expect(logoutLink).toBeInTheDocument();
    expect(logoutLink).toHaveAttribute('href', '/auth/logout');
  });

  it('hides user section when showUser is false', () => {
    render(<Header email="test@example.com" showUser={false} />);
    expect(screen.queryByText('test@example.com')).not.toBeInTheDocument();
    expect(
      screen.queryByRole('link', { name: 'Logout' })
    ).not.toBeInTheDocument();
  });

  it('hides user section when email is undefined', () => {
    render(<Header showUser />);
    expect(
      screen.queryByRole('link', { name: 'Logout' })
    ).not.toBeInTheDocument();
  });

  it('renders a semantic header', () => {
    render(<Header />);
    expect(screen.getByRole('banner')).toBeInTheDocument();
  });

  it('renders the theme switcher next to the logout button', () => {
    render(<Header email="test@example.com" showUser />);
    const logout = screen.getByRole('link', { name: 'Logout' });
    const switcher = screen.getByRole('button', { name: 'Change theme' });
    expect(switcher).toBeInTheDocument();
    expect(
      logout.compareDocumentPosition(switcher) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy();
  });

  it('hides the theme switcher with the user section', () => {
    render(<Header email="test@example.com" showUser={false} />);
    expect(
      screen.queryByRole('button', { name: 'Change theme' })
    ).not.toBeInTheDocument();
  });

  it('switches the theme from the dropdown', async () => {
    const user = userEvent.setup();
    render(
      <ThemeProvider>
        <Header email="test@example.com" showUser />
        <ThemeProbe />
      </ThemeProvider>
    );

    await user.click(screen.getByRole('button', { name: 'Change theme' }));
    await user.click(screen.getByRole('menuitemradio', { name: 'Dark' }));

    expect(screen.getByTestId('theme')).toHaveTextContent('dark');
    expect(window.localStorage.getItem('metio-theme')).toBe('dark');
    expect(document.documentElement).toHaveClass('dark');
  });
});
