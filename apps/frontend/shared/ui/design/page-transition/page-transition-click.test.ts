import { describe, expect, it } from 'vitest';
import {
  type TransitionAnchorFacts,
  type TransitionClickFacts,
  resolveTransitionHref,
} from './page-transition-click';

/** Клик-фильтр шторки (порт alterix-new): какой клик по ссылке ведёт
 * переход под листом. Правила эталона — обычный левый клик без
 * модификаторов по same-origin ссылке; якоря-исключения: skip-метка,
 * download, target≠_self, #ссылки, тот же URL, хэш текущей страницы. */

const IDLE_CLICK: TransitionClickFacts = {
  defaultPrevented: false,
  button: 0,
  modified: false,
};

const PLAIN_ANCHOR: TransitionAnchorFacts = {
  rawHref: '/operations',
  download: false,
  selfTarget: true,
  withinSkip: false,
};

const HERE = 'https://app.example.com/payments?tab=due#list';

describe('Клик-фильтр шторки переходов', () => {
  it('same-origin ссылка даёт путь с query и хэшем', () => {
    expect(
      resolveTransitionHref(IDLE_CLICK, PLAIN_ANCHOR, HERE),
    ).toBe('/operations');
  });

  it('ссылка с query и хэшем сохраняет их в цели', () => {
    expect(
      resolveTransitionHref(
        IDLE_CLICK,
        { ...PLAIN_ANCHOR, rawHref: '/operations?period=month#feed' },
        HERE,
      ),
    ).toBe('/operations?period=month#feed');
  });

  it('предотвращённый клик, правая кнопка и модификаторы мимо шторки', () => {
    const prevented: TransitionClickFacts = { ...IDLE_CLICK, defaultPrevented: true };
    const middle: TransitionClickFacts = { ...IDLE_CLICK, button: 1 };
    const modified: TransitionClickFacts = { ...IDLE_CLICK, modified: true };

    for (const click of [prevented, middle, modified]) {
      expect(resolveTransitionHref(click, PLAIN_ANCHOR, HERE)).toBeNull();
    }
  });

  it('мимо шторки: нет якоря, skip-метка, download, target≠_self', () => {
    expect(resolveTransitionHref(IDLE_CLICK, null, HERE)).toBeNull();
    expect(
      resolveTransitionHref(IDLE_CLICK, { ...PLAIN_ANCHOR, withinSkip: true }, HERE),
    ).toBeNull();
    expect(
      resolveTransitionHref(IDLE_CLICK, { ...PLAIN_ANCHOR, download: true }, HERE),
    ).toBeNull();
    expect(
      resolveTransitionHref(IDLE_CLICK, { ...PLAIN_ANCHOR, selfTarget: false }, HERE),
    ).toBeNull();
  });

  it('мимо шторки: пустой href, #ссылка, не-http схема, чужой origin', () => {
    expect(
      resolveTransitionHref(IDLE_CLICK, { ...PLAIN_ANCHOR, rawHref: '' }, HERE),
    ).toBeNull();
    expect(
      resolveTransitionHref(IDLE_CLICK, { ...PLAIN_ANCHOR, rawHref: '#' }, HERE),
    ).toBeNull();
    expect(
      resolveTransitionHref(IDLE_CLICK, { ...PLAIN_ANCHOR, rawHref: 'mailto:x@y.z' }, HERE),
    ).toBeNull();
    expect(
      resolveTransitionHref(
        IDLE_CLICK,
        { ...PLAIN_ANCHOR, rawHref: 'https://other.example.com/operations' },
        HERE,
      ),
    ).toBeNull();
    expect(
      resolveTransitionHref(IDLE_CLICK, { ...PLAIN_ANCHOR, rawHref: '//other.example.com/x' }, HERE),
    ).toBeNull();
  });

  it('мимо шторки: тот же URL и хэш текущей страницы', () => {
    expect(
      resolveTransitionHref(IDLE_CLICK, { ...PLAIN_ANCHOR, rawHref: HERE }, HERE),
    ).toBeNull();
    expect(
      resolveTransitionHref(
        IDLE_CLICK,
        { ...PLAIN_ANCHOR, rawHref: 'https://app.example.com/payments?tab=due#list' },
        HERE,
      ),
    ).toBeNull();
    // семантика эталона: та же path+search, но без хэша — хэш уходит, это
    // настоящая смена URL, переход под шторкой
    expect(
      resolveTransitionHref(
        IDLE_CLICK,
        { ...PLAIN_ANCHOR, rawHref: 'https://app.example.com/payments?tab=due' },
        HERE,
      ),
    ).toBe('/payments?tab=due');
  });
});
