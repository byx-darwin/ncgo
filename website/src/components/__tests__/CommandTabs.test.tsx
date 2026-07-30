import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import CommandTabs, { CommandTab } from '../CommandTabs';

const tabs: CommandTab[] = [
  { id: 'doctor', label: 'doctor', lines: [{ kind: 'command', text: 'ncgo doctor' }, { kind: 'output', text: 'hz: ok' }] },
  { id: 'infra', label: 'add infra', lines: [{ kind: 'command', text: 'ncgo add infra redis' }, { kind: 'output', text: 'redis: added' }] },
];

describe('CommandTabs', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('shows first tab content by default', () => {
    window.matchMedia = vi.fn().mockImplementation((q: string) => ({
      matches: q.includes('reduce'),
      media: q,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
    render(<CommandTabs tabs={tabs} />);
    expect(screen.getByText('ncgo doctor')).toBeInTheDocument();
  });

  it('switches content when a tab is clicked', async () => {
    window.matchMedia = vi.fn().mockImplementation((q: string) => ({
      matches: q.includes('reduce'),
      media: q,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<CommandTabs tabs={tabs} />);
    await user.click(screen.getByRole('tab', { name: 'add infra' }));
    expect(screen.getByText('ncgo add infra redis')).toBeInTheDocument();
  });
});
