import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/react';
import { Layout } from './Layout';
import { ThemeProvider, useTheme } from '../theme-provider';

function DarkProbe() {
  const { setTheme } = useTheme();
  return <button onClick={() => setTheme('dark')}>dark</button>;
}

describe('Layout', () => {
  beforeEach(() => {
    window.localStorage.clear();
    document.documentElement.className = '';
  });

  it('renders children', () => {
    render(<Layout>Page content</Layout>);
    expect(screen.getByText('Page content')).toBeInTheDocument();
  });

  it('renders in light theme by default', () => {
    render(
      <ThemeProvider>
        <Layout>Content</Layout>
      </ThemeProvider>,
    );
    expect(document.documentElement).not.toHaveClass('dark');
  });

  it('renders in dark theme when the provider switches', () => {
    render(
      <ThemeProvider>
        <Layout>
          Content
          <DarkProbe />
        </Layout>
      </ThemeProvider>,
    );
    fireEvent.click(screen.getByRole('button', { name: 'dark' }));
    expect(document.documentElement).toHaveClass('dark');
  });

  it('exposes a semantic main region', () => {
    render(<Layout>Content</Layout>);
    expect(screen.getByRole('main')).toBeInTheDocument();
  });

  it('merges custom className on the main shell', () => {
    render(<Layout className="custom-layout">Content</Layout>);
    expect(screen.getByRole('main')).toHaveClass('custom-layout');
  });
});
