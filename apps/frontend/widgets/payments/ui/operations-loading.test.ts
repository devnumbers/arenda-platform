import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { OperationsCategoriesLoading } from './operations-loading';

/** Канон верха выборщика категорий: контейнер контента без своего верхнего
 * паддинга — чип периода открывает контент в 24 от хедера (pt-6
 * PageContent), как на лентах (макет 1858-104152); лишние 16 сняты
 * (замечание владельца 23.10). Loading-близнец зеркалит живой экран —
 * паритет §7 проверяется на нём (живой экран с хуками функцией не
 * вызывается, идиома канон-тестов фронта). */

type InspectableProps = {
  children?: ReactNode;
  className?: string;
};

describe('OperationsCategoriesLoading (верх контейнера выборщика)', () => {
  it('контейнер с чипом периода без своего верхнего паддинга — gap-6, не pt-4', () => {
    const screen = OperationsCategoriesLoading() as ReactElement<InspectableProps>;
    const container = findElement(
      screen.props.children,
      (element) =>
        element.type === 'div' &&
        String(element.props.className).includes('gap-6'),
    );
    expect(container).not.toBeNull();
    const className = String(container?.props.className);
    expect(className).toContain('gap-6');
    expect(className).not.toContain('pt-4');
  });
});

function findElement(
  node: ReactNode,
  predicate: (element: ReactElement<InspectableProps>) => boolean,
): ReactElement<InspectableProps> | null {
  if (Array.isArray(node)) {
    for (const child of node) {
      const found = findElement(child, predicate);
      if (found !== null) {
        return found;
      }
    }
    return null;
  }
  if (
    node !== null &&
    typeof node === 'object' &&
    'props' in (node as ReactElement)
  ) {
    const element = node as ReactElement<InspectableProps>;
    if (predicate(element)) {
      return element;
    }
    return findElement(element.props.children, predicate);
  }
  return null;
}
