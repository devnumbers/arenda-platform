import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { PHOTO_RING_LENGTH, PhotoProgressRing } from './photo-progress-ring';

/** Канон тестов фронта — плоское дерево React-элементов без рендера
 * (прецедент button.test.ts): компонент без хуков вызывается как функция.
 * Геометрия известна из константы модуля (2π·46 ≈ 289.03) — ожидания
 * считаются в тесте независимо от реализации кольца. */

type InspectableProps = Record<string, unknown> & { children?: ReactNode };

function circles(node: ReactNode, found: ReactElement<InspectableProps>[] = []): ReactElement<InspectableProps>[] {
  if (Array.isArray(node)) {
    for (const child of node) {
      circles(child, found);
    }
    return found;
  }
  if (node !== null && typeof node === 'object' && (node as ReactElement).props !== undefined) {
    const element = node as ReactElement<InspectableProps>;
    if (element.type === 'circle') {
      found.push(element);
    }
    circles(element.props.children, found);
  }
  return found;
}

/** Дуга прогресса — второй circle в svg (первый — дорожка). */
function ringBar(svg: ReactNode): ReactElement<InspectableProps> {
  const bar = circles(svg)[1];
  if (bar === undefined) {
    throw new Error('PhotoProgressRing не нарисовал дугу прогресса');
  }
  return bar;
}

describe('PhotoProgressRing — дуга прогресса на круге фото (план 036)', () => {
  it('0% — дуга полностью скрыта (offset = длина окружности)', () => {
    const bar = ringBar(PhotoProgressRing({ percent: 0 }));
    expect(bar.props['strokeDasharray']).toBe(PHOTO_RING_LENGTH);
    expect(bar.props['strokeDashoffset']).toBeCloseTo(PHOTO_RING_LENGTH, 5);
  });

  it('100% — дуга замкнута (offset = 0)', () => {
    expect(ringBar(PhotoProgressRing({ percent: 100 })).props['strokeDashoffset']).toBe(0);
  });

  it('50% — половина окружности', () => {
    expect(ringBar(PhotoProgressRing({ percent: 50 })).props['strokeDashoffset']).toBeCloseTo(
      PHOTO_RING_LENGTH / 2,
      5,
    );
  });

  it('проценты вне 0–100 зажимаются: 150 → полный круг, −5 → пустой', () => {
    expect(ringBar(PhotoProgressRing({ percent: 150 })).props['strokeDashoffset']).toBe(0);
    expect(ringBar(PhotoProgressRing({ percent: -5 })).props['strokeDashoffset']).toBeCloseTo(
      PHOTO_RING_LENGTH,
      5,
    );
  });
});
