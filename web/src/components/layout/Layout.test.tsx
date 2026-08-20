import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Layout } from './Layout';

describe('Layout', () => {
  it('renders children', () => {
    render(<Layout>Page content</Layout>);
    expect(screen.getByText('Page content')).toBeInTheDocument();
  });

  it('applies dark theme classes', () => {
    const { container } = render(<Layout>Content</Layout>);
    const wrapper = container.firstChild as HTMLElement;
    expect(wrapper).toHaveClass('dark', 'min-h-screen', 'bg-background');
  });

  it('contains a semantic main shell with token-based layout classes', () => {
    render(<Layout>Content</Layout>);
    const main = screen.getByRole('main');
    expect(main).toHaveClass('mx-auto', 'max-w-4xl', 'space-y-6');
  });

  it('merges custom className on the main shell', () => {
    render(<Layout className="custom-layout">Content</Layout>);
    expect(screen.getByRole('main')).toHaveClass('custom-layout');
  });
});
