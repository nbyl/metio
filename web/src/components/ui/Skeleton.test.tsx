import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Skeleton } from './Skeleton';

describe('Skeleton', () => {
  it('renders with default classes', () => {
    render(<Skeleton data-testid="skeleton" />);
    const skeleton = screen.getByTestId('skeleton');

    expect(skeleton).toBeInTheDocument();
    expect(skeleton).toHaveClass('skeleton');
    expect(skeleton).toHaveClass('animate-pulse');
    expect(skeleton).toHaveClass('rounded');
  });

  it('accepts custom className', () => {
    render(<Skeleton data-testid="skeleton" className="h-4 w-32" />);
    const skeleton = screen.getByTestId('skeleton');

    expect(skeleton).toHaveClass('h-4');
    expect(skeleton).toHaveClass('w-32');
    expect(skeleton).toHaveClass('skeleton');
  });

  it('forwards additional props', () => {
    render(
      <Skeleton
        data-testid="skeleton"
        aria-label="Loading content"
        role="progressbar"
      />
    );
    const skeleton = screen.getByTestId('skeleton');

    expect(skeleton).toHaveAttribute('aria-label', 'Loading content');
    expect(skeleton).toHaveAttribute('role', 'progressbar');
  });

  it('matches snapshot', () => {
    const { container } = render(
      <Skeleton className="h-4 w-full" data-testid="skeleton" />
    );
    expect(container.firstChild).toMatchSnapshot();
  });
});
