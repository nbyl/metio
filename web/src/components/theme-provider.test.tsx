import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, act, fireEvent } from '@testing-library/react';
import { ThemeProvider, useTheme } from './theme-provider';
import { setPrefersDark } from '../test/setup';

function Probe() {
  const { theme, resolvedTheme, setTheme } = useTheme();
  return (
    <div>
      <span data-testid="theme">{theme}</span>
      <span data-testid="resolved">{resolvedTheme}</span>
      <button onClick={() => setTheme('light')}>light</button>
      <button onClick={() => setTheme('dark')}>dark</button>
      <button onClick={() => setTheme('system')}>system</button>
    </div>
  );
}

function renderProvider() {
  return render(
    <ThemeProvider>
      <Probe />
    </ThemeProvider>,
  );
}

describe('ThemeProvider', () => {
  beforeEach(() => {
    window.localStorage.clear();
    setPrefersDark(false);
    document.documentElement.className = '';
  });

  it('defaults to system mode and resolves to light on a light OS', () => {
    renderProvider();
    expect(screen.getByTestId('theme')).toHaveTextContent('system');
    expect(screen.getByTestId('resolved')).toHaveTextContent('light');
    expect(document.documentElement).not.toHaveClass('dark');
  });

  it('resolves system to dark when the OS prefers dark', () => {
    setPrefersDark(true);
    renderProvider();
    expect(screen.getByTestId('resolved')).toHaveTextContent('dark');
    expect(document.documentElement).toHaveClass('dark');
  });

  it('follows live OS preference changes while in system mode', () => {
    renderProvider();
    expect(document.documentElement).not.toHaveClass('dark');

    act(() => setPrefersDark(true));

    expect(screen.getByTestId('resolved')).toHaveTextContent('dark');
    expect(document.documentElement).toHaveClass('dark');

    act(() => setPrefersDark(false));

    expect(document.documentElement).not.toHaveClass('dark');
  });

  it('applies and persists an explicit theme choice', () => {
    renderProvider();
    fireEvent.click(screen.getByRole('button', { name: 'dark' }));

    expect(screen.getByTestId('theme')).toHaveTextContent('dark');
    expect(document.documentElement).toHaveClass('dark');
    expect(window.localStorage.getItem('metio-theme')).toBe('dark');
  });

  it('keeps an explicit choice when the OS preference changes', () => {
    renderProvider();
    act(() => {
      screen.getByRole('button', { name: 'light' }).click();
    });

    act(() => setPrefersDark(true));

    expect(screen.getByTestId('resolved')).toHaveTextContent('light');
    expect(document.documentElement).not.toHaveClass('dark');
  });

  it('restores a persisted choice on reload', () => {
    window.localStorage.setItem('metio-theme', 'dark');
    renderProvider();

    expect(screen.getByTestId('theme')).toHaveTextContent('dark');
    expect(document.documentElement).toHaveClass('dark');
  });

  it('falls back to system when storage holds an unknown value', () => {
    window.localStorage.setItem('metio-theme', 'sepia');
    renderProvider();

    expect(screen.getByTestId('theme')).toHaveTextContent('system');
    expect(document.documentElement).not.toHaveClass('dark');
  });
});
