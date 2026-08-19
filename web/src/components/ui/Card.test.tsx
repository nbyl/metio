import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Card, CardHeader, CardTitle, CardContent } from './Card';

describe('Card', () => {
  it('renders children', () => {
    render(<Card>Card content</Card>);
    expect(screen.getByText('Card content')).toBeInTheDocument();
  });

  it('forwards extra props', () => {
    render(<Card data-testid="card">Content</Card>);
    expect(screen.getByTestId('card')).toBeInTheDocument();
  });
});

describe('CardHeader', () => {
  it('renders children', () => {
    render(<CardHeader>Header content</CardHeader>);
    expect(screen.getByText('Header content')).toBeInTheDocument();
  });
});

describe('CardTitle', () => {
  it('renders children as a level-2 heading', () => {
    render(<CardTitle>Title text</CardTitle>);
    expect(screen.getByRole('heading', { level: 2 })).toHaveTextContent(
      'Title text'
    );
  });
});

describe('CardContent', () => {
  it('renders children', () => {
    render(<CardContent>Content text</CardContent>);
    expect(screen.getByText('Content text')).toBeInTheDocument();
  });
});

describe('Card composition', () => {
  it('renders a complete card with header, title and content', () => {
    render(
      <Card>
        <CardHeader>
          <CardTitle>Server Status</CardTitle>
        </CardHeader>
        <CardContent>
          <p>Server is running</p>
        </CardContent>
      </Card>
    );

    expect(
      screen.getByRole('heading', { level: 2, name: 'Server Status' })
    ).toBeInTheDocument();
    expect(screen.getByText('Server is running')).toBeInTheDocument();
  });
});
