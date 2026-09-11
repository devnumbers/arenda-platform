import { describe, expect, it } from 'vitest';
import {
  createPrefetchThrottle,
  matchHubPrefix,
  readPrefetchStrategy,
  type PrefetchStrategy,
} from './hub-prefetch-model';

describe('readPrefetchStrategy', () => {
  it('по умолчанию — off: прототип спит до решения владельца (#610)', () => {
    expect(readPrefetchStrategy(null)).toBe<PrefetchStrategy>('off');
    expect(readPrefetchStrategy('')).toBe<PrefetchStrategy>('off');
  });

  it('читает значение переключателя исследования', () => {
    expect(readPrefetchStrategy('off')).toBe<PrefetchStrategy>('off');
    expect(readPrefetchStrategy('intent')).toBe<PrefetchStrategy>('intent');
    expect(readPrefetchStrategy('warm')).toBe<PrefetchStrategy>('warm');
    expect(readPrefetchStrategy('all')).toBe<PrefetchStrategy>('all');
  });

  it('незнакомое значение — как отсутствие переключателя', () => {
    expect(readPrefetchStrategy('sometimes')).toBe<PrefetchStrategy>('off');
  });
});

describe('matchHubPrefix', () => {
  const entries = [
    { prefix: '/payments', tag: 'payments' },
    { prefix: '/tasks', tag: 'tasks' },
  ] as const;

  it('точное совпадение пути хаба', () => {
    expect(matchHubPrefix(entries, '/payments')?.tag).toBe('payments');
  });

  it('query и hash не мешают совпадению', () => {
    expect(matchHubPrefix(entries, '/tasks?filter=x#top')?.tag).toBe('tasks');
  });

  it('граница пути: /tasksFoo не хаб задач', () => {
    expect(matchHubPrefix(entries, '/tasksFoo')).toBeUndefined();
  });

  it('внутренние страницы хаба наследуют его префикс', () => {
    expect(matchHubPrefix(entries, '/payments/search')?.tag).toBe('payments');
  });

  it('неизвестный путь — undefined', () => {
    expect(matchHubPrefix(entries, '/profile/notifications')).toBeUndefined();
  });
});

describe('createPrefetchThrottle', () => {
  it('первый вызов проходит, повтор в окне — нет', () => {
    const throttle = createPrefetchThrottle(5000);
    expect(throttle.shouldRun('/tasks', 1000)).toBe(true);
    expect(throttle.shouldRun('/tasks', 3000)).toBe(false);
  });

  it('после окна — снова проходит', () => {
    const throttle = createPrefetchThrottle(5000);
    throttle.shouldRun('/tasks', 1000);
    expect(throttle.shouldRun('/tasks', 6000)).toBe(true);
  });

  it('ключи не мешают друг другу', () => {
    const throttle = createPrefetchThrottle(5000);
    throttle.shouldRun('/tasks', 1000);
    expect(throttle.shouldRun('/contacts', 1001)).toBe(true);
  });
});
