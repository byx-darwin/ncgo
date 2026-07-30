import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import Terminal, { TermLine } from '../Terminal';

const lines: TermLine[] = [
  { kind: 'command', text: 'ncgo new user-api' },
  { kind: 'output', text: 'manifest.yaml' },
  { kind: 'highlight', text: 'AGENTS.md' },
];

describe('Terminal', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('renders all lines when reduced motion is preferred', () => {
    window.matchMedia = vi.fn().mockImplementation((q: string) => ({
      matches: q.includes('reduce'),
      media: q,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
    render(<Terminal lines={lines} />);
    expect(screen.getByText('ncgo new user-api')).toBeInTheDocument();
    expect(screen.getByText('manifest.yaml')).toBeInTheDocument();
    expect(screen.getByText('AGENTS.md')).toBeInTheDocument();
  });

  it('eventually reveals all lines when animating', () => {
    render(<Terminal lines={lines} />);
    // 推进足够长的时间让递归 timeout 完成
    vi.advanceTimersByTime(60_000);
    expect(screen.getByText('ncgo new user-api')).toBeInTheDocument();
    expect(screen.getByText('manifest.yaml')).toBeInTheDocument();
    expect(screen.getByText('AGENTS.md')).toBeInTheDocument();
  });

  it('prefixes command lines with $ prompt', () => {
    window.matchMedia = vi.fn().mockImplementation((q: string) => ({
      matches: q.includes('reduce'),
      media: q,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
    const { container } = render(<Terminal lines={lines} />);
    expect(container.querySelector('.term-prompt')?.textContent).toBe('$ ');
  });

  it('applies kind classes to lines', () => {
    window.matchMedia = vi.fn().mockImplementation((q: string) => ({
      matches: q.includes('reduce'),
      media: q,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
    const { container } = render(<Terminal lines={lines} />);
    expect(container.querySelectorAll('.term-command').length).toBeGreaterThan(0);
    expect(container.querySelectorAll('.term-highlight').length).toBeGreaterThan(0);
  });
});
