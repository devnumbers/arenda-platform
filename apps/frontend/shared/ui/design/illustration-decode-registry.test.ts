import { afterEach, describe, expect, it, vi } from 'vitest';
import type { SyntheticEvent } from 'react';
import { onIllustrationLoad, rememberIllustrationRef } from './illustration-decode-registry';

/** Канон тестов фронта — чистая логика без DOM (vitest.config.mts):
 * `Image` браузера стабится счётчиком инстансов (прецедент vi.stubGlobal —
 * navigation.test.ts), поведенческие проверки — без чтения приватного
 * состояния реестра. Проводка реестра в слоты (ref + onLoad → currentSrc)
 * и «рисуется одним кадром» на возврате — экранная приёмка
 * /ui-walkthrough. */

type FakeImageInstance = {
  src: string;
  decode: () => Promise<void>;
};

function stubImage(): {
  instances: FakeImageInstance[];
} {
  const instances: FakeImageInstance[] = [];
  class FakeImage {
    src = '';
    decode = (): Promise<void> => {
      instances.push(this);
      return Promise.resolve();
    };
  }
  vi.stubGlobal('Image', FakeImage);
  return { instances };
}

function makeLoadEvent(currentSrc: string): SyntheticEvent<HTMLImageElement> {
  return { currentTarget: { currentSrc } } as unknown as SyntheticEvent<HTMLImageElement>;
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('onIllustrationLoad — decode-реестр статических иллюстраций', () => {
  it('первая пометка создаёт offscreen-копию с тем же src и вызывает decode', () => {
    const { instances } = stubImage();
    onIllustrationLoad(
      makeLoadEvent('/_next/image?url=%2Fimages%2Fproperties%2Fempty.webp&w=256&q=90'),
    );
    expect(instances).toHaveLength(1);
    expect(instances[0]?.src).toBe(
      '/_next/image?url=%2Fimages%2Fproperties%2Fempty.webp&w=256&q=90',
    );
  });

  it('повторная пометка того же URL — новой offscreen-копии нет (идемпотентность)', () => {
    const { instances } = stubImage();
    const src = '/_next/image?url=%2Fimages%2Fpayments%2Fempty-payments.webp&w=128&q=90';
    onIllustrationLoad(makeLoadEvent(src));
    onIllustrationLoad(makeLoadEvent(src));
    expect(instances).toHaveLength(1);
  });

  it('другой URL (другой вариант srcset) — своя offscreen-копия', () => {
    const { instances } = stubImage();
    onIllustrationLoad(makeLoadEvent('/_next/image?url=%2Fimages%2Fa.webp&w=128&q=90'));
    onIllustrationLoad(makeLoadEvent('/_next/image?url=%2Fimages%2Fa.webp&w=256&q=90'));
    expect(instances).toHaveLength(2);
  });

  it('unoptimized-иконка (прямой URL) ключуется как есть', () => {
    const { instances } = stubImage();
    onIllustrationLoad(makeLoadEvent('/images/notifications/category-tasks.webp'));
    onIllustrationLoad(makeLoadEvent('/images/notifications/category-tasks.webp'));
    expect(instances).toHaveLength(1);
    expect(instances[0]?.src).toBe('/images/notifications/category-tasks.webp');
  });

  it('пустой currentSrc — пометка игнорируется, offscreen-копии нет', () => {
    const { instances } = stubImage();
    onIllustrationLoad(makeLoadEvent(''));
    expect(instances).toHaveLength(0);
  });

  it('провал decode снимает отметку — следующий показ попробует снова', async () => {
    let rejectDecode: ((reason: unknown) => void) | undefined;
    const instances: FakeImageInstance[] = [];
    class FakeImage {
      src = '';
      decode = (): Promise<void> =>
        new Promise<void>((_, reject) => {
          rejectDecode = reject;
          instances.push(this);
        });
    }
    vi.stubGlobal('Image', FakeImage);

    const src = '/_next/image?url=%2Fimages%2Fbroken.webp&w=256&q=90';
    onIllustrationLoad(makeLoadEvent(src));
    expect(instances).toHaveLength(1);
    rejectDecode?.(new Error('decode failed'));
    // catch-хендлер реестра (снятие отметки) — микротаска после reject.
    await new Promise<void>((resolve) => {
      setTimeout(resolve, 0);
    });
    onIllustrationLoad(makeLoadEvent(src));
    expect(instances).toHaveLength(2);
  });
});

describe('rememberIllustrationRef — готовый на монтировании кадр (SSR-гидратация, кеш-хит)', () => {
  // Ключи уникальные на файл: реестр — модульное состояние, общее для
  // всех тестов (канон photo-session-cache.test.ts).
  it('complete-элемент с currentSrc помечается (onLoad для него не придёт)', () => {
    const { instances } = stubImage();
    rememberIllustrationRef({
      complete: true,
      currentSrc: '/_next/image?url=%2Fimages%2Fref-ready.webp&w=256&q=90',
    } as HTMLImageElement);
    expect(instances).toHaveLength(1);
  });

  it('недогруженный элемент (complete=false) ждёт onLoad — пометки нет', () => {
    const { instances } = stubImage();
    rememberIllustrationRef({
      complete: false,
      currentSrc: '/_next/image?url=%2Fimages%2Fref-loading.webp&w=256&q=90',
    } as HTMLImageElement);
    expect(instances).toHaveLength(0);
  });

  it('null (unmount) — игнорируется', () => {
    const { instances } = stubImage();
    expect(() => {
      rememberIllustrationRef(null);
    }).not.toThrow();
    expect(instances).toHaveLength(0);
  });
});
