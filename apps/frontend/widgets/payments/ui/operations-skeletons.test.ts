import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import { SearchResultsSkeleton } from './operations-skeletons';

/** Скелетон поисковой выдачи без рендера: канон тестов фронта — node без
 * DOM, компонент без хуков вызывается как функция и инспектируется
 * (идиома button.test.ts / operations-filter-chips.test.ts). */

type InspectableProps = {
  children?: ReactNode;
  className?: string;
};

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

describe('SearchResultsSkeleton (перенос чипов как живая выдача, #1076)', () => {
  it('ряд чипов-заглушек переносится (flex-wrap), не скроллится', () => {
    const skeleton = SearchResultsSkeleton({}) as ReactElement<InspectableProps>;
    const chipsRow = findElement(
      skeleton.props.children,
      (element) =>
        element.type === 'div' &&
        String(element.props.className).includes('gap-1.5'),
    );
    expect(chipsRow).not.toBeNull();
    const className = String(chipsRow?.props.className);
    expect(className).toContain('flex-wrap');
    expect(className).not.toContain('overflow-x-auto');
  });
});
