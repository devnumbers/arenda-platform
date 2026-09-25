import { describe, expect, it } from 'vitest';
import {
  FOLLOW_BOTTOM_PX,
  decideFeedScroll,
  type FeedEdges,
} from './feed-scroll';

const edges = (count: number, firstId: string | null, lastId: string | null): FeedEdges => ({
  count,
  firstId,
  lastId,
});

describe('decideFeedScroll — мессенджерская прокрутка живой ленты (тикет #718)', () => {
  it('первая загрузка встаёт на дно (видны самые свежие)', () => {
    const action = decideFeedScroll({ edges: null, height: 0, distanceToOldBottom: 0 }, edges(3, 'a1', 'a3'), 2000);
    expect(action).toStrictEqual({ kind: 'anchor' });
  });

  it('лента, выросшая из пустой, встаёт на дно', () => {
    const prev = { edges: edges(0, null, null), height: 800, distanceToOldBottom: 0 };
    const action = decideFeedScroll(prev, edges(3, 'a1', 'a3'), 2000);
    expect(action).toStrictEqual({ kind: 'anchor' });
  });

  it('пустая выдача не скроллит (пустые состояния — не сообщения)', () => {
    const prev = { edges: edges(3, 'a1', 'a3'), height: 2000, distanceToOldBottom: 0 };
    const action = decideFeedScroll(prev, edges(0, null, null), 800);
    expect(action).toStrictEqual({ kind: 'none' });
  });

  it('сужение выдачи (поиск/фильтры) — на дно свежих найденных', () => {
    const prev = { edges: edges(50, 'r50', 'r1'), height: 5000, distanceToOldBottom: 0 };
    const action = decideFeedScroll(prev, edges(4, 'm4', 'm1'), 900);
    expect(action).toStrictEqual({ kind: 'anchor' });
  });

  it('prepend старых при прокрутке вверх держит позицию компенсацией дельты', () => {
    const prev = { edges: edges(50, 'r50', 'r1'), height: 5000, distanceToOldBottom: 400 };
    const action = decideFeedScroll(prev, edges(100, 'r100', 'r1'), 6000);
    expect(action).toStrictEqual({ kind: 'compensate', delta: 1000 });
  });

  it('свежие снизу при читателе на дне — follow: дельта открывает новые строки', () => {
    const prev = { edges: edges(50, 'r50', 'r1'), height: 5000, distanceToOldBottom: FOLLOW_BOTTOM_PX - 1 };
    const action = decideFeedScroll(prev, edges(52, 'r50', 'f2'), 5120);
    expect(action).toStrictEqual({ kind: 'follow', delta: 120 });
  });

  it('свежие снизу при читателе выше дна — индикатор «есть новые», без рывков', () => {
    const prev = { edges: edges(50, 'r50', 'r1'), height: 5000, distanceToOldBottom: 2000 };
    const action = decideFeedScroll(prev, edges(52, 'r50', 'f2'), 5120);
    expect(action).toStrictEqual({ kind: 'indicate' });
  });

  it('выросли оба края (перечитанное окно после burst) — реанкер на дно', () => {
    const prev = { edges: edges(50, 'r50', 'r1'), height: 5000, distanceToOldBottom: 2000 };
    const action = decideFeedScroll(prev, edges(60, 's60', 's1'), 6100);
    expect(action).toStrictEqual({ kind: 'anchor' });
  });

  it('равная выдача с новыми границами — окно сдвинулось (рефетч после разрыва): на дно', () => {
    const prev = { edges: edges(50, 'r50', 'r1'), height: 5000, distanceToOldBottom: 2000 };
    const action = decideFeedScroll(prev, edges(50, 's50', 's1'), 5000);
    expect(action).toStrictEqual({ kind: 'anchor' });
  });

  it('равная выдача с теми же границами — ничего не делаем', () => {
    const prev = { edges: edges(50, 'r50', 'r1'), height: 5000, distanceToOldBottom: 500 };
    const action = decideFeedScroll(prev, edges(50, 'r50', 'r1'), 5000);
    expect(action).toStrictEqual({ kind: 'none' });
  });
});
