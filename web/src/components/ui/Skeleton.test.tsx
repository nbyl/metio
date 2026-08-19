import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Skeleton } from './Skeleton';

describe('Skeleton', () => {
  it('renders an empty placeholder div', () => {
    render(<Skeleton data-testid="skeleton" />);
    expect(screen.getByTestId('skeleton')).toBeInTheDocument();
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
});
