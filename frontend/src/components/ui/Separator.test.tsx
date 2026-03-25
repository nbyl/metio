import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Separator } from './Separator';

describe('Separator', () => {
  it('renders separator element', () => {
    render(<Separator />);
    expect(screen.getByRole('separator')).toBeInTheDocument();
  });

  it('applies separator class', () => {
    render(<Separator />);
    const separator = screen.getByRole('separator');
    expect(separator).toHaveClass('separator');
  });

  it('merges custom className', () => {
    render(<Separator className="my-4" />);
    const separator = screen.getByRole('separator');
    expect(separator).toHaveClass('separator', 'my-4');
  });

  it('matches snapshot', () => {
    const { container } = render(<Separator />);
    expect(container).toMatchSnapshot();
  });
});
