import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Header } from './Header';

describe('Header', () => {
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
    expect(screen.queryByRole('link', { name: 'Logout' })).not.toBeInTheDocument();
  });

  it('hides user section when email is undefined', () => {
    render(<Header showUser />);
    expect(screen.queryByRole('link', { name: 'Logout' })).not.toBeInTheDocument();
  });

  // Snapshot tests
  it('matches snapshot (with user)', () => {
    const { container } = render(<Header email="user@example.com" showUser />);
    expect(container).toMatchSnapshot();
  });

  it('matches snapshot (without user)', () => {
    const { container } = render(<Header />);
    expect(container).toMatchSnapshot();
  });
});
