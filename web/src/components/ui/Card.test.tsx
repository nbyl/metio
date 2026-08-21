import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from './Card';

describe('Card', () => {
  it('renders children and the canonical data slots', () => {
    render(
      <Card>
        <CardHeader>
          <CardTitle>Server Status</CardTitle>
          <CardDescription>Current state</CardDescription>
          <CardAction>Actions</CardAction>
        </CardHeader>
        <CardContent>Server is running</CardContent>
        <CardFooter>Footer</CardFooter>
      </Card>
    );

    expect(screen.getByText('Server Status')).toHaveAttribute(
      'data-slot',
      'card-title'
    );
    expect(screen.getByText('Current state')).toHaveAttribute(
      'data-slot',
      'card-description'
    );
    expect(screen.getByText('Actions')).toHaveAttribute(
      'data-slot',
      'card-action'
    );
    expect(screen.getByText('Server is running')).toHaveAttribute(
      'data-slot',
      'card-content'
    );
    expect(screen.getByText('Footer')).toHaveAttribute(
      'data-slot',
      'card-footer'
    );
  });

  it('supports the small card size', () => {
    render(<Card size="sm">Small card</Card>);

    expect(screen.getByText('Small card')).toHaveAttribute('data-size', 'sm');
  });
});
