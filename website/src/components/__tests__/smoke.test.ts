import { describe, expect, it } from 'vitest';

describe('test infra', () => {
  it('runs in jsdom with mocks', () => {
    expect(typeof window).toBe('object');
    expect(window.matchMedia('(prefers-reduced-motion: reduce)').matches).toBe(false);
    expect(navigator.clipboard.writeText).toBeDefined();
  });
});
