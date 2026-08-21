import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Layout } from './Layout';

describe('Layout', () => {
  it('renders children', () => {
    render(<Layout>Page content</Layout>);
    expect(screen.getByText('Page content')).toBeInTheDocument();
  });

  it('renders in light theme', () => {
    const { container } = render(<Layout>Content</Layout>);
    const wrapper = container.firstChild as HTMLElement;
    expect(wrapper).not.toHaveClass('dark');
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
