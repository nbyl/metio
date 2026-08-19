import { describe, it, expect, vi } from 'vitest';
import { useState } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Tabs, TabsList, TabsTrigger, TabsContent } from './Tabs';

function TabsHarness() {
  const [value, setValue] = useState('settings');
  return (
    <Tabs value={value} onValueChange={setValue}>
      <TabsList>
        <TabsTrigger value="settings">Settings</TabsTrigger>
        <TabsTrigger value="backup">Backup</TabsTrigger>
      </TabsList>
      <TabsContent value="settings">Settings content</TabsContent>
      <TabsContent value="backup">Backup content</TabsContent>
    </Tabs>
  );
}

function renderTabs(onValueChange = vi.fn()) {
  return render(
    <Tabs value="settings" onValueChange={onValueChange}>
      <TabsList>
        <TabsTrigger value="settings">Settings</TabsTrigger>
        <TabsTrigger value="backup">Backup</TabsTrigger>
      </TabsList>
      <TabsContent value="settings">Settings content</TabsContent>
      <TabsContent value="backup">Backup content</TabsContent>
    </Tabs>
  );
}

describe('Tabs', () => {
  it('renders tablist with tabs and the active panel', () => {
    renderTabs();

    expect(screen.getByRole('tablist')).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Settings' })).toBeInTheDocument();
    expect(screen.getByRole('tab', { name: 'Backup' })).toBeInTheDocument();
    expect(screen.getByText('Settings content')).toBeInTheDocument();
  });

  it('marks the active tab via aria-selected', () => {
    renderTabs();

    expect(screen.getByRole('tab', { name: 'Settings' })).toHaveAttribute(
      'aria-selected',
      'true'
    );
    expect(screen.getByRole('tab', { name: 'Backup' })).toHaveAttribute(
      'aria-selected',
      'false'
    );
  });

  it('requests tab changes via onValueChange on click', async () => {
    const user = userEvent.setup();
    const handleChange = vi.fn();
    renderTabs(handleChange);

    await user.click(screen.getByRole('tab', { name: 'Backup' }));
    expect(handleChange).toHaveBeenCalledWith('backup');
  });

  it('shows a new tab panel when the tab selection changes', async () => {
    const user = userEvent.setup();
    render(<TabsHarness />);

    await user.click(screen.getByRole('tab', { name: 'Backup' }));
    expect(screen.getByText('Backup content')).toBeInTheDocument();
  });

  it('supports keyboard navigation between tabs', async () => {
    const user = userEvent.setup();
    renderTabs();

    await user.tab();
    expect(screen.getByRole('tab', { name: 'Settings' })).toHaveFocus();

    await user.keyboard('{ArrowRight}');
    expect(screen.getByRole('tab', { name: 'Backup' })).toHaveFocus();
  });

  it('renders a disabled tab that is not selectable', async () => {
    const user = userEvent.setup();
    const handleChange = vi.fn();
    render(
      <Tabs value="settings" onValueChange={handleChange}>
        <TabsList>
          <TabsTrigger value="settings">Settings</TabsTrigger>
          <TabsTrigger value="backup" disabled>
            Backup
          </TabsTrigger>
        </TabsList>
        <TabsContent value="settings">Settings content</TabsContent>
        <TabsContent value="backup">Backup content</TabsContent>
      </Tabs>
    );

    const backupTab = screen.getByRole('tab', { name: 'Backup' });
    expect(backupTab).toBeDisabled();
    await user.click(backupTab);
    expect(handleChange).not.toHaveBeenCalled();
  });
});
