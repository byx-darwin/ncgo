import { render } from '@testing-library/react';
import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import FileTree, { TreeRow } from '../FileTree';

const rows: TreeRow[] = [
  { text: 'user-api/', depth: 0 },
  { text: 'manifest.yaml', depth: 1 },
  { text: '.claude/', depth: 1, ai: true },
];

describe('FileTree', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('renders all rows when reduced motion is preferred', () => {
    window.matchMedia = vi.fn().mockImplementation((q: string) => ({
      matches: q.includes('reduce'),
      media: q,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
    const { container } = render(<FileTree rows={rows} />);
    expect(container.querySelectorAll('.filetree-row').length).toBe(3);
  });

  it('reveals rows over time when animating', () => {
    const { container } = render(<FileTree rows={rows} />);
    vi.advanceTimersByTime(5_000);
    expect(container.querySelectorAll('.filetree-row').length).toBe(3);
  });

  it('marks ai rows with badge', () => {
    window.matchMedia = vi.fn().mockImplementation((q: string) => ({
      matches: q.includes('reduce'),
      media: q,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
    const { container } = render(<FileTree rows={rows} />);
    expect(container.querySelectorAll('.filetree-ai').length).toBe(1);
    expect(container.querySelector('.filetree-badge')?.textContent).toBe('AI');
  });
});
