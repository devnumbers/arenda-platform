import type { ReactElement, ReactNode } from 'react';
import { describe, expect, it } from 'vitest';
import {DEFAULT_PROPERTY_SORT} from '../lib/property-sort';
import {PropertiesSortMenu, PropertiesSortRow} from './PropertiesPage';

/** Плоское дерево React-элементов ряда сортировки без рендера: канон
 * тестов фронта — node без DOM (vitest.config.mts); ряд без хуков можно
 * вызывать как функцию и инспектировать элементы (как в
 * shared/ui/design/button.test.ts). */
type InspectableProps = {
  children?: ReactNode;
  className?: string;
  onClick?: () => void;
};

function flatten(
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
      flatten(child, elements, strings);
    }
    return;
  }
  if (node === null || typeof node !== 'object') {
    // числа, false/undefined (условный JSX), прочие листья
    return;
  }
  const element = node as ReactElement<InspectableProps>;
  elements.push(element);
  flatten(element.props.children, elements, strings);
  // Chип сортировки (PropertiesSortMenu) досматриваем только как элемент:
  // внутрь он уводит в PickerMenu с хуками — вне React-рендера его не
  // вызвать, да и не нужно (факт присутствия/отсутствия чипа — и есть
  // поведение #1051).
}

function collect(node: ReactNode): {
  elements: ReactElement<InspectableProps>[];
  strings: string[];
} {
  const elements: ReactElement<InspectableProps>[] = [];
  const strings: string[] = [];
  flatten(node, elements, strings);
  return {elements, strings};
}

describe('PropertiesSortRow — «Архив» на пустом хабе (#1051, решение владельца 02.10.2026)', () => {
  it('на подтверждённой пустоте чип сортировки скрыт, «Архив» остаётся справа', () => {
    const row = PropertiesSortRow({
      sort: DEFAULT_PROPERTY_SORT,
      onChange: () => {},
      showChip: false,
      onOpenArchive: () => {},
    });
    const {elements, strings} = collect(row);

    // Чипа нет (сортировать нечего), «Архив» на месте.
    expect(elements.some((element) => element.type === PropertiesSortMenu)).toBe(false);
    expect(strings).toContain('Архив');

    // Ряд выровнен вправо — кнопка на обычном месте, а не у левого края.
    expect(row.props.className).toContain('justify-end');
    expect(row.props.className).not.toContain('justify-between');
    expect(elements.length).toBeGreaterThan(0);
  });

  it('на хабе с объектами чип сортировки на месте, ряд между собой', () => {
    const row = PropertiesSortRow({
      sort: DEFAULT_PROPERTY_SORT,
      onChange: () => {},
      showChip: true,
      onOpenArchive: () => {},
    });
    const {elements, strings} = collect(row);

    expect(elements.some((element) => element.type === PropertiesSortMenu)).toBe(true);
    expect(strings).toContain('Архив');
    expect(row.props.className).toContain('justify-between');
  });

  it('колбэк страницы передан в кнопку «Архив»', () => {
    const openArchive = (): void => {};
    const row = PropertiesSortRow({
      sort: DEFAULT_PROPERTY_SORT,
      onChange: () => {},
      showChip: false,
      onOpenArchive: openArchive,
    });
    const {elements, strings} = collect(row);

    expect(strings).toContain('Архив');
    const archiveButton = elements.find((element) => element.props.onClick === openArchive);
    expect(archiveButton).toBeDefined();
    expect(collect(archiveButton).strings).toContain('Архив');
  });
});
