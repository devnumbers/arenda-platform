import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import {
  PaymentsGroup,
  PaymentsHeading,
  PaymentsRowsSkeleton,
  PaymentsSection,
  PaymentsSkeleton,
} from './payments-sections';
import { SkeletonListRow } from '@/shared/ui/design';

/** Канон отступов секций платежей (Figma 1096-37789, 1036-40829,
 * 1858-102657): серая карточка несёт паддинг только снизу — 12; заголовок —
 * бока 24, верх 24, низ 0; до списка — 8. Скелетоны повторяют геометрию
 * реальных блоков (§7 паритет): ряд 24/12. */

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

function findOne(
  node: ReactNode,
  predicate: (element: ReactElement<InspectableProps>) => boolean,
): ReactElement<InspectableProps> {
  const [first] = findElements(node, predicate);
  expect(first).toBeDefined();
  return first as ReactElement<InspectableProps>;
}

const noop = (): void => {};

describe('PaymentsSection (серая карточка секции)', () => {
  it('паддинг карточки только снизу — pb-3, pb-6 нет', () => {
    const section = PaymentsSection({
      title: 'Платежи',
      onOpen: noop,
      openLabel: 'Открыть',
      emptyHint: 'Пусто',
      testId: 'section-payments',
      children: 'строка',
    });
    const className = String(section.props.className);
    expect(className).toContain('pb-3');
    expect(className).not.toContain('pb-6');
  });

  it('заголовок → список — гэп 8 (mt-2)', () => {
    const section = PaymentsSection({
      title: 'Платежи',
      onOpen: noop,
      openLabel: 'Открыть',
      emptyHint: 'Пусто',
      testId: 'section-payments',
      children: 'строка',
    });
    const list = findOne(section.props.children, (element) =>
      String(element.props.className).includes('mt-2'),
    );
    expect(String(list.props.className)).toContain('flex-col');
  });
});

describe('PaymentsGroup (серая группа страницы платежа)', () => {
  it('паддинг карточки только снизу — pb-3, pb-6 нет', () => {
    const section = PaymentsGroup({ title: 'История', children: 'строка' });
    const className = String(section.props.className);
    expect(className).toContain('pb-3');
    expect(className).not.toContain('pb-6');
  });

  it('шапка: верх 24, до строк 8 — px-6 pb-2 pt-6', () => {
    const section = PaymentsGroup({ title: 'История', children: 'строка' });
    const header = findOne(section.props.children, (element) =>
      String(element.props.className).includes('pt-6'),
    );
    const className = String(header.props.className);
    expect(className).toContain('px-6');
    expect(className).toContain('pb-2');
    expect(className).not.toContain('pb-3');
  });
});

describe('PaymentsHeading (заголовок дат-группы)', () => {
  it('бока 24 и верх 24 — px-6 pt-6', () => {
    const heading = PaymentsHeading({ children: 'Сегодня' });
    const className = String(heading.props.className);
    expect(className).toContain('px-6');
    expect(className).toContain('pt-6');
  });

  it('inset=false — верх 24 остаётся, своей вставки по бокам нет', () => {
    const heading = PaymentsHeading({ children: 'Сегодня', inset: false });
    const className = String(heading.props.className);
    expect(className).toContain('pt-6');
    expect(className).not.toContain('px-6');
  });

  it('className потребителя гасит канонный верх — pt-0 побеждает внутренний pt-6', () => {
    const heading = PaymentsHeading({ children: 'Категории', className: 'pt-0' });
    const className = String(heading.props.className);
    expect(className).toContain('pt-0');
    expect(className).not.toContain('pt-6');
  });
});

describe('скелетоны (паритет §7 с рядом 24/12 и карточкой pb-3)', () => {
  it('PaymentsRowsSkeleton — строки высоты ряда 12/12 (py-3)', () => {
    const skeleton = PaymentsRowsSkeleton({ rows: 3 });
    const rows = findElements(skeleton.props.children, (element) => element.type === SkeletonListRow);
    expect(rows.length).toBe(3);
    for (const row of rows) {
      expect(String(row.props.className)).toContain('py-3');
    }
  });

  it('PaymentsSkeleton — карточка pt-6 pb-3, строки высоты реального ряда (h-17 = 68)', () => {
    const skeleton = PaymentsSkeleton({ withHeading: true });
    const className = String(skeleton.props.className);
    expect(className).toContain('pt-6');
    expect(className).toContain('pb-3');
    expect(className).not.toContain('py-6');
    // Полосы рядов уникальны высотой h-17 (68 = 44 + 12/12); заголовочная —
    // h-6, поэтому ищем по всему дереву скелетона.
    const bars = findElements(skeleton.props.children, (element) =>
      String(element.props.className).includes('h-17'),
    );
    expect(bars).toHaveLength(2);
  });
});
