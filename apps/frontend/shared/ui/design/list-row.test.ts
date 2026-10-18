import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { ListRow } from './list-row';

/** Плоское дерево React-элементов строки без рендера: канон тестов
 * фронта — node без DOM (vitest.config.mts); ListRow без хуков можно
 * вызывать как функцию и инспектировать элементы (как в
 * shared/ui/design/button.test.ts). */

type InspectableProps = { children?: ReactNode; className?: string };

function flatten(node: ReactNode, elements: ReactElement<InspectableProps>[]): void {
  if (Array.isArray(node)) {
    for (const child of node) {
      flatten(child, elements);
    }
    return;
  }
  if (node === null || typeof node !== 'object') {
    return;
  }
  const element = node as ReactElement<InspectableProps>;
  elements.push(element);
  flatten(element.props.children, elements);
}

function rowElements(props: Record<string, unknown>): {
  row: ReactElement<InspectableProps>;
  elements: ReactElement<InspectableProps>[];
} {
  const row = ListRow({
    title: 'Заголовок',
    onSelect: () => {},
    ...props,
  });
  const elements: ReactElement<InspectableProps>[] = [];
  flatten(row, elements);
  return { row, elements };
}

describe('ListRow — отклик на наведение/нажатие (решение владельца 17.10)', () => {
  it('по умолчанию — приглушение текста, без фона (канон Figma 699:7254)', () => {
    const { row, elements } = rowElements({});

    expect(String(row.props.className)).not.toContain('hover:bg-surface-muted-hover');
    const title = elements.find((element) => element.props.children === 'Заголовок');
    expect(String(title?.props.className)).toContain('group-hover/row:text-content-secondary');
  });

  it('hoverEffect="background" — фон при наведении и нажатии, как у «Управления» объекта; текст не гаснет', () => {
    const { row, elements } = rowElements({ hoverEffect: 'background' });

    const rowClass = String(row.props.className);
    expect(rowClass).toContain('hover:bg-surface-muted-hover');
    expect(rowClass).toContain('active:bg-surface-muted-hover');
    expect(rowClass).toContain('rounded-button');

    const title = elements.find((element) => element.props.children === 'Заголовок');
    expect(String(title?.props.className)).not.toContain('group-hover/row:text-content-secondary');
    expect(String(title?.props.className)).toContain('text-content');
  });

  it('hoverEffect="background" гасит и приглушение значения (value)', () => {
    const { elements } = rowElements({ hoverEffect: 'background', value: '12 000 ₽' });

    const value = elements.find((element) => element.props.children === '12 000 ₽');
    expect(String(value?.props.className)).not.toContain('group-hover/row:text-content-secondary');
  });
});
