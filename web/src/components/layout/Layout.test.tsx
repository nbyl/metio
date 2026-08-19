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

  it('contains container div', () => {
    const { container } = render(<Layout>Content</Layout>);
    const containerDiv = container.querySelector('.container');
    expect(containerDiv).toBeInTheDocument();
  });

  it('merges custom className on container', () => {
    const { container } = render(
      <Layout className="custom-layout">Content</Layout>
    );
    const containerDiv = container.querySelector('.container');
    expect(containerDiv).toHaveClass('container', 'custom-layout');
  });

  it('matches snapshot', () => {
    const { container } = render(
      <Layout>
        <h1>Test Page</h1>
        <p>Page content goes here</p>
      </Layout>
    );
    expect(container).toMatchSnapshot();
  });
});
