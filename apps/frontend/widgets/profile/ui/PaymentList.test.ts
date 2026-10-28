import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';

// Хуки данных и роутера — инфраструктура вне интереса теста; мок позволяет
// вызвать компонент как функцию (канон тестов фронта — node без DOM,
// идиома payment-row-button.test.ts).
vi.mock('@/features/billing', () => ({ useSubscriptionPayments: vi.fn() }));
vi.mock('next/navigation', () => ({ useRouter: vi.fn(() => ({ push: () => {} })) }));

import { useSubscriptionPayments } from '@/features/billing';
import { PaymentList } from './PaymentList';

const paymentsMock = vi.mocked(useSubscriptionPayments);

const emptyQuery = {
  data: [],
  isPending: false,
  isError: false,
  refetch: async () => {},
} as unknown as ReturnType<typeof useSubscriptionPayments>;

type InspectableProps = {
  children?: ReactNode;
  className?: string;
  src?: string;
};

function collect(
  node: ReactNode,
  elements: ReactElement<InspectableProps>[],
  strings: string[],
): void {
  if (typeof node === 'string') {
    strings.push(node);
    return;
  }
  if (Array.isArray(node)) {
    for (const child of node) {
      collect(child, elements, strings);
    }
    return;
  }
  if (node === null || typeof node !== 'object' || !('props' in (node as ReactElement))) {
    return;
  }
  const element = node as ReactElement<InspectableProps>;
  elements.push(element);
  collect(element.props.children, elements, strings);
}

/** Пустое состояние «Нет операций» (#1618, карта #1613): серая строка без
 * иллюстрации прижата к верху по канону посадки §3 — центрирование
 * страницы запрещено решением владельца 28.10.2026. */
describe('PaymentList — пустое состояние «Нет операций» (#1618, канон §3)', () => {
  it('прижата к верху: pt-10 по центру колонки, центрирующей обёртки нет', () => {
    paymentsMock.mockReturnValue(emptyQuery);
    const elements: ReactElement<InspectableProps>[] = [];
    const strings: string[] = [];
    collect(PaymentList(), elements, strings);

    // Обёртки min-h-[60vh] items-center justify-center больше нет.
    const classes = elements.map((element) => String(element.props.className ?? ''));
    expect(classes.some((className) => className.includes('min-h-[60vh]'))).toBe(false);
    expect(classes.some((className) => className.includes('items-center justify-center'))).toBe(
      false,
    );

    // Строка на каноническом верхе: pt-10 (64 от шапки суммарно с pt-6
    // PageContent), по центру колонки, серым.
    const line = elements.find((element) => element.props.children === 'Нет операций');
    expect(line).toBeDefined();
    const className = String(line?.props.className ?? '');
    expect(className).toContain('pt-10');
    expect(className).toContain('text-center');
    expect(className).toContain('text-content-secondary');
  });

  it('без иллюстрации — макет 1886-109856 проще канона EmptyState с PNG', () => {
    paymentsMock.mockReturnValue(emptyQuery);
    const elements: ReactElement<InspectableProps>[] = [];
    const strings: string[] = [];
    collect(PaymentList(), elements, strings);

    expect(elements.some((element) => typeof element.props.src === 'string')).toBe(false);
    expect(strings).toContain('Нет операций');
  });
});
