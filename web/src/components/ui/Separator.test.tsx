import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Separator } from './Separator';

describe('Separator', () => {
  it('renders a decorative separator by default', () => {
    render(<Separator data-testid="separator" />);

    expect(screen.getByTestId('separator')).toHaveAttribute(
      'data-slot',
      'separator'
    );
  });

  it('can expose the separator role when non-decorative', () => {
    render(<Separator decorative={false} data-testid="separator" />);

    expect(screen.getByRole('separator')).toHaveAttribute(
      'data-orientation',
      'horizontal'
    );
  });
});
