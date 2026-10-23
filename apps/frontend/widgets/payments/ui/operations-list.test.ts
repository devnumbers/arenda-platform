import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { OperationsDateList, OperationRow } from './operations-list';
import type { PaymentOperation } from '@/entities/payment';
import type { PaymentHistoryGroup } from '@/features/payments';

/** Канон дат-группы (Figma 1036-40829, 1858-102657): заголовок с верхом 24
 * и низом 0, до первого ряда — гэп 8 контейнера группы; ряды в столбик без
 * зазоров. */

type InspectableProps = {
  children?: ReactNode;
  className?: string;
};

const operation: PaymentOperation = {
  id: 'op-1',
  propertyId: 'p-1',
  paymentId: null,
  origin: 'manual',
  date: '2026-10-22',
  status: 'paid',
  type: 'expense',
  title: 'Интернет',
  amountKopecks: 100_000,
  categoryLabel: 'Интернет',
};

const groups: ReadonlyArray<PaymentHistoryGroup> = [
  { date: '2026-10-22', label: 'Сегодня, 22 октября', operations: [operation, operation] },
];

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

describe('OperationsDateList (гэп заголовок → ряды дат-группы)', () => {
  it('ряды группы сидят в контейнере с верхом 8 (pt-2), столбиком без зазоров', () => {
    const list = OperationsDateList({
      groups,
      onSelectOperation: () => {},
    });
    const rowsWrappers = findElements(
      list.props.children,
      (element) =>
        element.type === 'div' && String(element.props.className).includes('pt-2'),
    );
    expect(rowsWrappers).toHaveLength(1);
    const wrapper = rowsWrappers[0] as ReactElement<InspectableProps>;
    expect(String(wrapper.props.className)).toContain('flex-col');
    const rows = findElements(wrapper.props.children, (element) => element.type === OperationRow);
    expect(rows).toHaveLength(2);
  });
});
