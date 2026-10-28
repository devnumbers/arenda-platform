import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { RentalGroup } from './rental-group';

/** Канон кликабельной карточки аренды (решение владельца 28.10, зеркально
 * секциям «Платежей объекта»): вся карточка кликается на onOpen — фон
 * нативной кнопкой-оверлеем под контентом (CardBackgroundButton), клики
 * интерактивных детей остаются детям; contentInteractive=false — контент
 * прозрачен (TermRows/сводки без действий). Без onOpen карточка статична
 * («Управление»). Идиома теста — «component called as function»
 * (CODING_STANDARDS): внутренние компоненты материализуются в поддерево. */

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

describe('RentalGroup — кликабельная карточка (канон 28.10)', () => {
  const base = {
    title: 'Платеж',
    onOpen: noop,
    openLabel: 'Открыть платеж',
    children: 'строка',
  };

  it('карточка с onOpen: фон — кнопка-оверлей (aria-hidden, вне tab-порядка), overflow-hidden', () => {
    const group = RentalGroup(base);
    const className = String(group.props.className);
    expect(className).toContain('relative');
    expect(className).toContain('overflow-hidden');
    const overlay = findOne(
      group.props.children,
      (element) => typeof element.type === 'function',
    );
    const type = overlay.type as unknown as (
      props: InspectableProps,
    ) => ReactElement<InspectableProps>;
    expect(type.name).toBe('CardBackgroundButton');
    const button = type(overlay.props);
    expect(button.type).toBe('button');
    const buttonProps = button.props as {
      readonly onClick?: () => void;
      readonly 'aria-hidden'?: boolean;
      readonly tabIndex?: number;
    };
    expect(buttonProps.onClick).toBe(noop);
    expect(buttonProps['aria-hidden']).toBe(true);
    expect(buttonProps.tabIndex).toBe(-1);
  });

  it('кликабельны заголовок-кнопка и контент: обёртки прозрачны (паддинги → фон)', () => {
    const group = RentalGroup(base);
    const content = findOne(
      group.props.children,
      (element) =>
        String(element.props.className).includes('pointer-events-none'),
    );
    const heading = findOne(
      content.props.children,
      (element) => element.type === 'button',
    );
    expect(String(heading.props.className)).toContain('pointer-events-auto');
    // Дефолт contentInteractive=true: дети со своими действиями ловят клики
    // (контейнер контента — не кнопка заголовка).
    const interactive = findOne(
      content.props.children,
      (element) =>
        element.type !== 'button' &&
        String(element.props.className).includes('pointer-events-auto'),
    );
    expect(interactive.props.children).toBe('строка');
  });

  it('contentInteractive=false: контент прозрачен — вся карточка, кроме заголовка, ведёт на onOpen', () => {
    const group = RentalGroup({
      title: 'Условия аренды',
      onOpen: noop,
      children: 'сводка',
      contentInteractive: false,
    });
    const content = findOne(
      group.props.children,
      (element) =>
        String(element.props.className).includes('pointer-events-none'),
    );
    const wrappers = findElements(
      content.props.children,
      (element) =>
        element.type !== 'button' &&
        String(element.props.className).includes('pointer-events-auto'),
    );
    expect(wrappers).toHaveLength(0);
  });

  it('без onOpen — статичная карточка: оверлея нет, заголовок не кнопка', () => {
    const group = RentalGroup({ title: 'Управление', children: 'строки' });
    const className = String(group.props.className);
    expect(className).toContain('overflow-hidden');
    expect(className).not.toContain('relative');
    const functions = findElements(
      group.props.children,
      (element) => typeof element.type === 'function',
    );
    expect(functions).toHaveLength(0);
    const buttons = findElements(
      group.props.children,
      (element) => element.type === 'button',
    );
    expect(buttons).toHaveLength(0);
    const heading = findOne(
      group.props.children,
      (element) => element.type === 'h2',
    );
    expect(heading.props.children).toBe('Управление');
  });
});
