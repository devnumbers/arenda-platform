import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  PUSH_PROMPT_SHOWN_KEY,
  hasPushPromptBeenShown,
  markPushPromptShown,
} from './push-prompt-flag';

function stubStorage(storage: Pick<Storage, 'getItem' | 'setItem'>): void {
  vi.stubGlobal('window', { localStorage: storage });
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('hasPushPromptBeenShown', () => {
  it('флаг стоит — промпт уже показывали, повторов нет', () => {
    stubStorage({ getItem: (key) => (key === PUSH_PROMPT_SHOWN_KEY ? '1' : null), setItem: () => {} });
    expect(hasPushPromptBeenShown()).toBe(true);
  });

  it('флага нет — первый визит этого браузера', () => {
    stubStorage({ getItem: () => null, setItem: () => {} });
    expect(hasPushPromptBeenShown()).toBe(false);
  });

  it('нет window (SSR-оценка модуля) — показанным не считаем, решение примет клиент', () => {
    vi.stubGlobal('window', undefined);
    expect(hasPushPromptBeenShown()).toBe(false);
  });

  it('localStorage недоступен (приватный режим) — считаем показанным: авто-промпт молчит, повторов нет', () => {
    stubStorage({
      getItem: () => {
        throw new DOMException('denied');
      },
      setItem: () => {},
    });
    expect(hasPushPromptBeenShown()).toBe(true);
  });
});

describe('markPushPromptShown', () => {
  it('ставит флаг независимо от исхода промпта — раз навсегда', () => {
    const setItem = vi.fn();
    stubStorage({ getItem: () => null, setItem });
    markPushPromptShown();
    expect(setItem).toHaveBeenCalledWith(PUSH_PROMPT_SHOWN_KEY, '1');
  });

  it('ошибка записи проглатывается — флаг не критичен для загрузки', () => {
    stubStorage({
      getItem: () => null,
      setItem: () => {
        throw new DOMException('quota');
      },
    });
    expect(() => markPushPromptShown()).not.toThrow();
  });

  it('нет window — no-op', () => {
    vi.stubGlobal('window', undefined);
    expect(() => markPushPromptShown()).not.toThrow();
  });
});
