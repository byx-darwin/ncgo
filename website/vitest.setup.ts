import '@testing-library/jest-dom/vitest';
import { afterAll, vi } from 'vitest';

// Patch vi.useFakeTimers to default to shouldAdvanceTime: true.
// This prevents deadlocks when userEvent.setup() is used with fake timers,
// as userEvent's internal async operations need timers to advance.
// Override per-test if needed: vi.useFakeTimers({ shouldAdvanceTime: false })
const _origUseFakeTimers = vi.useFakeTimers.bind(vi);
vi.useFakeTimers = (opts?: any) => {
  return _origUseFakeTimers({ shouldAdvanceTime: true, ...opts });
};

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
// 1. Define our vi.fn()-backed mock as non-configurable.
const _origDefineProperty = Object.defineProperty;
_origDefineProperty(navigator, 'clipboard', {
  writable: false,
  configurable: false,
  value: { writeText: vi.fn().mockResolvedValue(undefined) },
});
// 2. Patch Object.defineProperty so userEvent.setup() cannot replace it.
//    userEvent would swap writeText for a non-spy class method, breaking
//    `toHaveBeenCalledWith` assertions in CopyCommand / CommandTabs tests.
//    Scope: restored in afterAll() so this interception does not leak past this setup file.
Object.defineProperty = function <T>(obj: T, prop: PropertyKey, desc: PropertyDescriptor & ThisType<T>): T {
  if (obj === navigator && prop === 'clipboard') {
    console.debug('[vitest.setup] blocked navigator.clipboard redefinition via Object.defineProperty');
    return obj;
  }
  return _origDefineProperty.call(Object, obj, prop, desc) as T;
} as typeof Object.defineProperty;

afterAll(() => {
  Object.defineProperty = _origDefineProperty;
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
