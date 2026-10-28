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

/** Идиома «component called as function» (CODING_STANDARDS): внутренний
 * компонент в дереве — элемент-функция; материализуем его в JSX-поддерево
 * для инспекции литеральных узлов (кнопки, хинты). */
function renderComponent(
  element: ReactElement<InspectableProps>,
): ReactElement<InspectableProps> {
  const type = element.type as unknown as (
    props: InspectableProps,
  ) => ReactElement<InspectableProps>;
  return type(element.props);
}

/** Кнопка пустой карточки — внутренний EmptySectionCardButton: находим
 * элемент-функцию в дереве и материализуем — он и есть литеральный <button>. */
function findEmptyCardButton(
  node: ReactNode,
): ReactElement<InspectableProps> {
  const card = findOne(
    node,
    (element) =>
      typeof element.type === 'function' &&
      element.type.name === 'EmptySectionCardButton',
  );
  const rendered = renderComponent(card);
  expect(rendered.type).toBe('button');
  return rendered;
}

/** Фоновая кнопка карточки с данными — внутренний CardBackgroundButton:
 * материализуем до литерального <button>. */
function findCardBackgroundButton(
  node: ReactNode,
): ReactElement<InspectableProps> {
  const card = findOne(
    node,
    (element) =>
      typeof element.type === 'function' &&
      element.type.name === 'CardBackgroundButton',
  );
  const rendered = renderComponent(card);
  expect(rendered.type).toBe('button');
  return rendered;
}

