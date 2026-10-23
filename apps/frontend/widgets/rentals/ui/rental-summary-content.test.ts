import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { RentalSummarySkeleton } from './rental-skeletons';
import { RentalSummaryContent } from './rental-summary-content';
import { makeRental, type RentalSummary } from '@/entities/rental';
import { PaymentRowButton } from '@/entities/payment';
import { SkeletonListRow } from '@/shared/ui/design';

/** Канон секции «Данные аренды» итогов аренды (Figma 1433:61927 — десктоп,
 * 1433:61074 — мобайль, геометрия одинакова): зазор после «Возвращения
 * залога» 24, заголовок — канон Heading 24/24/0 (свои верх и бока, низ 0),
 * до строк 8, белые строки Row Button на всю ширину (иконка в 24 от края —
 * 12+12 ряда, потребительские горизонтали не навешиваются). Скелетон
 * повторяет геометрию (§7 паритет): ряд базовый px-6 (круг в 24) + py-3 —
 * 68. */

type InspectableProps = {
  children?: ReactNode;
  className?: string;
  variant?: unknown;
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

const rental = makeRental({
  status: 'completed',
  startDate: '2026-05-10',
  plannedEndDate: null,
  completedDate: '2026-10-23',
  depositKopecks: 5_600_000,
  depositReturnKopecks: 5_600_000,
  depositReturnComment: 'Залог вернул',
  tenant: { contactId: 'contact-1', firstName: 'Александр', lastName: '', phone: '' },
  today: '2026-10-23',
});

const summary: RentalSummary = {
  from: '2026-05-10',
  until: '2026-10-23',
  incomeKopecks: 50_000_000,
  expenseKopecks: 5_342_068,
  profitKopecks: 44_657_932,
};

function renderContent(): ReactElement {
  return RentalSummaryContent({
    rental,
    summary,
    endDate: '2026-10-23',
    propertyName: 'Моя квартира',
    depositReturn: { kopecks: 5_600_000, comment: 'Залог вернул' },
  });
}

const isDataSection = (element: ReactElement<InspectableProps>): boolean =>
  String(element.props.className) === 'pt-6';

describe('Секция «Данные аренды» (канон Figma 1433:61927/1433:61074)', () => {
  it('зазор после «Возвращения залога» 24 — обёртка pt-6, pt-16 нет', () => {
    const wrapper = findOne(renderContent(), isDataSection);
    const className = String(wrapper.props.className);
    expect(className).toContain('pt-6');
    expect(className).not.toContain('pt-16');
  });

  it('заголовок — канон Heading 24/24/0: свои px-6 pt-6, низа нет', () => {
    const heading = findOne(
      renderContent(),
      (element) => element.type === 'h2' && element.props.children === 'Данные аренды',
    );
    const className = String(heading.props.className);
    expect(className).toContain('px-6');
    expect(className).toContain('pt-6');
    expect(className).not.toContain('pb-');
  });

  it('строки на всю ширину — контейнер без px-3, до строк 8 (pt-2)', () => {
    const wrapper = findOne(renderContent(), isDataSection);
    const list = findOne(
      wrapper.props.children,
      (element) =>
        String(element.props.className).includes('flex-col') &&
        String(element.props.className).includes('pt-2'),
    );
    const className = String(list.props.className);
    expect(className).not.toContain('px-3');
  });

  it('ряды — потребитель без своих горизонталей: pointer-events-none, px-3 нет', () => {
    const rows = findElements(
      renderContent(),
      (element) => element.type === PaymentRowButton,
    );
    expect(rows.length).toBe(3);
    for (const row of rows) {
      const className = String(row.props.className);
      expect(className).toContain('pointer-events-none');
      expect(className).not.toContain('px-3');
      expect(row.props.variant).toBe('white');
    }
  });
});

describe('RentalSummarySkeleton — паритет §7 за секцией «Данные аренды»', () => {
  it('обёртка pt-6, не pt-16', () => {
    const wrapper = findOne(RentalSummarySkeleton(), isDataSection);
    const className = String(wrapper.props.className);
    expect(className).toContain('pt-6');
    expect(className).not.toContain('pt-16');
  });

  it('заголовок-заглушка — px-6 pt-6', () => {
    const skeleton = RentalSummarySkeleton();
    const heading = findOne(
      skeleton,
      (element) =>
        element.type === 'div' &&
        String(element.props.className).includes('pt-6') &&
        String(element.props.className).includes('px-6'),
    );
    const className = String(heading.props.className);
    expect(className).toContain('px-6');
    expect(className).toContain('pt-6');
  });

  it('строки-заглушки — круг в 24 (базовый px-6) и высота ряда py-3, своих горизонталей и py-2 нет', () => {
    const rows = findElements(
      RentalSummarySkeleton(),
      (element) => element.type === SkeletonListRow,
    );
    expect(rows.length).toBe(3);
    for (const row of rows) {
      const className = String(row.props.className);
      expect(className).toContain('py-3');
      expect(className).not.toContain('px-3');
      expect(className).not.toContain('py-2');
    }
  });
});
