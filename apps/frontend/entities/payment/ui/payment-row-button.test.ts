import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';

// Хук клавиатурной активации — инфраструктура вне интереса теста; мок
// позволяет вызвать компонент как функцию (канон тестов фронта — node без
// DOM, идиома operations-skeletons.test.ts).
vi.mock('@/shared/lib/hooks/useKeyboardActivation', () => ({
  useKeyboardActivation: () => ({}),
}));

import { PaymentRowButton } from './payment-row-button';

type InspectableProps = {
  children?: ReactNode;
  className?: string;
};

function findElements(
  node: ReactNode,
  predicate: (element: ReactElement<InspectableProps>) => boolean,
): Array<ReactElement<InspectableProps>> {
  if (Array.isArray(node)) {
    return node.flatMap((child) => findElements(child, predicate));
  }
  if (node !== null && typeof node === 'object' && 'props' in (node as ReactElement)) {
    const element = node as ReactElement<InspectableProps>;
    const nested = findElements(element.props.children, predicate);
    return predicate(element) ? [element, ...nested] : nested;
  }
  return [];
}

/** Канон отступов ряда платежа (Figma 1096-37789, 1036-40829,
 * 1858-102657): вертикаль 12/12, горизонталь до контента 24 — паддинг 12
 * корня ряда + 12 контентной середины. */
describe('PaymentRowButton (канон отступов ряда)', () => {
  it('вертикаль 12/12 — py-3 корня, py-2 нет', () => {
    const row = PaymentRowButton({ title: 'Аренда стола', amountKopecks: 250_000 });
    const className = String((row as ReactElement<InspectableProps>).props.className);
    expect(className).toContain('py-3');
    expect(className).not.toContain('py-2');
  });

  it('горизонталь 24 — px-3 корня и px-3 контентной середины', () => {
    const row = PaymentRowButton({ title: 'Аренда стола', amountKopecks: 250_000 });
    expect(String((row as ReactElement<InspectableProps>).props.className)).toContain('px-3');
    const middles = findElements(
      (row as ReactElement<InspectableProps>).props.children,
      (element) =>
        element.type === 'span' &&
        String(element.props.className).includes('px-3') &&
        String(element.props.className).includes('flex-1'),
    );
    expect(middles).toHaveLength(1);
  });
});
