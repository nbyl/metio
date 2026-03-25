import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { Card, CardHeader, CardTitle, CardContent } from './Card';

describe('Card', () => {
  it('renders children', () => {
    render(<Card>Card content</Card>);
    expect(screen.getByText('Card content')).toBeInTheDocument();
  });

  it('applies card class', () => {
    render(<Card>Content</Card>);
    const card = screen.getByText('Content');
    expect(card).toHaveClass('card');
  });

  it('merges custom className', () => {
    render(<Card className="custom-card">Content</Card>);
    const card = screen.getByText('Content');
    expect(card).toHaveClass('card', 'custom-card');
  });

  it('matches snapshot', () => {
    const { container } = render(<Card>Card content</Card>);
    expect(container).toMatchSnapshot();
  });
});

describe('CardHeader', () => {
  it('renders children', () => {
    render(<CardHeader>Header content</CardHeader>);
    expect(screen.getByText('Header content')).toBeInTheDocument();
  });

  it('applies card-header class', () => {
    render(<CardHeader>Header</CardHeader>);
    const header = screen.getByText('Header');
    expect(header).toHaveClass('card-header');
  });

  it('merges custom className', () => {
    render(<CardHeader className="custom-header">Header</CardHeader>);
    const header = screen.getByText('Header');
    expect(header).toHaveClass('card-header', 'custom-header');
  });
});

describe('CardTitle', () => {
  it('renders children', () => {
    render(<CardTitle>Title text</CardTitle>);
    expect(screen.getByText('Title text')).toBeInTheDocument();
  });

  it('applies card-title class', () => {
    render(<CardTitle>Title</CardTitle>);
    const title = screen.getByRole('heading', { level: 2 });
    expect(title).toHaveClass('card-title');
  });

  it('merges custom className', () => {
    render(<CardTitle className="custom-title">Title</CardTitle>);
    const title = screen.getByRole('heading');
    expect(title).toHaveClass('card-title', 'custom-title');
  });
});

describe('CardContent', () => {
  it('renders children', () => {
    render(<CardContent>Content text</CardContent>);
    expect(screen.getByText('Content text')).toBeInTheDocument();
  });

  it('applies card-content class', () => {
    render(<CardContent>Content</CardContent>);
    const content = screen.getByText('Content');
    expect(content).toHaveClass('card-content');
  });

  it('merges custom className', () => {
    render(<CardContent className="custom-content">Content</CardContent>);
    const content = screen.getByText('Content');
    expect(content).toHaveClass('card-content', 'custom-content');
  });
});

describe('Card composition', () => {
  it('matches snapshot with full composition', () => {
    const { container } = render(
      <Card>
        <CardHeader>
          <CardTitle>Server Status</CardTitle>
        </CardHeader>
        <CardContent>
          <p>Server is running</p>
        </CardContent>
      </Card>
    );
    expect(container).toMatchSnapshot();
  });
});