describe('PaymentsSection (серая карточка секции)', () => {
  it('паддинг: с данными низ 12 (решение владельца 28.10 «пустая 24 / с данными 12»): pb-3, pb-6 нет', () => {
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

  it('карточка с данными: фоновая кнопка под контентом ведёт на страницу секции', () => {
    const section = PaymentsSection({
      title: 'Платежи',
      onOpen: noop,
      openLabel: 'Открыть',
      emptyHint: 'Пусто',
      testId: 'section-payments',
      children: 'строка',
    });
    const className = String(section.props.className);
    expect(className).toContain('overflow-hidden');
    expect(className).toContain('relative');
    // Фон — нативная кнопка-оверлей под контентом: вне tab-порядка и скрыта
    // от AT, клавиатурный путь — кнопка заголовка.
    const background = findCardBackgroundButton(section.props.children);
    const backgroundProps = background.props as {
      readonly onClick?: () => void;
      readonly 'aria-hidden'?: boolean;
      readonly tabIndex?: number;
      readonly className?: string;
    };
    expect(backgroundProps.onClick).toBe(noop);
    expect(backgroundProps['aria-hidden']).toBe(true);
    expect(backgroundProps.tabIndex).toBe(-1);
    expect(backgroundProps.className).toContain('absolute');
    expect(backgroundProps.className).toContain('inset-0');
    expect(backgroundProps.className).toContain('cursor-pointer');
  });

  it('кликабельны только заголовок-кнопка и строки: обёртки контента transparent для кликов', () => {
    const section = PaymentsSection({
      title: 'Платежи',
      onOpen: noop,
      openLabel: 'Открыть',
      emptyHint: 'Пусто',
      testId: 'section-payments',
      children: 'строка',
    });
    // Обёртка контента не ловит клики — паддинги и зазоры достаются фону:
    const content = findOne(
      section.props.children,
      (element) =>
        String(element.props.className).includes('pointer-events-none'),
    );
    // Кнопка заголовка сама возвращает pointer-events (клик по её строке):
    const heading = findOne(
      content.props.children,
      (element) => element.type === 'button',
    );
    expect(String(heading.props.className)).toContain('pointer-events-auto');
    // Контейнер строк возвращает pointer-events (клики строк остаются
    // строкам), его собственный бокс совпадает со строками:
    const rows = findOne(
      content.props.children,
      (element) =>
        element.type !== 'button' &&
        String(element.props.className).includes('pointer-events-auto') &&
        String(element.props.className).includes('flex-col'),
    );
    expect(rows).toBeDefined();
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

describe('PaymentsSection — пустая секция кликается целиком', () => {
  const emptyBase = {
    title: 'Платежи',
    onOpen: noop,
    openLabel: 'Открыть все платежи',
    emptyHint: 'Напомним, когда нужно будет отметить оплату',
    testId: 'section-payments',
    children: null,
  };

  it('пустая секция — ровно одна кнопка: вся карточка несёт действие заголовка', () => {
    const section = PaymentsSection(emptyBase);
    const button = findEmptyCardButton(section.props.children);
    const actionProps = button.props as {
      readonly onClick?: () => void;
      readonly 'aria-label'?: string;
    };
    expect(actionProps.onClick).toBe(noop);
    expect(actionProps['aria-label']).toBe('Открыть все платежи');
  });

  it('карточка-кнопка несёт канон взаимодействия заголовка и геометрию карточки', () => {
    const section = PaymentsSection(emptyBase);
    const button = findEmptyCardButton(section.props.children);
    const className = String(button.props.className);
    expect(className).toContain('pressable');
    expect(className).toContain('cursor-pointer');
    expect(className).toContain('text-left');
    expect(className).toContain('hover:opacity-80');
    expect(className).toContain('focus-visible:ring-4');
    expect(className).toContain('rounded-card');
    expect(className).toContain('bg-surface-muted');
  });

  it('хинт пустоты остаётся внутри карточки-кнопки', () => {
    const section = PaymentsSection(emptyBase);
    const button = findEmptyCardButton(section.props.children);
    const hint = findOne(
      button.props.children,
      (element) =>
        String(element.props.className).includes('text-content-secondary'),
    );
    expect(hint.props.children).toBe('Напомним, когда нужно будет отметить оплату');
  });

  it('пустая карточка не выпускает детей за скругление — overflow-hidden на секции', () => {
    const section = PaymentsSection(emptyBase);
    expect(String(section.props.className)).toContain('overflow-hidden');
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

describe('PaymentsGroup — кликабельность карточки и отступы пустоты', () => {
  const noopOpen = { label: 'Открыть график платежей', onOpen: noop };

  it('пустая группа с open — вся карточка одна кнопка с действием заголовка (низ 24, py-6)', () => {
    const group = PaymentsGroup({
      title: 'История платежа',
      open: noopOpen,
      emptyHint: 'Платежей еще не было',
      children: null,
    });
    const button = findEmptyCardButton(group.props.children);
    const actionProps = button.props as {
      readonly onClick?: () => void;
      readonly 'aria-label'?: string;
      readonly className?: string;
    };
    expect(actionProps.onClick).toBe(noop);
    expect(actionProps['aria-label']).toBe('Открыть график платежей');
    const className = String(actionProps.className);
    expect(className).toContain('pressable');
    expect(className).toContain('cursor-pointer');
    expect(className).toContain('text-left');
    expect(className).toContain('rounded-card');
    expect(className).toContain('bg-surface-muted');
    expect(className).toContain('py-6');
    const hint = findOne(
      button.props.children,
      (element) =>
        String(element.props.className).includes('text-content-secondary'),
    );
    expect(hint.props.children).toBe('Платежей еще не было');
  });

  it('пустая группа без open — статичная карточка с низом 24, кнопок нет', () => {
    const group = PaymentsGroup({
      title: 'Ближайший платеж',
      emptyHint: 'Платежей больше не будет',
      children: null,
    });
    const buttons = findElements(
      group.props.children,
      (element) => element.type === 'button',
    );
    expect(buttons).toHaveLength(0);
    const className = String(group.props.className);
    expect(className).toContain('rounded-card');
    expect(className).toContain('bg-surface-muted');
    expect(className).not.toContain('pb-3');
    const hint = findOne(
      group.props.children,
      (element) =>
        String(element.props.className).includes('text-content-secondary'),
    );
    expect(String(hint.props.className)).toContain('pb-6');
  });

  it('группа с данными и open: фоновая кнопка ведёт на страницу секции — overflow-hidden, низ 12', () => {
    const group = PaymentsGroup({
      title: 'История платежа',
      open: noopOpen,
      children: 'строка',
    });
    const className = String(group.props.className);
    expect(className).toContain('pb-3');
    expect(className).toContain('overflow-hidden');
    expect(className).toContain('relative');
    const background = findCardBackgroundButton(group.props.children);
    const backgroundProps = background.props as {
      readonly onClick?: () => void;
      readonly 'aria-hidden'?: boolean;
      readonly tabIndex?: number;
      readonly className?: string;
    };
    expect(backgroundProps.onClick).toBe(noop);
    expect(backgroundProps['aria-hidden']).toBe(true);
    expect(backgroundProps.tabIndex).toBe(-1);
  });

  it('группа с данными без open — карточка не кликабельна', () => {
    const group = PaymentsGroup({
      title: 'История платежа',
      children: 'строка',
    });
    const buttons = findElements(
      group.props.children,
      (element) => element.type === 'button',
    );
    expect(buttons).toHaveLength(0);
    expect(String(group.props.className)).not.toContain('relative');
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
