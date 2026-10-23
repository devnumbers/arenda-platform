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

describe('SearchResultsSkeleton (канон отступов выдачи, §7/1726-90433)', () => {
  const skeleton = SearchResultsSkeleton({}) as ReactElement<InspectableProps>;

  it('секции в контейнере с гэпом 8, верх контейнера не добавляет — верх секции даёт заголовок-заглушка', () => {
    const className = String(skeleton.props.className);
    expect(className).toContain('gap-2');
    expect(className).not.toContain('gap-6');
    expect(className).not.toContain('pt-4');
  });

  it('заголовки-заглушки: первый прижат к хедеру (px-6 без верха — 24 даёт PageContent), второй несёт верх 24', () => {
    const sections = findElements(
      skeleton.props.children,
      (element) => element.type === 'section',
    );
    expect(sections).toHaveLength(2);
    const [first, second] = sections as [typeof sections[number], typeof sections[number]];
    for (const section of sections) {
      const stripWrapper = findElement(
        section.props.children,
        (element) =>
          typeof element.type === 'string' &&
          String(element.props.className).startsWith('px-6'),
      );
      expect(stripWrapper).not.toBeNull();
      const strip = findElement(
        stripWrapper?.props.children,
        (element) => String(element.props.className).includes('h-6 w-40'),
      );
      expect(strip).not.toBeNull();
    }
    expect(String(findElement(first.props.children, (e) => typeof e.type === 'string' && String(e.props.className).startsWith('px-6'))?.props.className)).toBe('px-6');
    expect(String(findElement(second.props.children, (e) => typeof e.type === 'string' && String(e.props.className).startsWith('px-6'))?.props.className)).toBe('px-6 pt-6');
  });

  it('чипы под заголовком с верхом 16 (pt-4), ряды — с верхом 8 (pt-2)', () => {
    const chipsRow = findElement(
      skeleton.props.children,
      (element) =>
        element.type === 'div' &&
        String(element.props.className).includes('gap-1.5'),
    );
    expect(String(chipsRow?.props.className)).toContain('pt-4');
    const rowsWrapper = findElement(
      skeleton.props.children,
      (element) =>
        element.type === 'div' && String(element.props.className).includes('pt-2'),
    );
    expect(rowsWrapper).not.toBeNull();
  });
});
