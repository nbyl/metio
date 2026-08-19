import { describe, it, expect, vi } from 'vitest';
import { useState } from 'react';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Tabs, TabList, Tab, TabPanel } from './Tabs';

function TabsHarness() {
  const [value, setValue] = useState('settings');
  return (
    <Tabs value={value} onValueChange={setValue}>
      <TabList>
        <Tab value="settings">Settings</Tab>
        <Tab value="backup">Backup</Tab>
      </TabList>
      <TabPanel value="settings">Settings content</TabPanel>
      <TabPanel value="backup">Backup content</TabPanel>
    </Tabs>
  );
}

function renderTabs(onValueChange = vi.fn()) {
  return render(
    <Tabs value="settings" onValueChange={onValueChange}>
      <TabList>
        <Tab value="settings">Settings</Tab>
        <Tab value="backup">Backup</Tab>
      </TabList>
      <TabPanel value="settings">Settings content</TabPanel>
      <TabPanel value="backup">Backup content</TabPanel>
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

    expect(screen.getByText('Settings content')).toBeInTheDocument();
    await user.click(screen.getByRole('tab', { name: 'Backup' }));

    expect(screen.getByRole('tab', { name: 'Backup' })).toHaveAttribute(
      'aria-selected',
      'true'
    );
    expect(screen.getByText('Backup content')).toBeInTheDocument();
  });

  it('supports keyboard navigation between tabs', async () => {
    const user = userEvent.setup();
    renderTabs();

    const settingsTab = screen.getByRole('tab', { name: 'Settings' });
    await user.tab();

    expect(settingsTab).toHaveFocus();

    await user.keyboard('{ArrowRight}');
    expect(screen.getByRole('tab', { name: 'Backup' })).toHaveFocus();
  });

  it('renders a disabled tab that is not selectable', async () => {
    const user = userEvent.setup();
    const handleChange = vi.fn();
    render(
      <Tabs value="settings" onValueChange={handleChange}>
        <TabList>
          <Tab value="settings">Settings</Tab>
          <Tab value="backup" disabled>
            Backup
          </Tab>
        </TabList>
        <TabPanel value="settings">Settings content</TabPanel>
        <TabPanel value="backup">Backup content</TabPanel>
      </Tabs>
    );

    const backupTab = screen.getByRole('tab', { name: 'Backup' });
    expect(backupTab).toBeDisabled();

    await user.click(backupTab);
    expect(handleChange).not.toHaveBeenCalled();
  });
});
