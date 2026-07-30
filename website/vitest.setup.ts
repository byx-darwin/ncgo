import '@testing-library/jest-dom/vitest';
import { vi } from 'vitest';

// jsdom 无 matchMedia：默认「不减少动效」，测试可覆盖
if (typeof window !== 'undefined' && !window.matchMedia) {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    })),
  });
}

// jsdom 无 navigator.clipboard
Object.defineProperty(navigator, 'clipboard', {
  writable: true,
  value: { writeText: vi.fn().mockResolvedValue(undefined) },
});

// jsdom 无 IntersectionObserver：默认不相交（Reveal 组件应降级为可见）
class MockIntersectionObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}
if (typeof window !== 'undefined' && !('IntersectionObserver' in window)) {
  Object.defineProperty(window, 'IntersectionObserver', {
    writable: true,
    value: MockIntersectionObserver,
  });
  Object.defineProperty(globalThis, 'IntersectionObserver', {
    writable: true,
    value: MockIntersectionObserver,
  });
}
